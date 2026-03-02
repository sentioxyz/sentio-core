package cursor

import (
	"fmt"
	"math"
	"time"

	"sentioxyz/sentio-core/common/gonanoid"

	"github.com/bytedance/sonic"
	"github.com/pkg/errors"
)

const (
	rawSQLMaxLimit  = 1000
	infiniteLimit   = math.MaxUint32
	cursorKeyLength = 48
	cursorKeyPrefix = "sentio-timeseries-cursor-"
	TTL             = time.Minute * 10
)

type Cursor interface {
	Cursor() string
	Next() Cursor
	Dump() string
	GetLimit() int
	GetOffset() int
}

type cursor struct {
	Key           string `json:"key"`
	OriginLimit   *int   `json:"origin_limit,omitempty"`
	OriginOffset  *int   `json:"origin_offset,omitempty"`
	RewriteLimit  int    `json:"rewrite_limit"`
	RewriteOffset int    `json:"rewrite_offset"`
	Step          int    `json:"step"`
	GotLine       int    `json:"got_line"`
}

func genCursorKey() string {
	return fmt.Sprintf("%s%s", cursorKeyPrefix, gonanoid.Must(cursorKeyLength))
}

func NewCursor(limit, offset *int) Cursor {
	c := &cursor{
		Key:           genCursorKey(),
		OriginLimit:   limit,
		OriginOffset:  offset,
		RewriteLimit:  rawSQLMaxLimit,
		RewriteOffset: 0,
		Step:          rawSQLMaxLimit,
	}
	if c.OriginOffset != nil {
		c.RewriteOffset = *c.OriginOffset
	}
	if c.OriginLimit != nil {
		if *c.OriginLimit <= c.Step {
			c.RewriteLimit = *c.OriginLimit
		}
	}
	c.GotLine = c.RewriteLimit
	return c
}

func NewCursorWithStep(limit, offset *int, step int) Cursor {
	c := &cursor{
		Key:           genCursorKey(),
		OriginLimit:   limit,
		OriginOffset:  offset,
		RewriteLimit:  step,
		RewriteOffset: 0,
		Step:          step,
	}
	if c.OriginOffset != nil {
		c.RewriteOffset = *c.OriginOffset
	}
	if c.OriginLimit != nil {
		if *c.OriginLimit <= c.Step {
			c.RewriteLimit = *c.OriginLimit
		}
	}
	c.GotLine = c.RewriteLimit
	return c
}

func NewInfiniteCursorWithStep(step int) Cursor {
	c := &cursor{
		Key:           genCursorKey(),
		OriginLimit:   nil,
		OriginOffset:  nil,
		RewriteLimit:  step,
		RewriteOffset: 0,
		Step:          step,
	}
	c.GotLine = c.RewriteLimit
	return c
}

func LoadCursor(dump string) (Cursor, error) {
	c := &cursor{}
	err := sonic.Unmarshal([]byte(dump), c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *cursor) Cursor() string {
	return c.Key
}

func (c *cursor) getOriginLimit() int {
	if c.OriginLimit != nil {
		return *c.OriginLimit
	}
	return 0
}

func (c *cursor) getOriginOffset() int {
	if c.OriginOffset != nil {
		return *c.OriginOffset
	}
	return 0
}

func (c *cursor) needLine() int {
	if c.getOriginLimit() != 0 {
		return c.getOriginLimit()
	}
	return infiniteLimit
}

func (c *cursor) alreadyGotLine() int {
	return c.GotLine
}

func (c *cursor) Next() Cursor {
	if c.alreadyGotLine() >= c.needLine() {
		// has already got all lines
		return nil
	}

	next := &cursor{
		Key:           genCursorKey(),
		OriginLimit:   c.OriginLimit,
		OriginOffset:  c.OriginOffset,
		RewriteOffset: c.RewriteOffset + c.RewriteLimit,
		Step:          c.Step,
	}
	if c.getOriginLimit() != 0 && c.getOriginLimit()-c.alreadyGotLine() < c.Step {
		next.RewriteLimit = c.getOriginLimit() - c.alreadyGotLine()
	} else {
		next.RewriteLimit = c.Step
	}
	next.GotLine = c.alreadyGotLine() + next.RewriteLimit
	return next
}

func (c *cursor) Dump() string {
	s, _ := sonic.Marshal(c)
	return string(s)
}

func (c *cursor) GetLimit() int {
	return c.RewriteLimit
}

func (c *cursor) GetOffset() int {
	return c.RewriteOffset
}

type Metadata struct {
	Cursor     `json:"-"`
	SQL        string `json:"sql"`
	CursorData string `json:"cursor_data"`
}

func (c *Metadata) Dump() string {
	c.CursorData = c.Cursor.Dump()
	s, _ := sonic.Marshal(c)
	return string(s)
}

func LoadMetadata(data string) (*Metadata, error) {
	if data == "" {
		return nil, errors.Errorf("empty metadata")
	}
	var c Metadata
	if err := sonic.UnmarshalString(data, &c); err != nil {
		return nil, err
	}

	cursor, err := LoadCursor(c.CursorData)
	if err != nil {
		return nil, errors.Wrapf(err, "load cursor failed")
	}
	c.Cursor = cursor
	return &c, nil
}
