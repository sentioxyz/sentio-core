package version

import (
	"fmt"
	"runtime"
)

var Version = "local"
var CommitSha = "head"
var BuildTimestamp = "<na>"

func IsProduction() bool {
	return Version != "local"
}

// ClientVersion returns a geth-style identity string for this server binary,
// e.g. "sentio-super-node/v1.2.3-0abc12de/darwin-arm64/go1.24.1". Servers use it
// to answer node-identity RPC methods (web3_clientVersion, getVersion, ...)
// about themselves instead of forwarding them upstream.
func ClientVersion(name string) string {
	return fmt.Sprintf("%s/%s-%s/%s-%s/%s", name, Version, CommitSha, runtime.GOOS, runtime.GOARCH, runtime.Version())
}
