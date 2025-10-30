package flags

import (
	"flag"
	"strings"

	"sentioxyz/sentio-core/common/log"
)

func ParseAndInitLogFlag() {
	flag.Parse()
	log.BindFlag()
}

type StringSlice []string

func (s *StringSlice) String() string {
	return strings.Join(*s, ",")
}

func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}
