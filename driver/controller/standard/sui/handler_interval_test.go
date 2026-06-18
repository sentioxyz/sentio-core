package sui

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_loadDump(t *testing.T) {
	m0 := ObjectDictSetManager{}
	assert.NoError(t, m0.load(""))

	m0.Put("x1", ObjectDict{
		BlockNumber: 100,
		ObjectLatestVersion: map[string]uint64{
			"0x1": 100,
		},
	})

	m0.Put("x2", ObjectDict{
		BlockNumber: 101,
		ObjectLatestVersion: map[string]uint64{
			"0x21": 999,
			"0x22": 0,
		},
	})

	x3 := ObjectDict{
		BlockNumber:         100000000,
		ObjectLatestVersion: map[string]uint64{},
	}
	for i := 0; i < 100000; i++ {
		id := fmt.Sprintf("0x%016x%016x%016x%016x", rand.Uint64(), rand.Uint64(), rand.Uint64(), rand.Uint64())
		ver := rand.Uint64()
		x3.ObjectLatestVersion[id] = ver
	}
	m0.Put("x3", x3)

	d0 := m0.GetData()
	t.Logf("data0: %d", len(d0))

	var m1 ObjectDictSetManager
	assert.NoError(t, m1.load(d0))

	assert.Equal(t, m0.data, m1.data)
	assert.Equal(t, m0.cachedBlockNumber, m1.cachedBlockNumber)
	assert.Equal(t, m0.cachedData, m1.cachedData)
}
