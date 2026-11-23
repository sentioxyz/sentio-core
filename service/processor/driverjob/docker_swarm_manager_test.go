package driverjob

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/service/processor/models"
)

func TestProcessorScriptRunsUserCodeUnprivileged(t *testing.T) {
	d := &DockerSwarmManager{config: DriverConfig{ProcessorService: "processor:10010"}}
	scriptPath, err := d.generateProcessorScript(&models.Processor{ID: "testpid1"})
	require.NoError(t, err)
	defer func() { _ = os.Remove(scriptPath) }()
	content, err := os.ReadFile(scriptPath)
	require.NoError(t, err)
	script := string(content)

	// nothing in the shared cache may be made world-writable
	require.NotContains(t, script, "777")
	require.NotContains(t, script, "dumps && chmod")

	// the cache is hardened before prepare writes into it
	hardenAt := strings.Index(script, "chmod 0755 "+CacheDirMountPath+"\n")
	prepareAt := strings.Index(script, "-prepare-processor-env-only=true")
	require.GreaterOrEqual(t, hardenAt, 0)
	require.Greater(t, prepareAt, hardenAt)

	// every exec of the processor goes through the privilege drop
	require.Contains(t, script, `exec setpriv --nnp su -s /bin/sh -c 'exec "$@"' -- node processor "$@"`)
	for _, line := range strings.Split(script, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "exec ") {
			require.True(t, strings.HasPrefix(line, "exec setpriv --nnp su "), "unexpected exec: %s", line)
		}
	}
	require.Equal(t, 2, strings.Count(script, "    run_unprivileged "))
}

func TestProcessorServiceSpecNoNewPrivileges(t *testing.T) {
	d := &DockerSwarmManager{config: DriverConfig{CacheDir: "/var/lib/sentio/cache"}}
	spec, err := d.buildProcessorServiceSpec(&models.Processor{ID: "testpid1"}, "net", "/tmp/start.sh")
	require.NoError(t, err)
	require.NotNil(t, spec.TaskTemplate.ContainerSpec.Privileges)
	require.True(t, spec.TaskTemplate.ContainerSpec.Privileges.NoNewPrivileges)
}
