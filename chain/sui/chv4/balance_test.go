package chv4

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"math/big"
	"sentioxyz/sentio-core/chain/sui/types"
	"testing"
)

func Test_balanceItemSerialization(t *testing.T) {
	// #1
	item := balanceItem{
		Checkpoint: 10,
		TransactionIndex: TransactionIndex{
			TxIndex:  0,
			TxDigest: (&types.Digest{0, 1, 2}).String(),
		},
		Balance: big.NewInt(0),
	}
	b := item.toBytes()
	assert.Equal(t, []byte{
		1, 10, // checkpoint
		0,                                                                                              // txIndex
		0, 1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // txDigest
		0, // balance
	}, b)
	bi, err := newBalanceItemFromBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, item, bi)

	// #2
	item = balanceItem{
		Checkpoint: 4294967296,
		TransactionIndex: TransactionIndex{
			TxIndex:  1,
			TxDigest: (&types.Digest{0, 1, 2}).String(),
		},
		Balance: big.NewInt(-1),
	}
	b = item.toBytes()
	assert.Equal(t, []byte{
		5, 0, 0, 0, 0, 1, // checkpoint
		1, 1, // txIndex
		0, 1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // txDigest
		1, 1, // balance
	}, b)
	bi, err = newBalanceItemFromBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, item, bi)

	// #3
	item = balanceItem{
		Checkpoint: 4294967296,
		TransactionIndex: TransactionIndex{
			TxIndex:  32767,
			TxDigest: (&types.Digest{0, 1, 2}).String(),
		},
	}
	item.Balance, _ = new(big.Int).SetString("340282366920938463463374607431768211455", 10)
	b = item.toBytes()
	assert.Equal(t, []byte{
		5, 0, 0, 0, 0, 1, // checkpoint
		2, 0xff, 0x7f, // txIndex
		0, 1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // txDigest
		0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // balance
	}, b)
	bi, err = newBalanceItemFromBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, item, bi)

	// #4
	item = balanceItem{
		Checkpoint: 4294967296,
		TransactionIndex: TransactionIndex{
			TxIndex:  256,
			TxDigest: (&types.Digest{}).String(),
		},
	}
	item.Balance, _ = new(big.Int).SetString("-340282366920938463463374607431768211455", 10)
	b = item.toBytes()
	assert.Equal(t, []byte{
		5, 0, 0, 0, 0, 1, // checkpoint
		2, 0, 1, // txIndex
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // txDigest
		1, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, // balance
	}, b)
	bi, err = newBalanceItemFromBytes(b)
	assert.NoError(t, err)
	assert.Equal(t, item, bi)
}

func Test_itemKey(t *testing.T) {
	// #1
	addr := (types.Address{0, 1, 2}).String()
	coinType := "0x2::sui::SUI"
	key := buildItemKey(addr, coinType)
	assert.Equal(t, []byte{
		0, 1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // addr
		'0', 'x', '2', ':', ':', 's', 'u', 'i', ':', ':', 'S', 'U', 'I', // coinType
	}, key)
	a, c := cutItemKey(key)
	assert.Equal(t, addr, a)
	assert.Equal(t, coinType, c)
	fmt.Printf("!!! %s\n", keyText(key))

	// #2
	addr = (types.Address{}).String()
	coinType = "x"
	key = buildItemKey(addr, coinType)
	assert.Equal(t, []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // addr
		'x', // coinType
	}, key)
	a, c = cutItemKey(key)
	assert.Equal(t, addr, a)
	assert.Equal(t, coinType, c)
	fmt.Printf("!!! %s\n", keyText(key))
}

func Test_doByPage(t *testing.T) {
	ok := make(map[int]bool)
	err := doByPage(context.Background(), 100, 16, 5, "test",
		func(ctx context.Context, start, end int) (string, error) {
			if end-start > 10 {
				return "", errors.Errorf("page size too large")
			}
			//if end%10 == 0 {
			//	return "", errors.Errorf("end == %d", end)
			//}
			for i := start; i < end; i++ {
				ok[i] = true
			}
			return "good", nil
		})
	assert.NoError(t, err)
	assert.Equal(t, 100, len(ok))
}
