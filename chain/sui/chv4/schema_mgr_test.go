package chv4

import (
	"context"
	"fmt"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"os"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
	"testing"
	"time"
)

func Test_convert(t *testing.T) {
	fieldFilter := objectx.HasTag("clickhouse")
	ck := Checkpoint{
		CheckpointIndex: CheckpointIndex{
			Checkpoint:       1,
			CheckpointDigest: "xx",
			Timestamp:        time.Now(),
			Epoch:            2,
		},
		Summary:          "{}",
		Signature:        `{"a":1}`,
		Contents:         `{"b":"abc"}`,
		TransactionCount: 3,
		EventCount:       4,
	}

	values := objectx.CollectFieldValues(ck, fieldFilter)
	for i, v := range values {
		fmt.Printf("!!! %d (%T): %v\n", i, v, v)
	}
}

func Test_encode(t *testing.T) {
	// Enum use string value
	var tx rpcv2.Transaction
	kind := rpcv2.TransactionExpiration_EPOCH
	tx.Expiration = &rpcv2.TransactionExpiration{
		Kind: &kind,
	}
	var version int32 = 10
	var mutable = true
	inputKind := rpcv2.Input_PURE
	tx.Version = &version
	tx.Kind = &rpcv2.TransactionKind{
		Data: &rpcv2.TransactionKind_ProgrammableTransaction{
			ProgrammableTransaction: &rpcv2.ProgrammableTransaction{
				Inputs: []*rpcv2.Input{
					{
						Kind:    &inputKind,
						Mutable: &mutable,
					},
				},
			},
		},
	}
	b := mustBuildJSON("xx", &tx)
	fmt.Printf("!!! %s\n", b)
	assert.Equal(t, `{"version":10,"kind":{"programmableTransaction":{"inputs":[{"kind":"PURE","mutable":true}]}},"expiration":{"kind":"EPOCH"}}`, b)
}

func Test_decode(t *testing.T) {
	bs := []string{
		// number were enclosed in quota is OK
		// Enum use string value is OK
		`
{
  "version": "10",
  "kind": {
    "programmableTransaction": {
      "inputs": [
        {
          "kind": "PURE",
          "mutable": true
        }
      ]
    }
  },
  "expiration": {
    "kind": "EPOCH"
  }
}`,
		// Enum use int value is OK
		`
{
  "version": 10,
  "kind": {
    "programmableTransaction": {
      "inputs": [
        {
          "kind": 1,
          "mutable": true
        }
      ]
    }
  },
  "expiration": {
    "kind": 2
  }
}`,
	}
	for _, b := range bs {
		r := &rpcv2.Transaction{}
		has, err := decodeJSON(b, r)
		assert.True(t, has)
		assert.NoError(t, err)
		assert.Equal(t, rpcv2.TransactionExpiration_EPOCH, *r.Expiration.Kind)
		assert.Equal(t, int32(10), *r.Version)
		assert.Equal(t, rpcv2.Input_PURE, *r.GetKind().GetProgrammableTransaction().GetInputs()[0].Kind)
		assert.Equal(t, true, *r.GetKind().GetProgrammableTransaction().GetInputs()[0].Mutable)
	}

	bs = []string{
		"",
		"null",
		"  ",
		" null  ",
	}
	for _, b := range bs {
		r := &rpcv2.Transaction{}
		has, err := decodeJSON(b, r)
		assert.False(t, has)
		assert.NoError(t, err)
	}

	{
		// Enum use int value, but also be enclosed in quota, is NOT OK
		b := `
{
  "version": 10,
  "kind": {
    "programmableTransaction": {
      "inputs": [
        {
          "kind": "1",
          "mutable": true
        }
      ]
    }
  },
  "expiration": {
    "kind": "2"
  }
}`
		r := &rpcv2.Transaction{}
		has, err := decodeJSON(b, r)
		assert.True(t, has)
		assert.ErrorContains(t, err, "unknown value")
	}
}

func Test_convertReal(t *testing.T) {
	t.Skipf("call endpoint")

	const balanceStorePath = "xxx.test"
	sm, err := NewClickhouseSchemaMgr(chx.NewController(nil, ""), "", 0, balanceStorePath)
	assert.NoError(t, err)

	conf := sui.ClientConfig{
		Endpoint:     "http://127.0.0.1:9000",
		GrpcEndpoint: "http://127.0.0.1:9000",
	}

	cli := sui.NewClient(conf)
	_, err = cli.Init(context.Background())
	assert.NoError(t, err)

	var resp *rpcv2.GetCheckpointResponse
	r := cli.CallContext(context.Background(), &resp, "", "", "grpc_getCheckpoint", uint64(154000214), true)
	assert.NoError(t, r.Err)

	_, txs, _, _, _, err := sm.convert(context.Background(), resp.GetCheckpoint())
	assert.NoError(t, err)
	for i := range txs {
		etx, err := txs[i].ToExecutedTransaction()
		assert.NoError(t, err)
		assert.Equal(t, resp.GetCheckpoint().GetTransactions()[i], etx)
	}

	assert.NoError(t, os.RemoveAll(balanceStorePath))
}
