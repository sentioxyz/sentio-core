package ethereum

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/subgraph/common"
	"testing"
)

func Test_buildTypeMarshalingFromValue(t *testing.T) {
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "address",
		InternalType: "address",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindAddress,
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "int256",
		InternalType: "int256",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindInt,
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "uint256",
		InternalType: "uint256",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindUint,
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "bool",
		InternalType: "bool",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindBool,
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "string",
		InternalType: "string",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindString,
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "bytes32",
		InternalType: "bytes32",
	}, buildTypeMarshalingFromValue(&Value{
		Kind:  ValueKindFixedBytes,
		Value: &wasm.ByteArray{Data: make([]byte, 32)},
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "bytes",
		InternalType: "bytes",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindBytes,
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "bool[32]",
		InternalType: "bool[32]",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindFixedArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{
			{Kind: ValueKindBool}, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil,
			nil, nil, nil, nil, nil, nil, nil, nil,
		}},
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "address[2]",
		InternalType: "address[2]",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindFixedArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
			Kind: ValueKindAddress,
		}, {
			Kind: ValueKindAddress,
		}}},
	}))

	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "address[]",
		InternalType: "address[]",
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
			Kind: ValueKindAddress,
		}, {
			Kind: ValueKindAddress,
		}}},
	}))

	// (address)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple",
		InternalType: "tuple",
		Components: []abi.ArgumentMarshaling{{
			Name:         "_p0",
			Type:         "address",
			InternalType: "address",
		}},
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindTuple,
		Value: NewTuple(&Value{
			Kind: ValueKindAddress,
		}),
	}))

	// (address, int256)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple",
		InternalType: "tuple",
		Components: []abi.ArgumentMarshaling{{
			Name:         "_p0",
			Type:         "address",
			InternalType: "address",
		}, {
			Name:         "_p1",
			Type:         "int256",
			InternalType: "int256",
		}},
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindTuple,
		Value: NewTuple(&Value{
			Kind: ValueKindAddress,
		}, &Value{
			Kind: ValueKindInt,
		}),
	}))

	// (address, int256)[]
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple[]",
		InternalType: "tuple[]",
		Components: []abi.ArgumentMarshaling{{
			Name:         "_p0",
			Type:         "address",
			InternalType: "address",
		}, {
			Name:         "_p1",
			Type:         "int256",
			InternalType: "int256",
		}},
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
			Kind: ValueKindTuple,
			Value: NewTuple(&Value{
				Kind: ValueKindAddress,
			}, &Value{
				Kind: ValueKindInt,
			}),
		}}},
	}))

	//(address,uint256,(address,bool),(int256,bool)[])[]
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple[]",
		InternalType: "tuple[]",
		Components: []abi.ArgumentMarshaling{{
			Name:         "_p0",
			Type:         "address",
			InternalType: "address",
		}, {
			Name:         "_p1",
			Type:         "int256",
			InternalType: "int256",
		}, {
			Name:         "_p2",
			Type:         "tuple",
			InternalType: "tuple",
			Components: []abi.ArgumentMarshaling{{
				Name:         "_p0",
				Type:         "address",
				InternalType: "address",
			}, {
				Name:         "_p1",
				Type:         "bool",
				InternalType: "bool",
			}},
		}, {
			Name:         "_p3",
			Type:         "tuple[]",
			InternalType: "tuple[]",
			Components: []abi.ArgumentMarshaling{{
				Name:         "_p0",
				Type:         "int256",
				InternalType: "int256",
			}, {
				Name:         "_p1",
				Type:         "bool",
				InternalType: "bool",
			}},
		}},
	}, buildTypeMarshalingFromValue(&Value{
		Kind: ValueKindArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
			Kind: ValueKindTuple,
			Value: NewTuple(&Value{
				Kind: ValueKindAddress,
			}, &Value{
				Kind: ValueKindInt,
			}, &Value{
				Kind: ValueKindTuple,
				Value: NewTuple(&Value{
					Kind: ValueKindAddress,
				}, &Value{
					Kind: ValueKindBool,
				}),
			}, &Value{
				Kind: ValueKindArray,
				Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
					Kind: ValueKindTuple,
					Value: NewTuple(&Value{
						Kind: ValueKindInt,
					}, &Value{
						Kind: ValueKindBool,
					}),
				}}},
			}),
		}}},
	}))
}

func Test_encode(t *testing.T) {
	b, err := Encode(&Value{
		Kind: ValueKindArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
			Kind: ValueKindTuple,
			Value: NewTuple(&Value{
				Kind:  ValueKindAddress,
				Value: common.MustBuildAddressFromString("0xed8cd61b0bbce923134fffb58b4ffa07ec641972"),
			}, &Value{
				Kind:  ValueKindInt,
				Value: common.MustBuildBigInt(199),
			}),
		}}},
	})
	result := wasm.ByteArray{Data: b}
	assert.NoError(t, err)
	assert.Equal(t,
		"0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000001000000000000000000000000ed8cd61b0bbce923134fffb58b4ffa07ec64197200000000000000000000000000000000000000000000000000000000000000c7",
		result.String())
}

func Test_encodeNoError(t *testing.T) {
	testcases := []*Value{{
		Kind:  ValueKindAddress,
		Value: common.MustBuildAddressFromString("0x0102030405060708090a0b0c0d0e0f1011121314"),
	}, {
		Kind:  ValueKindFixedBytes,
		Value: wasm.MustBuildByteArrayFromHex("0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
	}, {
		Kind:  ValueKindBytes,
		Value: wasm.MustBuildByteArrayFromHex("0x000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"),
	}, {
		Kind:  ValueKindInt,
		Value: common.MustBuildBigInt(1234),
	}, {
		Kind:  ValueKindUint,
		Value: common.MustBuildBigInt(1234),
	}, {
		Kind:  ValueKindBool,
		Value: wasm.Bool(false),
	}, {
		Kind:  ValueKindString,
		Value: wasm.BuildString("abc"),
	}, {
		Kind: ValueKindFixedArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
			Kind:  ValueKindString,
			Value: wasm.BuildString("aaa"),
		}, {
			Kind:  ValueKindString,
			Value: wasm.BuildString("bbb"),
		}}},
	}, {
		Kind: ValueKindArray,
		Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
			Kind:  ValueKindString,
			Value: wasm.BuildString("aaa"),
		}, {
			Kind:  ValueKindString,
			Value: wasm.BuildString("bbb"),
		}}},
	}, {
		Kind: ValueKindTuple,
		Value: NewTuple(
			&Value{
				Kind:  ValueKindString,
				Value: wasm.BuildString("aaa"),
			},
			&Value{
				Kind:  ValueKindBool,
				Value: wasm.Bool(true),
			},
		),
	}}

	for i, testcase := range testcases {
		title := fmt.Sprintf("#%d", i)
		_, err := Encode(testcase)
		assert.NoError(t, err, title)
	}
}
