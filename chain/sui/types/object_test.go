package types

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseShortObjectID(t *testing.T) {
	o := StrToObjectIDMust("0x5")
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000005", o.String())
}

func TestObjectOwnerJsonENDE(t *testing.T) {
	dataStruct1 := struct {
		Owner *ObjectOwner `json:"owner"`
	}{}

	dataStruct2 := struct {
		Owner *ObjectOwner `json:"owner"`
	}{}
	jsonString1 := []byte(`{"owner":"Immutable"}`)

	jsonString2 := []byte(
		`{"owner":{"AddressOwner":"0xc16ecefaeeeba3d9d1ccce47751e266e0e362ee418796d2f494bf843c7855e92"}}`,
	)

	err := json.Unmarshal(jsonString1, &dataStruct1)
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(jsonString2, &dataStruct2)
	if err != nil {
		t.Fatal(err)
	}

	enData1, err := json.Marshal(dataStruct1)
	if err != nil {
		t.Fatal(err)
	}

	enData2, err := json.Marshal(dataStruct2)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(jsonString1, enData1) {
		t.Fatal("encode failed")
	}

	if !bytes.Equal(jsonString2, enData2) {
		t.Fatal("encode failed")
	}
}

func TestNewAddressFromHex(t *testing.T) {
	addr, err := StrToObjectID("0xc16ecefaeeeba3d9d1ccce47751e266e0e362ee418796d2f494bf843c7855e92")
	assert.Nil(t, err)

	t.Log(addr)
}
