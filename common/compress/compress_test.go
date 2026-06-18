package compress

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_loadAndDump(t *testing.T) {
	const count = 100000
	origin := make([]string, count)
	for i := 0; i < count; i++ {
		origin[i] = fmt.Sprintf("%08d", i)
	}

	d0, _ := json.Marshal(origin)
	t.Logf("origin len: %d", len(d0))
	d, err := Dump(origin)
	assert.NoError(t, err)

	t.Logf("dump len: %d", len(d))
	t.Logf("dump prefix: %s", string(d[:100]))
	t.Logf("dump suffix: %s", string(d[len(d)-100:]))

	var result []string
	assert.NoError(t, Load(d, &result))
	assert.Equal(t, origin, result)

	assert.NoError(t, Load(d0, &result))
	assert.Equal(t, origin, result)
}
