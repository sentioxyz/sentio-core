package ethereum

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"
)

func buildTypeMarshalingFromString(str string) (abi.ArgumentMarshaling, error) {
	//fmt.Printf("buildTypeMarshalingFromString: %s\n", str)
	if str[0] == '(' {
		var lvl int
		var sliced string
		var typ abi.ArgumentMarshaling
		for s, i := 1, 1; i < len(str) && lvl >= 0; i++ {
			switch str[i] {
			case '(':
				lvl++
			case ')':
				lvl--
				sliced = str[i+1:]
			case ',':
			default:
				continue
			}
			if lvl < 0 || (lvl == 0 && str[i] == ',') {
				prop, err := buildTypeMarshalingFromString(str[s:i])
				if err != nil {
					return typ, err
				}
				prop.Name = fmt.Sprintf("_p%d", len(typ.Components))
				typ.Components = append(typ.Components, prop)
				s = i + 1
			}
		}
		if lvl >= 0 {
			return abi.ArgumentMarshaling{}, errors.Errorf("invalid type string %q", str)
		}
		typ.Type = "tuple" + sliced
		typ.InternalType = "tuple" + sliced
		return typ, nil
	}

	return abi.ArgumentMarshaling{Type: str, InternalType: str}, nil
}

func Decode(typeStr string, data []byte) (*Value, error) {
	typeMarshaling, err := buildTypeMarshalingFromString(typeStr)
	if err != nil {
		return nil, err
	}
	typ, err := abi.NewType(typeMarshaling.Type, typeMarshaling.InternalType, typeMarshaling.Components)
	if err != nil {
		return nil, err
	}
	args := abi.Arguments{{Type: typ}}
	val, err := args.Unpack(data)
	if err != nil {
		return nil, err
	}

	retVal := &Value{}
	retVal.FromGoType(val[0], typ)
	return retVal, nil
}
