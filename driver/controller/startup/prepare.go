// Prepare step for driver v3/v4 (SDK 3.x / 4.x) processors: downloads the
// uploaded bundle, installs the Node.js runtime environment the
// processor-runner needs, and writes the SDK-facing chains-config override.
// The driver binary runs this in its -prepare-processor-env-only mode (as the
// init step of the processor container); driver v2 (SDK 2.x) processors keep
// their own preparation path outside sentio-core.
package startup

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc/credentials"

	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/processor"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	protossvc "sentioxyz/sentio-core/service/processor/protos"
)

// RPCNode describes the internal rpc-node proxy assigned to a processor. The
// SDK routes its own chain requests through it instead of reaching endpoints
// directly.
type RPCNode struct {
	URL           string
	GrpcURL       string
	ProcessorCode string
}

// RPCNodeResolver resolves the internal rpc-node proxy for a processor. The
// rpc-node service is not part of sentio-core, so the implementation lives in
// the driver binary; a nil resolver means no proxy is available and the
// SDK-facing chains config keeps the direct endpoints from the config file.
type RPCNodeResolver interface {
	Resolve(ctx context.Context, processorID string) (*RPCNode, error)
}

type PrepareConfig struct {
	ProcessorID      string
	ProcessorService string
	// CacheDir is the root cache directory shared with the processor
	// launcher: the prepared environment lands in
	// <CacheDir>/<hash(pid)>/sentio/<pid> and the launcher locates it through
	// <CacheDir>/.processor-path.
	CacheDir        string
	ChainConfigFile string
	// LocalSDK, when non-empty, installs the SDK from this local path instead
	// of the published package (development only).
	LocalSDK string
	// ProcessorRuntime, when non-empty, overrides the @sentio/runtime version
	// derived from the processor's SDK version.
	ProcessorRuntime string
	UsePNPM          bool

	RPCNode         RPCNodeResolver
	DialCredentials credentials.TransportCredentials
}

// PrepareMain prepares the processor-runner environment for a driver v3/v4
// processor and writes <CacheDir>/.processor-path on success. It returns false
// without doing anything when the processor is a driver v2 one
// (DriverVersion < 1), so the caller can fall back to its own v2 preparation
// path. Unrecoverable errors are fatal: the prepare step runs as an init
// container and relies on the restart policy to retry.
func PrepareMain(prepConfig PrepareConfig) bool {
	ctx, logger := log.FromContext(context.Background(), "processorID", prepConfig.ProcessorID)

	base := baseStartupController{config: Config{
		ProcessorID:      prepConfig.ProcessorID,
		ProcessorService: prepConfig.ProcessorService,
		DialCredentials:  prepConfig.DialCredentials,
	}}
	defer base.releaseAll()
	if err := base.connectToProcessorService(ctx); err != nil {
		logger.Fatale(err, "connect to processor service failed")
	}
	if err := base.getProcessor(ctx); err != nil {
		logger.Fatale(err, "get processor failed")
	}
	if base.processor.DriverVersion < 1 {
		logger.Warnf("driver version is %d < 1, will not prepare processor env here", base.processor.DriverVersion)
		return false
	}

	p := preparer{base: &base, config: prepConfig}
	switch base.processor.Project.Type {
	case commonmodels.ProjectTypeSentio, commonmodels.ProjectTypeAction:
		targetDir, targetPath := p.prepareEnv(ctx)
		content := targetDir + "\n" + targetPath + "\n"
		if err := os.WriteFile(filepath.Join(prepConfig.CacheDir, ".processor-path"), []byte(content), 0644); err != nil {
			logger.Fatale(err, "write .processor-path failed")
		}
	default:
		logger.Warnf("project type is %q, will do nothing for preparing processor env", base.processor.Project.Type)
	}
	return true
}

type preparer struct {
	base   *baseStartupController
	config PrepareConfig
}

// cacheDirHash spreads processors across cache subdirectories; it must stay
// stable because the prepared environments are reused across restarts.
func cacheDirHash(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

const packageAddTimeWarning = 10 * time.Second

// prepareEnv downloads the processor bundle and sets up targetDir with
// package.json (SDK + runtime deps), node_modules, lib.js (or the standalone
// binary) and the SDK-facing chains-config override.
func (p *preparer) prepareEnv(ctx context.Context) (targetDir string, targetPath string) {
	_, logger := log.FromContext(ctx)
	proc := p.base.processor

	targetDir = filepath.Join(p.config.CacheDir, fmt.Sprintf("%d", cacheDirHash(proc.ID)), "sentio", proc.ID)
	if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
		// This ignores already exists error.
		logger.Infoe(err, "Failed to create target dir")
	}

	resp, err := p.base.processorClient.DownloadProcessor(ctx, &protossvc.DownloadProcessorRequest{
		ProcessorId: proc.ID,
	})
	if err != nil {
		logger.Fatale(err, "download failed")
	}
	downloadURL := resp.Url
	if !strings.HasPrefix(downloadURL, "http") {
		// compat with old local files
		downloadURL = "http://" + p.config.ProcessorService + downloadURL
	}

	if proc.Binary {
		targetPath = filepath.Join(targetDir, "main")
		if err = downloadFile(ctx, targetPath, downloadURL); err != nil {
			logger.Fatale(err, "downloaded ", downloadURL, " to ", targetPath, " failed")
		}
		return targetDir, targetPath
	}

	targetPath = filepath.Join(targetDir, "lib.js")
	if err = downloadFile(ctx, targetPath, downloadURL); err != nil {
		logger.Fatale(err, "downloaded ", downloadURL, " to ", targetPath, " failed")
	}

	p.writePackageJSON(ctx, targetDir)
	p.installPackages(ctx, targetDir)
	p.writeSDKChainsConfig(ctx, targetDir)
	return targetDir, targetPath
}

