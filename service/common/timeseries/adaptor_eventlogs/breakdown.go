package adaptor_eventlogs

import (
	"strings"
)

type Breakdown []string

func (b Breakdown) String(addComma bool) string {
	var escaped []string
	for _, field := range b {
		escaped = append(escaped, "`"+field+"`")
	}
	str := strings.Join(escaped, ",")
	if addComma && str != "" {
		str = "," + str
	}
	return str
}
