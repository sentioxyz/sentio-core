package types

import (
	"bytes"
	"testing"

	"github.com/goccy/go-json"
	"github.com/kinbiko/jsonassert"

	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/sui/types/serde"
)

func TestStructTagToString(t *testing.T) {
	s := StructTag{
		Address: StrToAddressMust("0x3"),
		Module:  "validator",
		Name:    "StakingRequestEvent",
	}.String()
	assert.Equal(t, "0x3::validator::StakingRequestEvent", s)

	s = StructTag{
		Address: StrToAddressMust("0x3"),
		Module:  "validator",
		Name:    "StakingRequestEvent",
		TypeArgs: []TypeTag{
			{U64: true},
			{Address: true},
			{Struct: &StructTag{
				Address: StrToAddressMust("0x1"),
				Module:  "SUI",
				Name:    "coin",
			}},
		},
	}.String()
	assert.Equal(t, "0x3::validator::StakingRequestEvent<u64, address, 0x1::SUI::coin>", s)
}

func TestStructTagToString2(t *testing.T) {
	{
		s := "0xa283fd6b45f1103176e7ae27e870c89df7c8783b15345e2b13faa81ec25c4fa6"
		r, err := TypeTagFromString(s)
		assert.NoError(t, err)
		assert.Equal(t, "0xa283fd6b45f1103176e7ae27e870c89df7c8783b15345e2b13faa81ec25c4fa6", r.String())
	}
	{
		s := "0xa283fd6b45f1103176e7ae27e870c89df7c8783b15345e2b13faa81ec25c4fa6::abc"
		r, err := TypeTagFromString(s)
		assert.NoError(t, err)
		assert.Equal(t, "0xa283fd6b45f1103176e7ae27e870c89df7c8783b15345e2b13faa81ec25c4fa6::abc", r.String())
	}

}

func TestStringToStructTag(t *testing.T) {
	testOne := func(s string) {
		tt, err := StructTagFromString(s)
		assert.NoError(t, err)
		assert.Equal(t, s, tt.String())
	}

	testOne("0x3::validator::StakingRequestEvent<vector<u64>, address, 0x3::validator::XXX<0x5::SUI::coin>>")
	testOne(
		"0x5f866ded38f80220ef55221905e766ac40e626258841874357fbf1c633e5ed25::dex_volatile::VLPCoin<0x5f866ded38f80220ef55221905e766ac40e626258841874357fbf1c633e5ed25::coins::ETH, 0x5f866ded38f80220ef55221905e766ac40e626258841874357fbf1c633e5ed25::ipx::IPX>",
	)
	testOne(
		"0xee496a0cc04d06a345982ba6697c90c619020de9e274408c7819f787ff66e1a1::suifrens::SuiFren<0xee496a0cc04d06a345982ba6697c90c619020de9e274408c7819f787ff66e1a1::capy::Capy>0xee496a0cc04d06a345982ba6697c90c619020de9e274408c7819f787ff66e1a1::suifrens::SuiFren>",
	)
	testOne(
		"0xa4f76f898d8a202e41c14c683f7a69b56fc4ef3d6e696d61dc949fa2df50b615::stake::EventUnstake<0x7cf8b2a8743ce58ba404d87a28ccb1db16b532702adcb3353cd4ffab73b367e::liquidity_pool::LPToken<0x13d46aa812092673a05df4ee3e67445c03af67b8f33d23c888c03e4e6c00a4ac::dai::DAI, 0x13d46aa812092673a05df4ee3e67445c03af67b8f33d23c888c03e4e6c00a4ac::usdt::USDT, 0x7cf8b2a8743ce58ba404d87a28ccb1db16b532702adcb3353cd4ffab73b367e::curves::Stable>, 0x13d46aa812092673a05df4ee3e67445c03af67b8f33d23c888c03e4e6c00a4ac::bswt::BSWT>",
	)
}

