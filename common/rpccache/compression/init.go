package compression

import (
	"encoding/gob"

	protoscommon "sentioxyz/sentio-core/service/common/protos"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	gob.Register(&protoscommon.Any_StringValue{})
	gob.Register(&protoscommon.Any_IntValue{})
	gob.Register(&protoscommon.Any_LongValue{})
	gob.Register(&protoscommon.Any_DoubleValue{})
	gob.Register(&protoscommon.Any_BoolValue{})
	gob.Register(&protoscommon.Any_DateValue{})
	gob.Register(&protoscommon.Any_ListValue{})
	gob.Register(&protoscommon.Matrix_Sample{})
	gob.Register(&protoscommon.Matrix_Metric{})
	gob.Register(&protoscommon.Matrix_Value{})
	gob.Register(&protoscommon.CoinID_AddressIdentifier{})
	gob.Register(&protoscommon.CoinID{})
	gob.Register(&protoscommon.Contract{})
	gob.Register(&protoscommon.CoinID_Symbol{})
	gob.Register(&protoscommon.TabularData{})
	gob.Register(&protoscommon.RetentionMatrix{})
	gob.Register(&protoscommon.RetentionMatrix_Sample{})
	gob.Register(&protoscommon.SegmentParameter{})
	gob.Register(&protoscommon.ComputeStats{})
	gob.Register(&protoscommon.ComputeStats_ClickhouseStats{})
	gob.Register(&protoscommon.CachePolicy{})
	gob.Register(&protoscommon.EventLogEntry{})
	gob.Register(&protoscommon.Any{})

	// structpb
	gob.Register(&structpb.Value_StringValue{})
	gob.Register(&structpb.Value_NumberValue{})
	gob.Register(&structpb.Value_BoolValue{})
	gob.Register(&structpb.Value_NullValue{})
	gob.Register(&structpb.Value_StructValue{})
	gob.Register(&structpb.Value_ListValue{})
	gob.Register(&structpb.Struct{})
	gob.Register(&structpb.ListValue{})
	gob.Register(&structpb.Value{})
	gob.Register(&timestamppb.Timestamp{})
}
