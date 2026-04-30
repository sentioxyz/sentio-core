package serde

import (
	"fmt"
	"io"
	"strings"
)

var Trace bool = false

func getReaderPosForTracing(r io.Reader) int {
	if !Trace {
		return 0
	}
	pos, _ := r.(io.Seeker).Seek(0, io.SeekCurrent)
	return int(pos)
}

const tagName = "bcs"

const (
	tagValueOptional int64 = 1 << iota // optional
	tagValueIgnore                     // -
)

func parseTagValue(tag string) (int64, error) {
	var r int64
	tagSegs := strings.Split(tag, ",")
	for _, seg := range tagSegs {
		seg := strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		switch seg {
		case "optional":
			r |= tagValueOptional
		case "-":
			return tagValueIgnore, nil
		default:
			return 0, fmt.Errorf("unknown tag: %s in %s", seg, tag)
		}
	}

	return r, nil
}
