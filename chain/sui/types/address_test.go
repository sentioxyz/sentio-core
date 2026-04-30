package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShortAddress(t *testing.T) {
	o := StrToAddressMust("0x5")
	assert.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000005", o.String())
	assert.Equal(t, "0x5", o.ShortString())
}
