package adaptor_eventlogs

import (
	"strings"

	"sentioxyz/sentio-core/common/chliteral"
)

type Breakdown []string

func (b Breakdown) String(addComma bool) string {
	var escaped []string
	for _, field := range b {
		escaped = append(escaped, chliteral.Identifier(field))
	}
	str := strings.Join(escaped, ",")
	if addComma && str != "" {
		str = "," + str
	}
	return str
}
