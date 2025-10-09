package utils

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

func LoadFile(title, file string, obj any) error {
	cnt, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s file %q failed: %w", title, file, err)
	}
	if strings.HasSuffix(file, ".yaml") {
		err = yaml.Unmarshal(cnt, obj)
	} else {
		err = json.Unmarshal(cnt, obj)
	}
	if err != nil {
		return fmt.Errorf("unmarshal %s from file %q failed (%w): %s", title, file, err, string(cnt))
	}
	return nil
}

func SaveFile(title, file string, obj any) error {
	cnt, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("marshal %s to file %q failed(%w): %#v", title, file, err, obj)
	}
	if err = os.WriteFile(file, cnt, 0644); err != nil {
		return fmt.Errorf("write %s to file %q failed: %w", title, file, err)
	}
	return nil
}
