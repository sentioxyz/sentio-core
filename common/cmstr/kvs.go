package cmstr

import (
	"bytes"
	"errors"
	"strings"
)

type KV struct {
	Key   string
	Value string
}

type KVS struct {
	data []KV
}

func (c *KVS) Add(key, value string) {
	c.data = append(c.data, KV{key, value})
}

func (c *KVS) Get(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	for _, kv := range c.data {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return "", false
}

func (c *KVS) Load(str string) error {
	c.data = nil
	var s int
	var lvl int
	var sectors []string
	for i, x := range str {
		switch x {
		case '(':
			lvl++
		case ')':
			lvl--
			if lvl < 0 {
				return errors.New("miss '('")
			}
		case ' ':
			if lvl == 0 {
				if sector := strings.TrimSpace(str[s:i]); sector != "" {
					sectors = append(sectors, sector)
				}
				s = i + 1
			}
		}
	}
	if lvl > 0 {
		return errors.New("miss ')'")
	}
	if sector := strings.TrimSpace(str[s:]); sector != "" {
		sectors = append(sectors, sector)
	}
	for _, sector := range sectors {
		p := strings.Index(sector, "(")
		if p < 0 {
			c.data = append(c.data, KV{Key: sector})
		} else {
			c.data = append(c.data, KV{Key: sector[:p], Value: sector[p+1 : len(sector)-1]})
		}
	}
	return nil
}

func (c *KVS) String() string {
	if c == nil {
		return ""
	}
	var buf bytes.Buffer
	for i, d := range c.data {
		if i > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(d.Key)
		buf.WriteString("(")
		buf.WriteString(d.Value)
		buf.WriteString(")")
	}
	return buf.String()
}
