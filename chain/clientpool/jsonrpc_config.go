package clientpool

import (
	"strings"
	"time"

	"sentioxyz/sentio-core/common/utils"
)

// JSONRPCConfig is the common part of the JSON-RPC chain client configs (evm/sui/sol).
// Chain configs embed it inline (json flattens embedded structs natively; yaml needs the
// `yaml:",inline"` tag on the embedding field), so the wire format is unchanged.
type JSONRPCConfig struct {
	Endpoint      string                   `json:"endpoint" yaml:"endpoint"`
	KeepWatch     time.Duration            `json:"keep_watch" yaml:"keep_watch"`
	MethodTimeout map[string]time.Duration `json:"method_timeout" yaml:"method_timeout"`

	// method black list
	MethodBlackList []string `json:"method_black_list" yaml:"method_black_list"`

	// method white list, empty means no white list
	MethodWhiteList []string `json:"method_white_list" yaml:"method_white_list"`

	// MethodAuthority marks the endpoint as defining the supported method set of the chain
	// (typically the chain's own full nodes). When such an endpoint rejects a method as not
	// supported at runtime, the client raises MethodNotSupportedByAuthorityTag on top of the
	// regular MethodNotSupportedTag (see Result.WithAuthorityVeto), and method-scoped consumers
	// pass that tag via InterruptWithTags to fail the method fast instead of probing endpoints
	// the authority already ruled out. A deliberate ACL cannot raise the authority tag: methods
	// disabled by this endpoint's own black/white list are rejected by CheckMethod before any
	// call is made (and the clients' internal probes, which bypass CheckMethod, discard tags),
	// so the endpoint simply abstains for those methods.
	MethodAuthority bool `json:"method_authority" yaml:"method_authority"`
}

// Trim normalizes the common fields; each chain's ClientConfig.Trim calls it with its own
// defaulted methodTimeout so a new common field only needs handling here.
func (c JSONRPCConfig) Trim(methodTimeout map[string]time.Duration) JSONRPCConfig {
	return JSONRPCConfig{
		Endpoint:        strings.TrimSpace(c.Endpoint),
		KeepWatch:       utils.Select(c.KeepWatch == 0, time.Second, c.KeepWatch),
		MethodTimeout:   methodTimeout,
		MethodBlackList: c.MethodBlackList,
		MethodWhiteList: c.MethodWhiteList,
		MethodAuthority: c.MethodAuthority,
	}
}