func TestPureValueSerialize(t *testing.T) {
	a := StrToAddressMust("0xaa31171e5bf7e30ad8acef62bea3d0a38308906bf2a17c73526156d2f407cdf5")
	testCases := [][]interface{}{
		{"vector<u8>", json.RawMessage(`"hello world"`), []byte("hello world")},
		{"u64", json.RawMessage(`"1999555"`), uint64(1999555)},
		{"u16", json.RawMessage(`1995`), uint16(1995)},
		{"address", json.RawMessage(`"0xaa31171e5bf7e30ad8acef62bea3d0a38308906bf2a17c73526156d2f407cdf5"`), &a},
	}
	for _, tc := range testCases {
		tt, err := TypeTagFromString(tc[0].(string))
		assert.NoError(t, err)
		b1, err := tt.SerializeBCS(tc[1].(json.RawMessage))
		assert.NoError(t, err)
		buf := bytes.NewBuffer(nil)
		err = serde.Encode(buf, tc[2])
		assert.NoError(t, err)
		b2 := buf.Bytes()
		assert.Equal(t, b1, b2)
	}
}

func TestMatchIgnoreGeneric(t *testing.T) {
	s0, err := TypeTagFromString("0x1111111111111111111111111111111111111111::SUI::coin")
	assert.NoError(t, err)
	s1, err := TypeTagFromString("0x4c10b61966a34d3bb5c8a8f063e6b7445fc41f93::SUI::coin")
	assert.NoError(t, err)
	s2, err := TypeTagFromString("0x4c10b61966a34d3bb5c8a8f063e6b7445fc41f93::SUI::coin<vector<u64>>")
	assert.NoError(t, err)
	s3, err := TypeTagFromString("0x4c10b61966a34d3bb5c8a8f063e6b7445fc41f93::SUI::coin<address, vector<u64>>")
	assert.NoError(t, err)
	s4, err := TypeTagFromString("0x4c10b61966a34d3bb5c8a8f063e6b7445fc41f93::capy::ItemAdded")
	assert.NoError(t, err)
	assert.False(t, s0.Include(*s1))
	assert.True(t, s1.Include(*s2))
	assert.True(t, s1.Include(*s3))
	assert.False(t, s1.Include(*s4))
	assert.False(t, s2.Include(*s1))
	assert.False(t, s2.Include(*s3))
	assert.False(t, s2.Include(*s4))
	assert.False(t, s3.Include(*s1))
	assert.False(t, s3.Include(*s2))
	assert.False(t, s3.Include(*s4))
	assert.False(t, s4.Include(*s1))
	assert.False(t, s4.Include(*s2))
	assert.False(t, s4.Include(*s3))
}

func TestUnmarshalAnyType(t *testing.T) {
	s, err := TypeTagFromString("0x2::coin::Coin<any>")
	assert.NoError(t, err)
	assert.Equal(t, &TypeTag{
		Struct: &StructTag{
			Address: StrToAddressMust("0x2"),
			Module:  "coin",
			Name:    "Coin",
			TypeArgs: []TypeTag{{
				Any: true,
			}},
		},
	}, s)

	s, err = TypeTagFromString("0x2::coin::Coin<any,0x03::mm::zz,any>")
	assert.NoError(t, err)
	assert.Equal(t, &TypeTag{
		Struct: &StructTag{
			Address: StrToAddressMust("0x2"),
			Module:  "coin",
			Name:    "Coin",
			TypeArgs: []TypeTag{{
				Any: true,
			}, {
				Struct: &StructTag{
					Address: StrToAddressMust("0x03"),
					Module:  "mm",
					Name:    "zz",
				},
			}, {
				Any: true,
			}},
		},
	}, s)

	s, err = TypeTagFromString("0x2::coin::Coin<any,0x03::mm::zz<any,u8>,any>")
	assert.NoError(t, err)
	assert.Equal(t, &TypeTag{
		Struct: &StructTag{
			Address: StrToAddressMust("0x2"),
			Module:  "coin",
			Name:    "Coin",
			TypeArgs: []TypeTag{{
				Any: true,
			}, {
				Struct: &StructTag{
					Address: StrToAddressMust("0x03"),
					Module:  "mm",
					Name:    "zz",
					TypeArgs: []TypeTag{{
						Any: true,
					}, {
						U8: true,
					}},
				},
			}, {
				Any: true,
			}},
		},
	}, s)
}

