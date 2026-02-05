package ethereum

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/wasm"
)

func buildTypeMarshalingFromValue(val *Value) abi.ArgumentMarshaling {
	switch val.Kind {
	case ValueKindAddress:
		return abi.ArgumentMarshaling{Type: "address", InternalType: "address"}
	case ValueKindFixedBytes:
		typeStr := fmt.Sprintf("bytes%d", len(val.Value.(*wasm.ByteArray).Data))
		return abi.ArgumentMarshaling{Type: typeStr, InternalType: typeStr}
	case ValueKindBytes:
		return abi.ArgumentMarshaling{Type: "bytes", InternalType: "bytes"}
	case ValueKindInt:
		return abi.ArgumentMarshaling{Type: "int256", InternalType: "int256"}
	case ValueKindUint:
		return abi.ArgumentMarshaling{Type: "uint256", InternalType: "uint256"}
	case ValueKindBool:
		return abi.ArgumentMarshaling{Type: "bool", InternalType: "bool"}
	case ValueKindString:
		return abi.ArgumentMarshaling{Type: "string", InternalType: "string"}
	case ValueKindFixedArray, ValueKindArray:
		arr := val.Value.(*wasm.ObjectArray[*Value])
		var sliced string
		if val.Kind == ValueKindArray {
			sliced = "[]"
		} else {
			sliced = fmt.Sprintf("[%d]", len(arr.Data))
		}
		if len(arr.Data) == 0 {
			// unknown item type, just use string as item type
			return abi.ArgumentMarshaling{Type: "string" + sliced, InternalType: "string" + sliced}
		}
		itemType := buildTypeMarshalingFromValue(arr.Data[0])
		return abi.ArgumentMarshaling{
			Type:         itemType.Type + sliced,
			InternalType: itemType.InternalType + sliced,
			Components:   itemType.Components,
		}
	case ValueKindTuple:
		typ := abi.ArgumentMarshaling{
			Type:         "tuple",
			InternalType: "tuple",
		}
		for _, prop := range val.Value.(*Tuple).Data {
			propComponent := buildTypeMarshalingFromValue(prop)
			propComponent.Name = fmt.Sprintf("_p%d", len(typ.Components))
			typ.Components = append(typ.Components, propComponent)
		}
		return typ
	default:
		panic(errors.Errorf("unknown value kind %d", val.Kind))
	}
}

func Encode(val *Value) ([]byte, error) {
	typeMarshaling := buildTypeMarshalingFromValue(val)
	typ, err := abi.NewType(typeMarshaling.Type, typeMarshaling.InternalType, typeMarshaling.Components)
	if err != nil {
		return nil, err
	}
	args := abi.Arguments{{Type: typ}}
	v := val.ToGoType(typ)
	return args.Pack(v)
}