// writePackageJSON writes the package.json that pins the SDK, runtime and
// protos versions for the processor-runner. Driver v3/v4 processors are always
// SDK 3.x+ (ESM, @sentio/sdk-bundle), so none of the SDK 2.x compatibility
// patches apply here.
func (p *preparer) writePackageJSON(ctx context.Context, targetDir string) {
	_, logger := log.FromContext(ctx)
	proc := p.base.processor

	sdkVersionStr := proc.SdkVersion
	version, err := processor.ParseVersion(sdkVersionStr)
	if err != nil {
		logger.Fatale(err, "version parse failed")
	}

	runtimeVersion, err := processor.GetRuntimeVersion(sdkVersionStr)
	if err != nil {
		logger.Fatale(err, "failed to get runtime version")
	}
	if p.config.ProcessorRuntime != "" {
		runtimeVersion = p.config.ProcessorRuntime
	}

	protosVersion := sdkVersionStr
	if version.IsDevelopmentVersion() {
		protosVersion = "^" + fmt.Sprint(version.Major) + ".0.0"
	}

	sdkDep := ""
	if p.config.LocalSDK == "" {
		sdkPkg, sdkVer := processor.GetSDKPackageDep(sdkVersionStr)
		sdkDep = fmt.Sprintf(`"%s": "%s",`, sdkPkg, sdkVer)
	}

	actionPkg := ""
	if proc.Project.Type == commonmodels.ProjectTypeAction {
		// action package has the same version as runtime
		actionPkg = fmt.Sprintf(`"@sentio/action": "%s",`, runtimeVersion)
	}

	packagePath := filepath.Join(targetDir, "package.json")
	logger.Info("creating package json with resolution at " + packagePath)
	packageContent := fmt.Sprintf(`{
	"dependencies": {
		%s
		%s
		"@sentio/runtime": "%s"
	},
	"resolutions": {
		"@sentio/protos": "%s",
		"@sentio/runtime": "%s"
	},
	"type": "module"
}
`, sdkDep, actionPkg, runtimeVersion, protosVersion, runtimeVersion)
	if err = os.WriteFile(packagePath, []byte(packageContent), 0644); err != nil {
		logger.Fatale(err, "write package.json failed")
	}
}

// installPackages runs yarn/pnpm in targetDir to materialize node_modules.
func (p *preparer) installPackages(ctx context.Context, targetDir string) {
	_, logger := log.FromContext(ctx)

	if version, err := processor.ParseVersion(p.base.processor.SdkVersion); err == nil && version.IsDevelopmentVersion() {
		// a development version floats, so drop the lock and modules from the
		// reused cache dir to force a fresh resolution
		lockFile := "yarn.lock"
		if p.config.UsePNPM {
			lockFile = "pnpm-lock.yaml"
		}
		if err = os.Remove(filepath.Join(targetDir, lockFile)); err != nil {
			logger.Errore(err, "failed to clean lock file")
		}
		if err = os.RemoveAll(filepath.Join(targetDir, "node_modules")); err != nil {
			logger.Errore(err, "failed to clean node_modules")
		}
	}

	var cmd *exec.Cmd
	if p.config.LocalSDK != "" {
		packageID := "file:" + p.config.LocalSDK
		if p.config.UsePNPM {
			cmd = exec.Command("pnpm", "add", "--shamefully-hoist", "--node-linker=hoisted", packageID)
		} else {
			cmd = exec.Command("yarn", "add", packageID)
		}
	} else {
		if p.config.UsePNPM {
			cmd = exec.Command("pnpm", "install", "--shamefully-hoist", "--node-linker=hoisted")
		} else {
			cmd = exec.Command("yarn", "install")
		}
	}
	cmd.Dir = targetDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	logger.Info("Install SDK with command: " + cmd.String())
	start := time.Now()
	err := cmd.Run()
	usedTime := time.Since(start)
	if err != nil || usedTime > packageAddTimeWarning {
		logger.Infow("package add completed", "stdout", stdout.String(), "stderr", stderr.String())
	} else {
		logger.Debugw("package add completed", "stdout", stdout.String(), "stderr", stderr.String())
	}
	if err != nil {
		if dur, _ := time.ParseDuration(os.Getenv("ADD_PACKAGE_FAIL_WAIT")); dur > 0 {
			logger.Errorfe(err, "package add failed, will exit after %s", dur)
			time.Sleep(dur)
		}
		// will retry via the init container restart policy
		logger.Fatale(err, "package add failed")
	}
	logger.Infow("package add succeed", "used", usedTime)
}

