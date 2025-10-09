package utils

import "regexp"

func CollectParameters(re *regexp.Regexp, orig string) map[string]string {
	if re == nil {
		return nil
	}
	values := re.FindStringSubmatch(orig)
	if len(values) == 0 {
		return nil
	}
	pp := make(map[string]string)
	keys := re.SubexpNames()
	for i, key := range keys {
		if key != "" {
			pp[key] = values[i]
		}
	}
	return pp
}
