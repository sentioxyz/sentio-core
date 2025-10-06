package format

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

func Format(format string, args map[string]any) string {
	var printFormat bytes.Buffer
	var printArgs []any
	orig := []rune(format)
	origLen := len(orig)
	var p int
	for p < origLen {
		printFormat.WriteRune(orig[p])
		if orig[p] == '%' && p+1 < origLen {
			switch orig[p+1] {
			case '%':
				printFormat.WriteRune('%')
				p += 2
			default:
				var q = p + 1
				for q < origLen && orig[q] != '#' {
					q++
				}
				if q >= origLen {
					panic(fmt.Errorf("unterminated parameter name start from position %d of format string %q", p+1, format))
				}
				argName := string(orig[p+1 : q])
				printArg, has := args[argName]
				if !has {
					panic(fmt.Errorf("unknown parameter %q in format string %q", argName, format))
				}
				printArgs = append(printArgs, printArg)
				p = q + 1
			}
		} else {
			p++
		}
	}
	return fmt.Sprintf(printFormat.String(), printArgs...)
}

// FormatV2 use the slot like '$Property', which 'Property' is the key of a map[string]string
// or a property name of an Struct.
// The performance is not very good, so don’t use it in scenarios with high performance requirements.
// example:
//
//	FormatV2("#$P1-$P2-$P3", ma[string]string{"P1":"abc","P2":"123"}) returns "#abc-123-$P3"
func FormatV2(tpl string, params ...any) string {
	var s = tpl
	for _, param := range params {
		t := reflect.TypeOf(param)
		switch t.Kind() {
		case reflect.Struct:
			p := reflect.ValueOf(param)
			for i := 0; i < p.NumField(); i++ {
				name := t.Field(i).Name
				value := fmt.Sprintf("%v", p.Field(i).Interface())
				s = strings.ReplaceAll(s, "$"+name, value)
			}
		case reflect.Map:
			p := param.(map[string]string)
			for name, value := range p {
				s = strings.ReplaceAll(s, "$"+name, value)
			}
		default:
			panic(fmt.Errorf("invalid param %#v", param))
		}
	}
	return s
}
