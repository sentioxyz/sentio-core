package ethereum

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/common/wasm"
	"testing"
)

func Test_buildTypeMarshalingFromString(t *testing.T) {
	var typ abi.ArgumentMarshaling
	var err error

	typ, err = buildTypeMarshalingFromString("address")
	assert.NoError(t, err)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "address",
		InternalType: "address",
	}, typ)

	typ, err = buildTypeMarshalingFromString("int256")
	assert.NoError(t, err)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "int256",
		InternalType: "int256",
	}, typ)

	typ, err = buildTypeMarshalingFromString("(address)")
	assert.NoError(t, err)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple",
		InternalType: "tuple",
		Components: []abi.ArgumentMarshaling{
			{
				Name:         "_p0",
				Type:         "address",
				InternalType: "address",
			},
		},
	}, typ)

	typ, err = buildTypeMarshalingFromString("(address,uint256)")
	assert.NoError(t, err)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple",
		InternalType: "tuple",
		Components: []abi.ArgumentMarshaling{
			{
				Name:         "_p0",
				Type:         "address",
				InternalType: "address",
			},
			{
				Name:         "_p1",
				Type:         "uint256",
				InternalType: "uint256",
			},
		},
	}, typ)

	typ, err = buildTypeMarshalingFromString("(address,uint256)[]")
	assert.NoError(t, err)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple[]",
		InternalType: "tuple[]",
		Components: []abi.ArgumentMarshaling{
			{
				Name:         "_p0",
				Type:         "address",
				InternalType: "address",
			},
			{
				Name:         "_p1",
				Type:         "uint256",
				InternalType: "uint256",
			},
		},
	}, typ)

	typ, err = buildTypeMarshalingFromString("(address,uint256,(address,bool),(int256,bool)[])[]")
	assert.NoError(t, err)
	assert.Equal(t, abi.ArgumentMarshaling{
		Type:         "tuple[]",
		InternalType: "tuple[]",
		Components: []abi.ArgumentMarshaling{
			{
				Name:         "_p0",
				Type:         "address",
				InternalType: "address",
			},
			{
				Name:         "_p1",
				Type:         "uint256",
				InternalType: "uint256",
			},
			{
				Name:         "_p2",
				Type:         "tuple",
				InternalType: "tuple",
				Components: []abi.ArgumentMarshaling{
					{
						Name:         "_p0",
						Type:         "address",
						InternalType: "address",
					},
					{
						Name:         "_p1",
						Type:         "bool",
						InternalType: "bool",
					},
				},
			},
			{
				Name:         "_p3",
				Type:         "tuple[]",
				InternalType: "tuple[]",
				Components: []abi.ArgumentMarshaling{
					{
						Name:         "_p0",
						Type:         "int256",
						InternalType: "int256",
					},
					{
						Name:         "_p1",
						Type:         "bool",
						InternalType: "bool",
					},
				},
			},
		},
	}, typ)

}

func Test_decode(t *testing.T) {
	data := "0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000001000000000000000000000000ed8cd61b0bbce923134fffb58b4ffa07ec64197200000000000000000000000000000000000000000000000000000000000000c7"
	d := wasm.MustBuildByteArrayFromHex(data)
	val, err := Decode("(address,uint256)[]", d.Data)
	assert.NoError(t, err)
	assert.Equal(t, "Array[Tuple[Address[0xed8cd61b0bbce923134fffb58b4ffa07ec641972], UInt[199]]]", val.String())
}
