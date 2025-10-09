package utils

import "strings"

func ToHeaders(s string) map[string]string {
	ret := map[string]string{}
	if s != "" {
		for _, line := range strings.Split(s, "\r\n") {
			split := strings.Split(line, ":")
			if len(split) == 2 {
				ret[split[0]] = split[1]
			}
		}
	}
	return ret
}
