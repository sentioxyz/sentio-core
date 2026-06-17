package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"sentioxyz/sentio-core/common/jsonutils"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/processor/models"
)

// ConfigV2 is the per-chain configuration consumed by the streaming
// (driver v3/v4) controller. The legacy driver v2 configuration (chain.Config)
// stays in the sentio repository.
type ConfigV2 struct {
	ChainID                  string
	Endpoint                 string
	StartBlockOverride       int64
	ProcessingDelayBlocks    uint64
	KeepSuiEventTypePackage  bool
	SkipStartBlockValidation bool
	IsCustomizedEndpoint     bool
}

// PatchChainsConfigEnv is the env var that, when set, carries a JSON patch
// applied on top of the chains config file before it is parsed.
const PatchChainsConfigEnv = "CHAIN_CONFIG_JSON_PATCH"

func LoadChainsConfigV2(
	path string,
	patchEnv string,
	networkOverrides []models.NetworkOverride,
) (map[string]*ConfigV2, error) {
	var file []byte
	var err error
	if file, err = os.ReadFile(path); err != nil {
		return nil, err
	}
	if patch := strings.TrimSpace(os.Getenv(patchEnv)); patchEnv != "" && patch != "" {
		file, err = jsonutils.Patch(file, []byte(patch), func(path string, or, pa any) {
			log.Infof("patch chains config %s %v => %v", path, or, pa)
		})
		if err != nil {
			return nil, fmt.Errorf("patch chain config from env %s failed: %w", patchEnv, err)
		}
	}
	var chainsConfig map[string]*ConfigV2
	if err = json.Unmarshal(file, &chainsConfig); err != nil {
		return nil, err
	}
	for _, no := range networkOverrides {
		chainsConfig[no.Chain] = &ConfigV2{ChainID: no.Chain, Endpoint: no.Host, IsCustomizedEndpoint: true}
		log.Infof("will use customized host %q in chain %s", no.Host, no.Chain)
	}
	return chainsConfig, nil
}

func NewCustomizedChainConfigV2(chainID, endpoint string) *ConfigV2 {
	return &ConfigV2{
		ChainID:  chainID,
		Endpoint: endpoint,
	}
}
