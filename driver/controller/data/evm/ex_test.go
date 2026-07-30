package evm

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/chain/evm"
)

func exHash(n uint64) common.Hash { return common.BigToHash(big.NewInt(int64(n))) }

func exLinks(from uint64, count int) []evm.BlockLink {
	links := make([]evm.BlockLink, count)
	for i := range links {
		n := from + uint64(i)
		links[i] = evm.BlockLink{Number: n, Hash: exHash(n), ParentHash: exHash(n - 1), Timestamp: n * 12}
	}
	return links
}

func exLog(n uint64) types.Log {
	return types.Log{
		Address:     common.BigToAddress(big.NewInt(1)),
		Topics:      []common.Hash{common.BigToHash(big.NewInt(11))},
		Data:        []byte{0x01},
		BlockNumber: n,
		TxHash:      common.BigToHash(big.NewInt(int64(n))),
	}
}

func TestBlockMainDataIsEmpty(t *testing.T) {
	assert.True(t, BlockMainData{}.IsEmpty())
	assert.False(t, BlockMainData{Exact: true}.IsEmpty())
	// a block that carries only its chain identity is NOT empty: it must flow through the
	// pipeline so the header list stays contiguous and the checkpoint records the identity
	assert.False(t, BlockMainData{Header: &BlockHeader{BlockNumber: 1}}.IsEmpty())
}

func TestBuildLogEntriesFromEx(t *testing.T) {
	part, err := evm.NewBlockHashLinkPart(exLinks(100, 4)) // covers [100,103]
	assert.NoError(t, err)
	resp := GetLogsExResponse{
		BlockHashLinkPart: part,
		Logs:              []types.Log{exLog(99), exLog(102)}, // 99 uncovered, 102 covered
	}
	entries, err := buildLogEntriesFromEx(nil, nil, LogRequirement{}, resp)
	assert.NoError(t, err)
	assert.Len(t, entries, 5)

	// covered blocks without logs are header-only entries with the linked identity
	for _, bn := range []uint64{100, 101, 103} {
		entry := entries[bn]
		assert.NotNil(t, entry.Header, "block %d", bn)
		assert.Empty(t, entry.Logs)
		assert.False(t, entry.IsEmpty())
		assert.Equal(t, exHash(bn).Hex(), entry.Header.BlockHash)
		assert.Equal(t, exHash(bn-1).Hex(), entry.Header.ParentBlockHash)
		assert.Equal(t, int64(bn*12), entry.Header.BlockTime.Unix())
	}
	// the covered log got its stripped blockHash and blockTimestamp backfilled from the link
	assert.Equal(t, exHash(102), entries[102].Logs[0].BlockHash)
	assert.Equal(t, uint64(102*12), entries[102].Logs[0].BlockTimestamp)
	assert.NotNil(t, entries[102].Header)
	// the uncovered log keeps whatever the upstream sent and carries no identity
	assert.Nil(t, entries[99].Header)
	assert.Equal(t, common.Hash{}, entries[99].Logs[0].BlockHash)
}

func TestBuildLogEntriesFromExMalformedLink(t *testing.T) {
	// a malformed link shape (hashes != timestamps + 1) must be rejected; the hash CHAIN itself
	// is consistent by construction of the wire format, wrong-fork answers are caught by the
	// anchor comparisons downstream instead
	from := hexutil.Uint64(100)
	malformed := evm.BlockHashLinkPart{
		LinkFromBlock:  &from,
		BlockHashLink:  []common.Hash{exHash(99), exHash(100)},
		BlockTimestamp: []hexutil.Uint64{1200, 1212},
	}
	_, err := buildLogEntriesFromEx(nil, nil, LogRequirement{}, GetLogsExResponse{BlockHashLinkPart: malformed})
	assert.Error(t, err)
}

func TestBuildTraceEntriesFromEx(t *testing.T) {
	part, err := evm.NewBlockHashLinkPart(exLinks(200, 3)) // covers [200,202]
	assert.NoError(t, err)
	var covered, uncovered Trace
	assert.NoError(t, json.Unmarshal([]byte(traceRawJSON(201, "")), &covered))
	assert.NoError(t, json.Unmarshal([]byte(traceRawJSON(150, exHash(150).Hex())), &uncovered))
	resp := GetTracesExResponse{
		BlockHashLinkPart: part,
		Traces:            []Trace{covered, uncovered},
	}
	entries, err := buildTraceEntriesFromEx(TraceRequirement{}, resp)
	assert.NoError(t, err)
	assert.Len(t, entries, 4)

	// the covered trace got blockHash backfilled into BOTH the struct and the Raw JSON —
	// Raw is what gets handed to the processor
	got := entries[201].Traces[0]
	assert.Equal(t, exHash(201).Hex(), got.BlockHash)
	var raw map[string]json.RawMessage
	assert.NoError(t, json.Unmarshal(got.Raw, &raw))
	var rawHash string
	assert.NoError(t, json.Unmarshal(raw["blockHash"], &rawHash))
	assert.Equal(t, exHash(201).Hex(), rawHash)
	// action payload untouched by the patch
	assert.Contains(t, string(raw["action"]), "0xdeadbeef")

	// header-only covered blocks exist, uncovered trace keeps its own hash without identity
	assert.NotNil(t, entries[200].Header)
	assert.NotNil(t, entries[202].Header)
	assert.Nil(t, entries[150].Header)
	assert.Equal(t, exHash(150).Hex(), entries[150].Traces[0].BlockHash)
}

func TestBuildTraceEntriesFromExAppliesFilter(t *testing.T) {
	part, err := evm.NewBlockHashLinkPart(exLinks(200, 1))
	assert.NoError(t, err)
	var trace Trace
	assert.NoError(t, json.Unmarshal([]byte(traceRawJSON(200, "")), &trace))
	resp := GetTracesExResponse{BlockHashLinkPart: part, Traces: []Trace{trace}}
	entries, err := buildTraceEntriesFromEx(TraceRequirement{
		TraceFilter: TraceFilter{Signature: []string{"0x99999999"}}, // does not match 0xdeadbeef
	}, resp)
	assert.NoError(t, err)
	// the trace is filtered out but the block keeps its identity entry
	assert.Empty(t, entries[200].Traces)
	assert.NotNil(t, entries[200].Header)
}

func traceRawJSON(bn uint64, blockHash string) string {
	hashField := ""
	if blockHash != "" {
		hashField = fmt.Sprintf(`"blockHash": %q,`, blockHash)
	}
	return fmt.Sprintf(`{
		"action": {"input": "0xdeadbeef00", "to": "0x00000000000000000000000000000000000000aa"},
		%s
		"blockNumber": %d,
		"transactionPosition": 3,
		"transactionHash": "0x00000000000000000000000000000000000000000000000000000000000000bb"
	}`, hashField, bn)
}
