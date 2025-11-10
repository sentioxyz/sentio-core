package processor

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
)

const ServerMinVersion = "1.13.x"

type Version struct {
	Major int
	Minor int
	Patch string
}

func (v *Version) RuntimeSupported() bool {
	if v.Major > 1 {
		return true
	}
	return v.Minor >= 37
}

func (v *Version) String() string {
	return fmt.Sprintf("%d.%d.%s", v.Major, v.Minor, v.Patch)
}

func ParseVersion(version string) (*Version, error) {
	versions := strings.SplitN(version, ".", 3)
	if len(versions) != 3 {
		return nil, errors.Errorf("invalid version length: %s", version)
	}
	major, err := strconv.Atoi(versions[0])
	if err != nil {
		return nil, errors.Errorf("invalid major version: %s", version)
	}
	minor, err := strconv.Atoi(versions[1])
	if err != nil {
		return nil, errors.Errorf("invalid minor version: %s", version)
	}
	return &Version{
		Major: major,
		Minor: minor,
		Patch: versions[2],
	}, nil
}

func (v *Version) IsDevelopmentVersion() bool {
	return strings.HasSuffix(v.Patch, "-development")
}

func (v *Version) GetRCVersion() (patchVer int, rcVer int, isRC bool) {
	patchVerStr, additional, hasAdd := strings.Cut(v.Patch, "-")
	patchVer, _ = strconv.Atoi(patchVerStr)
	if !hasAdd {
		return
	}
	if isRC = strings.HasPrefix(additional, "rc."); !isRC {
		return
	}
	rcVer, _ = strconv.Atoi(strings.TrimPrefix(additional, "rc."))
	return
}

// VersionCheck the version compatibility check follows logic below
// for version: x.y.z, x is called major version, y is minor, z is patch (follow semantic versioning)
//  1. If major version is different report error
//  2. Server minor version means the minimal version server would support
//     If server minor version greater than client version, report error
//  3. Patch version doesn't matter
func VersionCheck(hostVersion string, clientVersion string) error {
	upgradeServerVersion := "consider upgrade @sentio/SDK to " + hostVersion
	if clientVersion == "" {
		return errors.Errorf("no version included in the request\n%s", upgradeServerVersion)
	}

	log.Debugf("Checking host version %s vs clientVersion %s", hostVersion, clientVersion)

	//clientVersion = StripReleaseCandidateVersion(clientVersion)

	hostVersions, err := ParseVersion(hostVersion)
	if err != nil {
		log.Fatale(err)
	}

	clientVersions, err := ParseVersion(clientVersion)
	if clientVersions.IsDevelopmentVersion() {
		return nil
	}
	if err != nil {
		log.Fatale(err)
	}

	// Rule 1 check
	if clientVersions.Major < hostVersions.Major {
		return errors.Errorf(
			"SDK major version (%d) not compatible with server (%d)\n%s",
			clientVersions.Major,
			hostVersions.Major,
			upgradeServerVersion,
		)
	}

	// Rule 2 check
	if clientVersions.Major == hostVersions.Major && clientVersions.Minor < hostVersions.Minor {
		return errors.Errorf(
			"SDK minor version (%s) not compatible with server (%s)\n%s\nnote the restriction might be loosen in the future",
			clientVersion,
			hostVersion,
			upgradeServerVersion,
		)
	}
	return nil
}

//func StripReleaseCandidateVersion(version string) string {
//	index := strings.Index(version, "-rc.")
//	if index != -1 {
//		return version[:index]
//	}
//	return version
//}

//func GetInstallationVersion(version string) string {
//	if IsDevelopmentVersion(version) {
//		return "^" + version
//	}
//	return version
//}

func GetRuntimeVersion(version string) (string, error) {
	value, exists := os.LookupEnv("PROCESSOR_RUNTIME_VERSION")
	if exists {
		return value, nil
	}

	v, err := ParseVersion(version)
	if err != nil {
		return version, err
	}
	if !v.RuntimeSupported() {
		return "^1.0.0", nil
	}

	// For RC version, use same runtime for testing
	if strings.Contains(v.Patch, "rc") {
		if v.Major == 2 && v.Minor > 47 {
			return version, nil
		}
		if v.Major >= 3 {
			return version, nil
		}
	}

	return "^" + fmt.Sprint(v.Major) + ".0.0", nil
}

func GetSDKPackageDep(version string) (pkg string, ver string) {
	pkg = "@sentio/sdk"
	ver = version
	v, err := ParseVersion(version)
	if v.Major > 1 {
		verPrefix := ""
		if v.Major > 2 ||
			(v.Major == 2 && v.Minor >= 48) {
			// >= 2.48.0 use @sentio/sdk-bundle
			verPrefix = "npm:@sentio/sdk-bundle@"
		}
		if v.IsDevelopmentVersion() {
			ver = verPrefix + "^" + fmt.Sprint(v.Major) + ".0.0"
			return
		}
		ver = verPrefix + version
		return
	}

	if v.IsDevelopmentVersion() {
		pkg = "@sentio/sdk-all"
		ver = "^" + fmt.Sprint(v.Major) + ".0.0"
		return
	}

	if err == nil && v.RuntimeSupported() {
		pkg = "@sentio/sdk-all"
	}
	return
}

func HostProcessorVersion() string {
	return ServerMinVersion
}
