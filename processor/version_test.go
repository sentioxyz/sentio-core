package processor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionCheck(t *testing.T) {
	assert.ErrorContains(t, VersionCheck("1.0.0", "0.0.1"), "major")
	assert.NoError(t, VersionCheck("2.3.5", "2.3.0"))

	assert.ErrorContains(t, VersionCheck("1.2.0", "1.0.1"), "minor")
	assert.NoError(t, VersionCheck("1.2.0", "1.3.1"))
	assert.NoError(t, VersionCheck("1.3.5", "1.3.4"))

	assert.NoError(t, VersionCheck("1.2.5", "1.2.10"))
	assert.NoError(t, VersionCheck("1.2.5", "1.2.4"))
	assert.NoError(t, VersionCheck("1.2.5", "1.2.5"))

	assert.NoError(t, VersionCheck("1.2.5", "1.3.0-rc"))
}

func TestVersionParse(t *testing.T) {
	v, _ := ParseVersion("1.0.0")
	assert.True(t, v.Major == 1 && v.Minor == 0 && v.Patch == "0")

	v, _ = ParseVersion("1.0.1-rc.1")
	assert.True(t, v.Major == 1 && v.Minor == 0 && v.Patch == "1-rc.1")

	runtime, _ := GetRuntimeVersion("2.1.0")
	assert.Equal(t, "^2.0.0", runtime)

	runtime, _ = GetRuntimeVersion("2.2.0-rc")
	assert.Equal(t, "^2.0.0", runtime)

	runtime, _ = GetRuntimeVersion("1.1.0")
	assert.Equal(t, "^1.0.0", runtime)

}

func TestGetSDKPackageDep(t *testing.T) {
	var pkg, ver string

	pkg, ver = GetSDKPackageDep("2.1.0")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "2.1.0", ver)

	pkg, ver = GetSDKPackageDep("1.38.0")
	assert.Equal(t, "@sentio/sdk-all", pkg)
	assert.Equal(t, "1.38.0", ver)

	pkg, ver = GetSDKPackageDep("1.32.0")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "1.32.0", ver)

	pkg, ver = GetSDKPackageDep("2.40.0")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "2.40.0", ver)

	pkg, ver = GetSDKPackageDep("2.41.0")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "2.41.0", ver)

	pkg, ver = GetSDKPackageDep("2.40.0-rc.44")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "2.40.0-rc.44", ver)

	pkg, ver = GetSDKPackageDep("2.48.0-rc.1")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "npm:@sentio/sdk-bundle@2.48.0-rc.1", ver)

	pkg, ver = GetSDKPackageDep("2.48.0")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "npm:@sentio/sdk-bundle@2.48.0", ver)

	pkg, ver = GetSDKPackageDep("2.58.2-rc.39")
	assert.Equal(t, "@sentio/sdk", pkg)
	assert.Equal(t, "npm:@sentio/sdk-bundle@2.58.2-rc.39", ver)
}
