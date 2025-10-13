package configmanager

import (
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type ConfigEncoder string

const (
	ConfigEncoderJSON ConfigEncoder = "json"
	ConfigEncoderYAML ConfigEncoder = "yaml"
	ConfigEncoderTOML ConfigEncoder = "toml"
	ConfigEncoderTEXT ConfigEncoder = "text"
)

func (c ConfigEncoder) Parse(data []byte) (map[string]any, error) {
	var m map[string]any
	switch c {
	case ConfigEncoderJSON:
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
	case ConfigEncoderYAML:
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, err
		}
	case ConfigEncoderTOML:
		if err := toml.Unmarshal(data, &m); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("config encoder not supported: %s", c)
	}
	return m, nil
}
