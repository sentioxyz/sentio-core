package move

import "strings"

func ToShortAddress(addr string) string {
	if !strings.HasPrefix(addr, "0x") {
		return addr
	}
	if hexPart := strings.TrimLeft(addr[2:], "0"); hexPart != "" {
		return "0x" + hexPart
	}
	return "0x0"
}

func TrimTypeString(t string) string {
	x, err := BuildType(t)
	if err != nil {
		return t
	}
	return x.String()
}
