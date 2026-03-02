package util

import "time"

var (
	SupportGoTimeLayouts = []string{
		time.Layout,
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC822Z,
	}

	SupportClickhouseTimeLayouts = []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
)
