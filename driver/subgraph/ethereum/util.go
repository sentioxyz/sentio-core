package ethereum

import (
	"google.golang.org/protobuf/types/known/structpb"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/subgraph/common"
)

func isNullValue(value *structpb.Value) bool {
	if value == nil {
		return true
	}
	_, is := value.GetKind().(*structpb.Value_NullValue)
	return is
}

func MustBuildByteArrayFromHex(value *structpb.Value) *wasm.ByteArray {
	if isNullValue(value) {
		return nil
	}
	return wasm.MustBuildByteArrayFromHex(value.GetStringValue())
}

func MustBuildAddressFromString(value *structpb.Value) *common.Address {
	if isNullValue(value) {
		return nil
	}
	return common.MustBuildAddressFromString(value.GetStringValue())
}

func MustBuildBigIntFromHex(value *structpb.Value) *common.BigInt {
	if isNullValue(value) {
		return nil
	}
	return common.MustBuildBigInt(value.GetStringValue())
}
