package fuel

import (
	"github.com/pkg/errors"
	fuelGo "github.com/sentioxyz/fuel-go"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_IsQueryErrors(t *testing.T) {
	assert.True(t, IsQueryErrors(fuelGo.QueryErrors{
		{
			Message: "error",
		},
	}))
	assert.True(t, IsQueryErrors(fuelGo.QueryErrors{}))
	assert.True(t, IsQueryErrors(fuelGo.QueryErrors(nil)))
	assert.False(t, IsQueryErrors(nil))
	assert.False(t, IsQueryErrors(errors.New("normal")))
}
