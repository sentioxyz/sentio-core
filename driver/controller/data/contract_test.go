package data

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func Test_BinarySearchContractStart(t *testing.T) {
	const startBlock = 500
	checker := func(_ context.Context, bn uint64) (bool, error) {
		if bn < 100 || bn > 1000 {
			return false, errors.Errorf("out of range")
		}
		return bn >= startBlock, nil
	}

	for s := uint64(100); s <= 1000; s++ {
		for e := s; e <= 1000; e++ {
			n, has, err := BinarySearchContractStart(context.Background(), s, e, checker)
			assert.NoError(t, err)
			assert.Equalf(t, startBlock <= e, has, "s=%d,e=%d", s, e)
			if has {
				assert.Equalf(t, max(startBlock, s), n, "s=%d,e=%d", s, e)
			}
		}
	}
}