func TestTypeTagInclude(t *testing.T) {
	types := []string{
		"any",
		"u8",
		"u64",
		"vector<any>",
		"vector<u8>",
		"0x2::coin::Coin",
		"0x2::coin::Coin<any>",
		"0x2::coin::Coin<u8>",
		"0x2::coin::Coin<any, any>",
		"0x2::coin::Coin<u8, any>",
		"0x2::coin::Coin<any, u8>",
		"0x2::coin::Coin<u8, u8>",
		"0x2::coin::Coin<0x3::m::n, any>",
		"0x2::coin::Coin<0x3::m::n, u8>",
		"0x2::coin::Coin<0x3::m::n<any>, any>",
		"0x2::coin::Coin<0x3::m::n<any>, u8>",
		"0x2::coin::Coin<0x3::m::n<u8>, any>",
		"0x2::coin::Coin<0x3::m::n<u8>, u8>",
	}
	includes := [18][18]int{
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // any
		{0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // u8
		{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // u64
		{0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // vector<any>
		{0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // vector<u8>
		{0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // 0x2::coin::Coin
		{0, 0, 0, 0, 0, 0, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 0x2::coin::Coin<any>
		{0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, // 0x2::coin::Coin<u8>
		{0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, // 0x2::coin::Coin<any,any>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0}, // 0x2::coin::Coin<u8,any>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 0, 1, 0, 1, 0, 1}, // 0x2::coin::Coin<any,u8>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}, // 0x2::coin::Coin<u8,u8>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1}, // 0x2::coin::Coin<0x3::m::n,any>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1}, // 0x2::coin::Coin<0x3::m::n,u8>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1}, // 0x2::coin::Coin<0x3::m::n<any>,any>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1}, // 0x2::coin::Coin<0x3::m::n<any>,u8>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 1}, // 0x2::coin::Coin<0x3::m::n<u8>,any>
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, // 0x2::coin::Coin<0x3::m::n<u8>,u8>
	}
	for i := range types {
		for j := range types {
			assert.Equalf(t, includes[i][j] == 1, TypeTagFromStringMust(types[i]).Include(TypeTagFromStringMust(types[j])),
				"test case #%d-%d: %s include %s", i, j, types[i], types[j])
		}
	}
}

func TestPureValueWithNoType(t *testing.T) {
	rawJSON := `{
		"type": "pure",
		"valueType": null,
		"value": [
			0,4,6
		]
	}`
	var v PureValue
	err := json.Unmarshal([]byte(rawJSON), &v)
	assert.NoError(t, err)
	assert.Equal(t, TypeUnresolved, v.ValueType())

	rawBytes, err := v.RawBytes()
	assert.NoError(t, err)
	assert.Equal(t, []byte{0, 4, 6}, rawBytes)

	j, err := v.MarshalJSON()
	assert.NoError(t, err)

	ja := jsonassert.New(t)
	ja.Assertf(string(j), rawJSON)
}

func Test_xxx(t *testing.T) {
	// sui-mainnet transaction with seq number 10076320 has value below:
	//
	//value := json.RawMessage{
	//	0x22, 0x49, 0x20, 0x68, 0x61, 0x64, 0x20, 0x61, 0x20, 0x64,
	//	0x72, 0x65, 0x61, 0x6d, 0x20, 0x61, 0x62, 0x6f, 0x75, 0x74,
	//	0x20, 0x79, 0x6f, 0x75, 0x2e, 0x20, 0x57, 0x65, 0x20, 0x69,
	//	0x6e, 0x73, 0x74, 0x61, 0x6c, 0x6c, 0x65, 0x64, 0x20, 0x44,
	//	0x72, 0x2e, 0x20, 0x52, 0x6f, 0x62, 0x65, 0x72, 0x74, 0x20,
	//	0x4a, 0x61, 0x72, 0x76, 0x69, 0x6b, 0xe2, 0x80, 0x99, 0x73,
	//	0x20, 0x61, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x69, 0x61,
	//	0x6c, 0x20, 0x68, 0x65, 0x61, 0x72, 0x74, 0x20, 0x69, 0x6e,
	//	0x20, 0x61, 0x20, 0x6d, 0x61, 0x6e, 0x6e, 0x65, 0x71, 0x75,
	//	0x69, 0x6e, 0x20, 0x61, 0x6e, 0x64, 0x20, 0x62, 0x72, 0x6f,
	//	0x75, 0x67, 0x68, 0x74, 0x20, 0x69, 0x74, 0x20, 0x74, 0x6f,
	//	0x20, 0x6c, 0x69, 0x66, 0x65, 0x2c, 0x20, 0x6f, 0x6e, 0x6c,
	//	0x79, 0x20, 0x74, 0x6f, 0x20, 0x6c, 0x61, 0x74, 0x65, 0x72,
	//	0x20, 0x6b, 0x69, 0x6c, 0x6c, 0x20, 0x69, 0x74, 0x20, 0x62,
	//	0x65, 0x63, 0x61, 0x75, 0x73, 0x65, 0x20, 0x61, 0x20, 0x63,
	//	0x72, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x20, 0x74, 0x68,
	//	0x61, 0x74, 0xe2, 0x80, 0x99, 0x73, 0x20, 0x61, 0x6c, 0x6c,
	//	0x20, 0x66, 0x61, 0x6b, 0x65, 0x20, 0x68, 0x65, 0x61, 0x72,
	//	0x74, 0x20, 0x61, 0x6e, 0x64, 0x20, 0x6e, 0x6f, 0x20, 0x62,
	//	0x72, 0x61, 0x69, 0x6e, 0x20, 0x69, 0x73, 0x20, 0x77, 0x68,
	//	0x61, 0x74, 0xe2, 0x80, 0x99, 0x73, 0x20, 0x63, 0x6f, 0x6d,
	//	0x6d, 0x6f, 0x6e, 0x6c, 0x79, 0x20, 0x63, 0x61, 0x6c, 0x6c,
	//	0x65, 0x64, 0x20, 0x61, 0x20, 0xe2, 0x80, 0x9c, 0x70, 0x6f,
	//	0x6c, 0x69, 0x74, 0x69, 0x63, 0x69, 0x61, 0x6e, 0x2c, 0xe2,
	//	0x80, 0x9d, 0x20, 0x61, 0x6e, 0x64, 0x20, 0x6d, 0x75, 0x73,
	//	0x74, 0x20, 0x62, 0x65, 0x20, 0x64, 0x65, 0x73, 0x74, 0x72,
	//	0x6f, 0x79, 0x65, 0x64, 0x2e, 0xe2, 0x80, 0xa8, 0x22}
	//fmt.Println("LEN:", len(value))
	//fmt.Println("STR:", string(value))
	//v := PureValue{
	//	json: &pureValueJSON{
	//		Type:      "pure",
	//		ValueType: nil,
	//		Value:     value,
	//	},
	//}

	v := struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}{
		Type:  "pure",
		Value: json.RawMessage{0x22, 0x2e, 0xe2, 0x80, 0xa8, 0x22},
	}
	b, err := json.Marshal(&v)
	assert.Equal(t, `{"type":"pure","value":".\u2028"}`, string(b))
	assert.NoError(t, err)
}
