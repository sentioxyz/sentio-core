package serde

import (
	"io"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
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

// selectorValue holds an optional unscoped (global) default plus per-selector
// overrides for a single `bcs` tag attribute. See bcs_enum_selector_design.md.
type selectorValue[T any] struct {
	global     *T
	bySelector map[string]T
}

// resolve returns the effective value for selector, applying scoped-over-global
// precedence; ok is false when the attribute is unset for that selector.
func (sv selectorValue[T]) resolve(selector string) (v T, ok bool) {
	if sv.bySelector != nil {
		if x, found := sv.bySelector[selector]; found {
			return x, true
		}
	}
	if sv.global != nil {
		return *sv.global, true
	}
	return v, false
}

func (sv selectorValue[T]) any() bool {
	return sv.global != nil || len(sv.bySelector) > 0
}

func (sv *selectorValue[T]) set(selector string, v T) {
	if selector == "" {
		sv.global = &v
		return
	}
	if sv.bySelector == nil {
		sv.bySelector = map[string]T{}
	}
	sv.bySelector[selector] = v
}

// fieldTag is the parsed `bcs` struct tag. Every attribute is per-selector
// capable with a uniform scoped-over-global resolution rule.
type fieldTag struct {
	ignore   selectorValue[bool]
	optional selectorValue[bool]
	enumNum  selectorValue[int]
}

func (t fieldTag) isIgnored(selector string) bool {
	v, ok := t.ignore.resolve(selector)
	return ok && v
}

func (t fieldTag) isOptional(selector string) bool {
	v, ok := t.optional.resolve(selector)
	return ok && v
}

func (t fieldTag) variantNum(selector string) (int, bool) {
	return t.enumNum.resolve(selector)
}

func (t fieldTag) hasAnyEnumNum() bool {
	return t.enumNum.any()
}

// parseTag parses a `bcs` tag. Segments are comma-separated; each is an
// attribute optionally scoped with "[selector]" and optionally with "=value".
// Examples: "-", "-[sui]", "optional", "optional[iota]", "enumNum=3",
// "enumNum[sui]=2,enumNum[iota]=1".
func parseTag(tag string) (fieldTag, error) {
	var ft fieldTag
	for _, seg := range strings.Split(tag, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		attrPart, valPart, hasVal := strings.Cut(seg, "=")
		attrPart = strings.TrimSpace(attrPart)

		name := attrPart
		selector := ""
		if i := strings.IndexByte(attrPart, '['); i >= 0 {
			if !strings.HasSuffix(attrPart, "]") {
				return ft, errors.Errorf("malformed tag segment %q in %q", seg, tag)
			}
			name = attrPart[:i]
			selector = attrPart[i+1 : len(attrPart)-1]
		}

		switch name {
		case "-":
			if hasVal {
				return ft, errors.Errorf("'-' takes no value in %q", seg)
			}
			ft.ignore.set(selector, true)
		case "optional":
			if hasVal {
				return ft, errors.Errorf("'optional' takes no value in %q", seg)
			}
			ft.optional.set(selector, true)
		case "enumNum":
			if !hasVal {
				return ft, errors.Errorf("'enumNum' requires a value in %q", seg)
			}
			n, err := strconv.Atoi(strings.TrimSpace(valPart))
			if err != nil {
				return ft, errors.Wrapf(err, "invalid enumNum value in %q", seg)
			}
			ft.enumNum.set(selector, n)
		default:
			return ft, errors.Errorf("unknown tag: %s in %s", name, tag)
		}
	}
	return ft, nil
}

// enumLayout is the resolved variant mapping of an enum struct for one selector.
// tagged is true when the type uses enumNum tags at all (in which case position
// mode is disabled). byVariant/byField are populated only with the variants that
// exist under the selector.
type enumLayout struct {
	tagged    bool
	byVariant map[int]int // variant index -> field index
	byField   map[int]int // field index -> variant index
}

type enumLayoutKey struct {
	t        reflect.Type
	selector string
}

var enumLayoutCache sync.Map // enumLayoutKey -> *enumLayout | error

func enumLayoutFor(t reflect.Type, selector string) (*enumLayout, error) {
	key := enumLayoutKey{t, selector}
	if cached, ok := enumLayoutCache.Load(key); ok {
		if l, ok := cached.(*enumLayout); ok {
			return l, nil
		}
		return nil, cached.(error)
	}
	l, err := buildEnumLayout(t, selector)
	if err != nil {
		enumLayoutCache.Store(key, err)
		return nil, err
	}
	enumLayoutCache.Store(key, l)
	return l, nil
}

func buildEnumLayout(t reflect.Type, selector string) (*enumLayout, error) {
	l := &enumLayout{byVariant: map[int]int{}, byField: map[int]int{}}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		ft, err := parseTag(f.Tag.Get(tagName))
		if err != nil {
			return nil, err
		}
		if ft.hasAnyEnumNum() {
			l.tagged = true
		}
		if n, ok := ft.variantNum(selector); ok {
			if prev, dup := l.byVariant[n]; dup {
				return nil, errors.Errorf("duplicate enumNum %d for selector %q on %s (fields %s and %s)",
					n, selector, t, t.Field(prev).Name, f.Name)
			}
			l.byVariant[n] = i
			l.byField[i] = n
		}
	}
	return l, nil
}
