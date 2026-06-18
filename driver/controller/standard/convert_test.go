package standard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/processor/protos"
)

func indexOf(s, substr string) int { return strings.Index(s, substr) }

func Test_ConvertTemplateInstance(t *testing.T) {
	hundred := uint64(100)

	// basic conversion without labels
	assert.Equal(t, []controller.TemplateInstance{
		{
			TemplateID:   1,
			TemplateName: "MyContract",
			Address:      "0x1111",
			Labels:       "",
			Removed:      false,
			BlockRange:   controller.BlockRange{StartBlock: 10},
		},
	}, ConvertTemplateInstance([]*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Name: "MyContract", Address: "0x1111"},
			TemplateId: 1,
			StartBlock: 10,
		},
	}, false))

	// with EndBlock
	assert.Equal(t, []controller.TemplateInstance{
		{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 10, EndBlock: &hundred},
		},
	}, ConvertTemplateInstance([]*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			TemplateId: 1,
			StartBlock: 10,
			EndBlock:   100,
		},
	}, false))

	// with remove=true
	assert.Equal(t, []controller.TemplateInstance{
		{
			TemplateID: 1,
			Address:    "0x1111",
			Removed:    true,
			BlockRange: controller.BlockRange{StartBlock: 10},
		},
	}, ConvertTemplateInstance([]*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			TemplateId: 1,
			StartBlock: 10,
		},
	}, true))

	// with BaseLabels — should be marshaled to JSON string
	labels, err := structpb.NewStruct(map[string]any{"pid": "abc", "foo": "bar"})
	assert.NoError(t, err)
	result := ConvertTemplateInstance([]*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			TemplateId: 1,
			StartBlock: 10,
			BaseLabels: labels,
		},
	}, false)
	assert.Len(t, result, 1)
	assert.Equal(t, `{"foo":"bar","pid":"abc"}`, result[0].Labels)

	// empty BaseLabels — Labels should be empty string
	emptyLabels, err := structpb.NewStruct(map[string]any{})
	assert.NoError(t, err)
	assert.Equal(t, []controller.TemplateInstance{
		{
			TemplateID: 1,
			Address:    "0x1111",
			Labels:     "",
			BlockRange: controller.BlockRange{StartBlock: 10},
		},
	}, ConvertTemplateInstance([]*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			TemplateId: 1,
			StartBlock: 10,
			BaseLabels: emptyLabels,
		},
	}, false))
}

// Test_ConvertTemplateInstance_LabelsMarshalStability verifies that BaseLabels with multiple
// keys always marshals to the same JSON string (keys sorted alphabetically by protojson),
// so the result can be used as a stable identity key in UniqID.
func Test_ConvertTemplateInstance_LabelsMarshalStability(t *testing.T) {
	// Build two Struct values with identical content but keys inserted in different orders.
	labelsABC, err := structpb.NewStruct(map[string]any{"a": "1", "b": "2", "c": "3"})
	assert.NoError(t, err)
	labelsCBA, err := structpb.NewStruct(map[string]any{"c": "3", "b": "2", "a": "1"})
	assert.NoError(t, err)

	convert := func(s *structpb.Struct) string {
		result := ConvertTemplateInstance([]*protos.TemplateInstance{
			{Contract: &protos.ContractInfo{Address: "0x1"}, TemplateId: 1, BaseLabels: s},
		}, false)
		return result[0].Labels
	}

	gotABC := convert(labelsABC)
	gotCBA := convert(labelsCBA)

	// Both must produce identical JSON regardless of insertion order.
	assert.Equal(t, gotABC, gotCBA)

	// No unnecessary whitespace — compact JSON only.
	assert.NotContains(t, gotABC, " ")

	// Keys must be sorted alphabetically (protojson guarantee): "a" before "b" before "c".
	assert.Contains(t, gotABC, `"a"`)
	assert.Contains(t, gotABC, `"b"`)
	assert.Contains(t, gotABC, `"c"`)
	assert.Less(t, indexOf(gotABC, `"a"`), indexOf(gotABC, `"b"`))
	assert.Less(t, indexOf(gotABC, `"b"`), indexOf(gotABC, `"c"`))

	// Run 20 more times to rule out map-iteration luck.
	for range 20 {
		assert.Equal(t, gotABC, convert(labelsABC))
		assert.Equal(t, gotABC, convert(labelsCBA))
	}
}

func Test_ConvertTemplateInstanceBack(t *testing.T) {
	assert.Equal(t, []*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			StartBlock: 10,
			EndBlock:   0,
			TemplateId: 1,
		},
	}, ConvertTemplateInstanceBack("", map[uint64][]controller.TemplateInstance{
		10: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 10},
		}},
	}))

	hundred := uint64(100)
	assert.Equal(t, []*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			StartBlock: 10,
			EndBlock:   100,
			TemplateId: 1,
		},
	}, ConvertTemplateInstanceBack("", map[uint64][]controller.TemplateInstance{
		10: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 10, EndBlock: &hundred},
		}},
	}))

	assert.Equal(t, []*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			StartBlock: 10,
			EndBlock:   19,
			TemplateId: 1,
		},
	}, ConvertTemplateInstanceBack("", map[uint64][]controller.TemplateInstance{
		10: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 10, EndBlock: &hundred},
		}},
		12: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 20},
			Removed:    true,
		}},
	}))

	assert.Equal(t, []*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			StartBlock: 10,
			EndBlock:   0,
			TemplateId: 1,
		},
	}, ConvertTemplateInstanceBack("", map[uint64][]controller.TemplateInstance{
		10: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 10, EndBlock: &hundred},
		}},
		12: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 20},
			Removed:    true,
		}},
		15: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 15},
		}},
	}))

	assert.Equal(t, []*protos.TemplateInstance{
		{
			Contract:   &protos.ContractInfo{Address: "0x1111"},
			StartBlock: 30,
			EndBlock:   0,
			TemplateId: 1,
		},
	}, ConvertTemplateInstanceBack("", map[uint64][]controller.TemplateInstance{
		10: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 10, EndBlock: &hundred},
		}},
		12: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 20},
			Removed:    true,
		}},
		15: {{
			TemplateID: 1,
			Address:    "0x1111",
			BlockRange: controller.BlockRange{StartBlock: 30},
		}},
	}))

	// BaseLabels round-trip: use proto.Equal to compare protobuf messages correctly.
	expectedLabels, err := structpb.NewStruct(map[string]any{"pid": "abc"})
	assert.NoError(t, err)
	gotBack := ConvertTemplateInstanceBack("", map[uint64][]controller.TemplateInstance{
		10: {{
			TemplateID: 1,
			Address:    "0x1111",
			Labels:     `{"pid":"abc"}`,
			BlockRange: controller.BlockRange{StartBlock: 10},
		}},
	})
	assert.Len(t, gotBack, 1)
	assert.Equal(t, int32(1), gotBack[0].TemplateId)
	assert.Equal(t, "0x1111", gotBack[0].Contract.GetAddress())
	assert.Equal(t, uint64(10), gotBack[0].StartBlock)
	assert.True(t, proto.Equal(expectedLabels, gotBack[0].BaseLabels))
}
