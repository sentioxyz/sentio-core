package types

import (
	"sentioxyz/sentio-core/chain/sui/types/serde"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeMakeMoveVec(t *testing.T) {
	raw := []byte{0x05, // MakeMoveVec variant
		0x00, // optional field TypeTag, present=false
		0x04, // Argument slice of size 4
		0x01, 0x01, 0x00,
		0x01, 0x02, 0x00,
		0x01, 0x03, 0x00,
		0x01, 0x04, 0x00}

	command := &Command{}
	if err := serde.Unmarshal(raw, command); err != nil {
		t.Fatal(err)
	}

	var v1, v2, v3, v4 uint16
	v1, v2, v3, v4 = 1, 2, 3, 4
	assert.Equal(t, &Command{
		MakeMoveVec: &MakeMoveVec{
			Args: []Argument{
				{Input: &v1},
				{Input: &v2},
				{Input: &v3},
				{Input: &v4},
			},
		},
	}, command)
}
