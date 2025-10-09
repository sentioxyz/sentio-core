package utils

import (
	"encoding/json"
	"errors"
	"regexp"
	commonProtos "sentioxyz/sentio/service/common/protos"
	"time"
)

var re = regexp.MustCompile(`(?m)([\.0-9]+)([mhs])`)

func DurationToPB(d time.Duration) *commonProtos.Duration {
	s := d.String()
	unit := "s"

	for _, m := range re.FindAllStringSubmatch(s, -1) {
		num := m[1]
		if num != "0" {
			unit = m[2]
		}
	}

	switch unit {
	case "m":
		return &commonProtos.Duration{
			Unit:  "m",
			Value: d.Minutes(),
		}
	case "h":
		return &commonProtos.Duration{
			Unit:  "h",
			Value: d.Hours(),
		}
	case "s":
		fallthrough
	default:
		return &commonProtos.Duration{
			Unit:  "s",
			Value: d.Seconds(),
		}
	}
}

type Duration time.Duration

func (d *Duration) String() string {
	return d.GetDuration().String()
}

func (d *Duration) UnmarshalJSON(raw []byte) error {
	if len(raw) == 0 {
		*d = 0
		return nil
	}
	if raw[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return err
		}
		if dur, err := time.ParseDuration(str); err != nil {
			return err
		} else {
			*d = (Duration)(dur)
			return nil
		}
	} else {
		return errors.New("parse duration failed: must be a string")
	}
}

func (d *Duration) MarshalJSON() ([]byte, error) {
	if d == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(d.String())
}

func (d *Duration) GetDuration() time.Duration {
	if d == nil || *d == 0 {
		return time.Hour * 24
	}
	return (time.Duration)(*d)
}