// writeSDKChainsConfig builds the chains config the sentio-sdk processor-runner
// reads and writes it into targetDir. The SDK reads a much smaller shape than
// the streaming controller's config.ChainConfig: only the endpoint —
// ChainServer for the SDK 3.x runtime, Rpc.Url for the SDK 4.x one, both
// falling back to Https[0].
func (p *preparer) writeSDKChainsConfig(ctx context.Context, targetDir string) {
	_, logger := log.FromContext(ctx)
	proc := p.base.processor

	chainsConfig, err := config.LoadChainsConfig(
		p.config.ChainConfigFile, config.PatchChainsConfigEnv, proc.NetworkOverrides)
	if err != nil {
		logger.Fatale(err, "load chains config failed")
	}

	// Default to the direct case: the SDK reaches the endpoint itself, so
	// carry ChainServer + Https straight from the config file. A customized
	// endpoint (network override) only carries Endpoint.
	sdkChainsConfig := make(map[string]*processor.ChainConfig, len(chainsConfig))
	for chainID, cfg := range chainsConfig {
		id := cfg.ChainID
		if id == "" {
			id = chainID
		}
		https := cfg.HTTPServers
		if len(https) == 0 && cfg.IsCustomizedEndpoint && cfg.Endpoint != "" {
			https = []string{cfg.Endpoint}
		}
		sdkChainsConfig[chainID] = &processor.ChainConfig{
			ChainID:     id,
			ChainServer: cfg.ChainServer,
			Https:       https,
		}
	}

	// With an rpc-node proxy available, route the SDK through it: point both
	// ChainServer (SDK 3.x runtime) and Rpc (SDK 4.x runtime) at the proxy;
	// Https is then redundant and dropped.
	if p.config.RPCNode != nil {
		node, err := p.config.RPCNode.Resolve(ctx, proc.ID)
		if err != nil {
			logger.Fatalfe(err, "get rpc node url failed")
		}
		for chainID := range chainsConfig {
			info, has := chains.ChainIDToInfo[chains.ChainID(chainID)]
			if !has {
				continue
			}
			chainServer := strings.TrimRight(node.URL, "/") + "/" + info.Slug
			rpc := &processor.Rpc{Url: chainServer}
			var useGrpc bool
			switch chainType, _ := chains.GetChainType(chains.ChainID(chainID)); chainType {
			case chains.SuiChainType:
				useGrpc = SuiEnableGRPC(chainID, proc.DriverVersion)
			}
			if useGrpc {
				rpc = &processor.Rpc{
					Url: node.GrpcURL,
					Headers: map[string]string{
						// rpc-node proxy routes internal processor requests by this host.
						"X-Forwarded-Host": fmt.Sprintf("%s-%s-%s", proc.ID, node.ProcessorCode, info.Slug),
					},
				}
			}
			sc := sdkChainsConfig[chainID]
			sc.ChainServer = chainServer
			sc.Rpc = rpc
			sc.Https = nil
			logger.Infof("chain server for chain %s will use rpc: %+v", chainID, sc.Rpc)
		}
	}

	overrideChainConfigsFile := filepath.Join(targetDir, filepath.Base(p.config.ChainConfigFile))
	if err = processor.SaveChainsConfig(overrideChainConfigsFile, sdkChainsConfig); err != nil {
		logger.Fatalfe(err, "save chains config to %s failed", overrideChainConfigsFile)
	}
	logger.Infow("saved override chains config", "path", overrideChainConfigsFile)
}

// downloadFile fetches url into localPath; a zip response is unpacked to its
// lib.js / main entry.
func downloadFile(ctx context.Context, localPath string, url string) error {
	_, logger := log.FromContext(ctx)

	logger.Infof("Downloading from %s to %s", url, localPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errors.Wrap(err, "build download request failed")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "download request failed")
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("error downloading: %s", resp.Status)
	}

	out, err := os.Create(localPath)
	if err != nil {
		return errors.Wrap(err, "create download target failed")
	}
	defer func() {
		_ = out.Close()
	}()

	var body io.Reader = resp.Body
	if resp.Header.Get("Content-Type") == "application/zip" {
		content, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "read zip response failed")
		}
		zipReader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
		if err != nil {
			return errors.Wrap(err, "open zip response failed")
		}
		for _, zipFile := range zipReader.File {
			if zipFile.Name == "lib.js" || zipFile.Name == "main" {
				entry, err := zipFile.Open()
				if err != nil {
					return errors.Wrapf(err, "open %s in zip response failed", zipFile.Name)
				}
				defer func() {
					_ = entry.Close()
				}()
				body = entry
			}
		}
	}

	_, err = io.Copy(out, body)
	return errors.Wrap(err, "write downloaded file failed")
}
