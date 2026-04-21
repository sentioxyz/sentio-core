package rg

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"io"
	"strconv"
	"strings"
)

type RangeParser struct{}

func (p RangeParser) Marshal(r Range, w io.Writer) (err error) {
	if r.IsEmpty() {
		_, err = fmt.Fprintf(w, "EMPTY")
	} else if r.End == nil {
		_, err = fmt.Fprintf(w, "%d,INF", r.Start)
	} else {
		_, err = fmt.Fprintf(w, "%d,%d", r.Start, *r.End)
	}
	return err
}

var ErrInvalidFormat = errors.New("invalid format")

func (p RangeParser) Unmarshal(r io.Reader) (Range, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return Range{}, err
	}
	if strings.TrimSpace(string(d)) == "EMPTY" {
		return EmptyRange, nil
	}
	s := strings.Split(string(d), ",")
	if len(s) < 2 {
		return Range{}, ErrInvalidFormat
	}
	start, err := strconv.ParseUint(strings.TrimSpace(s[0]), 10, 64)
	if err != nil {
		return Range{}, err
	}
	if strings.TrimSpace(s[1]) == "INF" {
		return Range{Start: start}, nil
	}
	end, err := strconv.ParseUint(strings.TrimSpace(s[1]), 10, 64)
	if err != nil {
		return Range{}, err
	}
	return NewRange(start, end), nil
}

type SetParser struct {
	RangeParser
}

func (p SetParser) Marshal(s RangeSet, w io.Writer) error {
	for i, r := range s.GetRanges() {
		if i > 0 {
			_, err := w.Write([]byte{'|'})
			if err != nil {
				return err
			}
		}
		if err := p.RangeParser.Marshal(r, w); err != nil {
			return err
		}
	}
	return nil
}

func (p SetParser) Unmarshal(r io.Reader) (RangeSet, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return RangeSet{}, err
	}
	var list []Range
	for _, rs := range strings.Split(string(d), "|") {
		ra, err := p.RangeParser.Unmarshal(bytes.NewReader([]byte(rs)))
		if err != nil {
			return RangeSet{}, err
		}
		list = append(list, ra)
	}
	return NewRangeSet(list...), nil
}
