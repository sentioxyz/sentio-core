package move

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type FullyName struct {
	Address string // empty string means *, always be short address
	Module  string // empty string means *
	Name    string // empty string means *
}

func (f FullyName) Include(a FullyName) bool {
	return (f.Address == "" || f.Address == a.Address) &&
		(f.Module == "" || f.Module == a.Module) &&
		(f.Name == "" || f.Name == a.Name)
}

func (f FullyName) HasAny() bool {
	return f.Address == "" || f.Module == "" || f.Name == ""
}

func (f FullyName) String() string {
	var b bytes.Buffer
	if f.Address != "" {
		b.WriteString(f.Address)
	} else {
		b.WriteString("*")
	}
	b.WriteString("::")
	if f.Module != "" {
		b.WriteString(f.Module)
	} else {
		b.WriteString("*")
	}
	b.WriteString("::")
	if f.Name != "" {
		b.WriteString(f.Name)
	} else {
		b.WriteString("*")
	}
	return b.String()
}

// Type is a type ${Main}<${Args[0]},${Args[1]},...>
type Type struct {
	// main part
	// Simple and FQN are both nil means main part is `any`
	Simple *string
	FQN    *FullyName

	Args TypeArgs
}

func (t Type) Main() string {
	if t.Simple != nil {
		return *t.Simple
	}
	if t.FQN != nil {
		return t.FQN.String()
	}
	return "*"
}

func (t Type) MainHasAny() bool {
	if t.Simple != nil {
		return false
	}
	if t.FQN != nil {
		return t.FQN.HasAny()
	}
	return true
}

func (t Type) String() string {
	return t.Main() + t.Args.String()
}

func (t *Type) Equal(a *Type) bool {
	if t == nil && a == nil {
		return true
	}
	if t != nil && a != nil {
		return t.Main() == a.Main() && t.Args.Equal(a.Args)
	}
	return false
}

func (t Type) IsAny() bool {
	return t.Simple == nil && t.FQN == nil && len(t.Args) == 0
}

func (t Type) HasAny() bool {
	return t.MainHasAny() || t.Args.HasAny()
}

func (t Type) IncludeTypeString(s *string) bool {
	if t.IsAny() {
		return true
	}
	if s == nil {
		return false
	}
	a, err := BuildType(*s)
	if err != nil {
		return false // invalid type
	}
	return t.Include(a)
}

func (t Type) IncludeBy(a Type) bool {
	return a.Include(t)
}

func (t Type) Include(a Type) bool {
	// check main part
	if t.Simple != nil {
		if a.Simple == nil || *t.Simple != *a.Simple {
			return false
		}
	}
	if t.FQN != nil {
		if a.FQN == nil || !t.FQN.Include(*a.FQN) {
			return false
		}
	}
	// check args part
	if len(t.Args) == 0 {
		return true
	}
	if len(t.Args) != len(a.Args) {
		return false
	}
	for i := range t.Args {
		if !t.Args[i].Include(a.Args[i]) {
			return false
		}
	}
	return true
}

func (t Type) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

func (t *Type) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if v, err := BuildType(s); err != nil {
		return err
	} else {
		*t = v
	}
	return nil
}

func BuildType(s string) (t Type, err error) {
	var mainPart string
	if p := strings.IndexRune(s, '<'); p >= 0 {
		mainPart = strings.TrimSpace(s[:p])
		t.Args, err = parseArgs(s[p:])
		if err != nil {
			return Type{}, err
		}
	} else {
		mainPart = strings.TrimSpace(s)
	}
	if address, ex, has := strings.Cut(mainPart, "::"); has {
		module, name, _ := strings.Cut(ex, "::")
		t.FQN = &FullyName{
			Address: ToShortAddress(strings.TrimSpace(address)),
			Module:  strings.TrimSpace(module),
			Name:    strings.TrimSpace(name),
		}
		if t.FQN.Address == "*" {
			t.FQN.Address = ""
		}
		if t.FQN.Module == "*" {
			t.FQN.Module = ""
		}
		if t.FQN.Name == "*" {
			t.FQN.Name = ""
		}
		return t, nil
	}
	if mainPart != "any" && mainPart != "*" && mainPart != "" {
		t.Simple = &mainPart
		return t, nil
	}
	return
}

func MustBuildType(typ string, what ...string) Type {
	t, err := BuildType(typ)
	if err != nil {
		if len(what) == 0 {
			panic(fmt.Errorf("build move type from %q failed: %w", typ, err))
		}
		panic(fmt.Errorf("build move type from %q for %s failed: %w", typ, what[0], err))
	}
	return t
}

type TypeArgs []Type

func (t TypeArgs) String() string {
	if len(t) == 0 {
		return ""
	}
	return fmt.Sprintf("<%s>", strings.Join(utils.MapSliceNoError(t, Type.String), ","))
}

func (t TypeArgs) HasAny() bool {
	return utils.HasAny(t, Type.HasAny)
}

func (t TypeArgs) Equal(a TypeArgs) bool {
	if len(t) != len(a) {
		return false
	}
	for i := range t {
		if !t[i].Equal(&a[i]) {
			return false
		}
	}
	return true
}

func (t TypeArgs) Include(a TypeArgs) bool {
	if len(t) == 0 {
		return true
	}
	if len(t) != len(a) {
		return false
	}
	for i := range t {
		if !t[i].Include(a[i]) {
			return false
		}
	}
	return true
}

// txt should be the format of '<?,?,?>'
func parseArgs(txt string) (args TypeArgs, err error) {
	txt = strings.TrimSpace(txt)
	switch len(txt) {
	case 0:
		return nil, nil
	case 1:
		return nil, errors.Errorf("invalid type args %q: should be wrapped in <>", txt)
	}
	if txt[0] != '<' || txt[len(txt)-1] != '>' {
		return nil, errors.Errorf("invalid type args %q: should be wrapped in <>", txt)
	}
	txt = txt[1 : len(txt)-1]
	var lvl int
	var parts []string
	var s int
	for i := 0; i < len(txt); i++ {
		switch txt[i] {
		case '<':
			lvl++
		case '>':
			lvl--
			if lvl < 0 {
				return nil, errors.Errorf("invalid type args %q: missing '<'", txt)
			}
		case ',':
			if lvl == 0 {
				parts = append(parts, txt[s:i])
				s = i + 1
			}
		}
	}
	if lvl > 0 {
		return nil, errors.Errorf("invalid type args %q: missing '>'", txt)
	}
	parts = append(parts, txt[s:])
	args = make(TypeArgs, len(parts))
	for i, part := range parts {
		if args[i], err = BuildType(part); err != nil {
			return nil, err
		}
	}
	return args, nil
}

type TypeSet []Type

func (s TypeSet) String() string {
	return strings.Join(utils.MapSliceNoError(s, Type.String), "|")
}

func (s TypeSet) Include(t Type) bool {
	return utils.HasAny(s, t.IncludeBy)
}

func (s TypeSet) Equal(a TypeSet) bool {
	if len(s) != len(a) {
		return false
	}
	for i := range s {
		if !s[i].Equal(&a[i]) {
			return false
		}
	}
	return true
}

func (s TypeSet) IncludeTypeString(t *string) bool {
	return utils.HasAny(s, func(x Type) bool {
		return x.IncludeTypeString(t)
	})
}

func (s TypeSet) Merge(a TypeSet) TypeSet {
	var ra TypeSet = utils.FilterArr(a, func(t Type) bool {
		return !s.Include(t)
	})
	var rs TypeSet = utils.FilterArr(s, func(t Type) bool {
		return !ra.Include(t)
	})
	return append(rs, ra...)
}
