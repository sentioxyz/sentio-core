package evm

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
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
	entries, err := buildLogEntriesFromEx(nil, nil, LogRequirement{}, resp, 103)
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
	_, err := buildLogEntriesFromEx(nil, nil, LogRequirement{}, GetLogsExResponse{BlockHashLinkPart: malformed}, 102)
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
	entries, err := buildTraceEntriesFromEx(TraceRequirement{}, resp, 202)
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
	}, resp, 200)
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

func TestCheckBlockTxsMatch(t *testing.T) {
	expected := []string{"0xa", "0xb", "0xc"}
	assert.NoError(t, checkBlockTxsMatch(1, expected, []string{"0xc", "0xa", "0xb"}))
	assert.NoError(t, checkBlockTxsMatch(1, nil, nil))
	// count mismatch: the answer misses or adds transactions vs the header's list
	assert.ErrorContains(t, checkBlockTxsMatch(1, expected, []string{"0xa", "0xb"}), "different fork")
	// same count but a foreign transaction: answered from a sibling block
	assert.ErrorContains(t, checkBlockTxsMatch(1, expected, []string{"0xa", "0xb", "0xd"}), "not listed")
}

func TestBuildLogEntriesFromExBehindHead(t *testing.T) {
	// links stop before the requested end: the endpoint's head is behind (e.g. a lagging
	// replica), treating the missing tail as empty blocks would be silent loss
	part, err := evm.NewBlockHashLinkPart(exLinks(100, 3)) // covers [100,102]
	assert.NoError(t, err)
	_, err = buildLogEntriesFromEx(nil, nil, LogRequirement{}, GetLogsExResponse{BlockHashLinkPart: part}, 105)
	assert.ErrorContains(t, err, "head is behind")

	// no link at all is the deep-history range of a slot-cache-only super node: accepted
	entries, err := buildLogEntriesFromEx(nil, nil, LogRequirement{}, GetLogsExResponse{}, 105)
	assert.NoError(t, err)
	assert.Empty(t, entries)
}

// fakeTipClient stubs just the calls fetchLogsAtTip needs; everything else panics via the
// embedded nil interface.
type fakeTipClient struct {
	Client
	headers       map[uint64]BlockHeader
	byHashErr     error
	byHashLogs    map[string][]types.Log
	byNumberLogs  map[uint64][]types.Log
	byNumberCalls int
}

func (c *fakeTipClient) GetHeader(ctx context.Context, blockNumber uint64) (BlockHeader, error) {
	return c.headers[blockNumber], nil
}

func (c *fakeTipClient) GetLogsByBlockHash(
	ctx context.Context, priority uint64, blockHash string, address []string, topics [][]string,
) ([]types.Log, error) {
	if c.byHashErr != nil {
		return nil, c.byHashErr
	}
	return c.byHashLogs[blockHash], nil
}

func (c *fakeTipClient) GetLogs(
	ctx context.Context, fromBlock, toBlock uint64, address []string, topics [][]string,
) ([]types.Log, error) {
	c.byNumberCalls++
	return c.byNumberLogs[fromBlock], nil
}

func tipHeader(n uint64) BlockHeader {
	return BlockHeader{
		BlockNumber:     n,
		BlockHash:       exHash(n).Hex(),
		ParentBlockHash: exHash(n - 1).Hex(),
	}
}

func TestFetchLogsAtTipByHashFallback(t *testing.T) {
	cli := &fakeTipClient{
		headers:      map[uint64]BlockHeader{100: tipHeader(100), 101: tipHeader(101)},
		byHashLogs:   map[string][]types.Log{exHash(100).Hex(): {exLog(100)}},
		byNumberLogs: map[uint64][]types.Log{100: {exLog(100)}},
	}

	// by-hash works: no by-number call, entries carry the header
	entries, err := fetchLogsAtTip(context.Background(), cli, LogRequirement{}, 100, 101, nil, nil)
	assert.NoError(t, err)
	assert.Zero(t, cli.byNumberCalls)
	assert.Len(t, entries[100].Logs, 1)
	assert.Empty(t, entries[101].Logs)
	assert.Equal(t, exHash(101).Hex(), entries[101].Header.BlockHash)

	// the endpoint rejects the blockHash filter (capability signal): by-number fallback kicks in
	cli.byHashErr = errors.Wrapf(errMethodNotSupported, "invalid params")
	cli.byNumberCalls = 0
	entries, err = fetchLogsAtTip(context.Background(), cli, LogRequirement{}, 100, 100, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, cli.byNumberCalls)
	assert.Len(t, entries[100].Logs, 1)
	assert.NotNil(t, entries[100].Header)

	// any other error (e.g. a super node missing the hash in its cache) propagates for retry
	cli.byHashErr = errors.New("block 0x64 is not in the latest slot cache")
	cli.byNumberCalls = 0
	_, err = fetchLogsAtTip(context.Background(), cli, LogRequirement{}, 100, 100, nil, nil)
	assert.Error(t, err)
	assert.Zero(t, cli.byNumberCalls)
}

func TestCheckTipCovered(t *testing.T) {
	header := tipHeader(100)
	assert.NoError(t, checkTipCovered(map[uint64]BlockMainData{100: {Header: &header}}, 100))
	assert.ErrorContains(t, checkTipCovered(map[uint64]BlockMainData{}, 100), "did not cover")
	assert.ErrorContains(t, checkTipCovered(map[uint64]BlockMainData{100: {Exact: true}}, 100), "did not cover")
}

func TestTracesFromExForBlock(t *testing.T) {
	part, err := evm.NewBlockHashLinkPart(exLinks(200, 1))
	assert.NoError(t, err)
	var trace Trace
	assert.NoError(t, json.Unmarshal([]byte(traceRawJSON(200, "")), &trace))

	// covered and matching: stripped blockHash backfilled into struct and Raw
	traces, err := tracesFromExForBlock(GetTracesExResponse{BlockHashLinkPart: part, Traces: []Trace{trace}},
		200, exHash(200).Hex())
	assert.NoError(t, err)
	assert.Equal(t, exHash(200).Hex(), traces[0].BlockHash)

	// covered with a different identity: the extend data came from a sibling fork
	_, err = tracesFromExForBlock(GetTracesExResponse{BlockHashLinkPart: part},
		200, exHash(999).Hex())
	assert.ErrorContains(t, err, "different fork")

	// empty result is verifiable through the link alone — the reason Ex is preferred here
	traces, err = tracesFromExForBlock(GetTracesExResponse{BlockHashLinkPart: part}, 200, exHash(200).Hex())
	assert.NoError(t, err)
	assert.Empty(t, traces)
}

func TestCheckTracesBlockHash(t *testing.T) {
	var trace Trace
	assert.NoError(t, json.Unmarshal([]byte(traceRawJSON(300, exHash(300).Hex())), &trace))
	assert.NoError(t, checkTracesBlockHash([]Trace{trace}, 300, exHash(300).Hex()))
	assert.ErrorContains(t, checkTracesBlockHash([]Trace{trace}, 300, exHash(999).Hex()), "different fork")
	assert.NoError(t, checkTracesBlockHash([]Trace{trace}, 300, ""))
}
