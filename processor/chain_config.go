package processor

import (
	"encoding/json"
	"os"
)

// Rpc mirrors the sentio-sdk runtime RpcConfig
// (sentio-sdk/packages/runtime/src/chain-config.ts).
type Rpc struct {
	Url     string            `json:"Url"`
	Headers map[string]string `json:"Headers,omitempty"`
}

// ChainConfig matches the chains-config shape the sentio-sdk processor-runner
// reads: it carries only the fields the SDK consumes.
//
// It is the union of the endpoint fields every in-use SDK runtime reads,
// because SDK 2.x, 3.x and 4.x all run concurrently and select the endpoint
// differently:
//   - SDK 2.x / 3.x runtime: ChainServer (primary), else Https[0].
//   - SDK 4.x runtime:       Rpc.Url (primary), else Https[0].
//
// So the driver sets ChainServer and Rpc together when routing through the
// rpc-node proxy, and falls back to ChainServer + Https for the
// direct/customized-endpoint case where the SDK reaches the endpoint itself.
//
// This is distinct from driver/controller/config.ChainConfig, which is the Go
// streaming controller's own configuration; ChainConfig here is the shape the
// TypeScript SDK reads from the chains-config.json handed to processor-runner.
type ChainConfig struct {
	ChainID     string   `json:"ChainID"`
	Https       []string `json:"Https,omitempty"`
	ChainServer string   `json:"ChainServer,omitempty"`
	Rpc         *Rpc     `json:"Rpc,omitempty"`
}

// SaveChainsConfig marshals the SDK-facing chains config and writes it to path.
func SaveChainsConfig(path string, configs map[string]*ChainConfig) error {
	file, err := json.Marshal(configs)
	if err != nil {
		return err
	}
	return os.WriteFile(path, file, 0644)
}
