package evm

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func testLinks(from uint64, count int) []BlockLink {
	links := make([]BlockLink, count)
	for i := range links {
		n := from + uint64(i)
		links[i] = BlockLink{
			Number:     n,
			Hash:       common.BigToHash(big.NewInt(int64(n))),
			ParentHash: common.BigToHash(big.NewInt(int64(n - 1))),
			Timestamp:  n * 12,
		}
	}
	return links
}

func TestNewBlockHashLinkPart(t *testing.T) {
	empty, err := NewBlockHashLinkPart(nil)
	assert.NoError(t, err)
	assert.Nil(t, empty.LinkFromBlock)
	assert.False(t, empty.Covers(0))

	links := testLinks(100, 3)
	part, err := NewBlockHashLinkPart(links)
	assert.NoError(t, err)
	assert.Equal(t, uint64(100), uint64(*part.LinkFromBlock))
	assert.Len(t, part.BlockHashLink, 4)
	assert.Equal(t, links[0].ParentHash, part.BlockHashLink[0])
	assert.Equal(t, links[2].Hash, part.BlockHashLink[3])
	assert.Len(t, part.BlockTimestamp, 3)
	assert.False(t, part.Covers(99))
	assert.True(t, part.Covers(100))
	assert.True(t, part.Covers(102))
	assert.False(t, part.Covers(103))

	rebuilt, err := part.Links()
	assert.NoError(t, err)
	assert.Equal(t, links, rebuilt)

	// non-contiguous numbers
	broken := testLinks(100, 3)
	broken[2].Number = 103
	_, err = NewBlockHashLinkPart(broken)
	assert.ErrorContains(t, err, "not contiguous")

	// broken hash link (mid-reorg read)
	broken = testLinks(100, 3)
	broken[2].ParentHash = common.BigToHash(big.NewInt(999))
	_, err = NewBlockHashLinkPart(broken)
	assert.ErrorContains(t, err, "link mismatch")
}

func TestEthGetLogsExResponseMarshalJSON(t *testing.T) {
	part, err := NewBlockHashLinkPart(testLinks(100, 2))
	assert.NoError(t, err)
	resp := EthGetLogsExResponse{
		BlockHashLinkPart: part,
		Logs: []types.Log{
			{ // covered: blockHash must be stripped on the wire
				Address:     common.BigToAddress(big.NewInt(1)),
				Topics:      []common.Hash{common.BigToHash(big.NewInt(11))},
				Data:        []byte{0x01},
				BlockNumber: 100,
				BlockHash:   common.BigToHash(big.NewInt(100)),
				TxHash:      common.BigToHash(big.NewInt(0xaa)),
			},
			{ // not covered: blockHash must be kept
				Address:     common.BigToAddress(big.NewInt(2)),
				Topics:      []common.Hash{common.BigToHash(big.NewInt(22))},
				Data:        []byte{0x02},
				BlockNumber: 99,
				BlockHash:   common.BigToHash(big.NewInt(99)),
				TxHash:      common.BigToHash(big.NewInt(0xbb)),
			},
		},
	}
	raw, err := json.Marshal(resp)
	assert.NoError(t, err)

	var wire struct {
		LinkFromBlock  string                       `json:"linkFromBlock"`
		BlockHashLink  []common.Hash                `json:"blockHashLink"`
		BlockTimestamp []string                     `json:"blockTimestamp"`
		Logs           []map[string]json.RawMessage `json:"logs"`
	}
	assert.NoError(t, json.Unmarshal(raw, &wire))
	assert.Equal(t, "0x64", wire.LinkFromBlock)
	assert.Len(t, wire.BlockHashLink, 3)
	assert.Len(t, wire.BlockTimestamp, 2)
	assert.NotContains(t, wire.Logs[0], "blockHash")
	assert.NotContains(t, wire.Logs[0], "blockTimestamp")
	assert.Contains(t, wire.Logs[1], "blockHash")

	// the response must round-trip: default unmarshal restores everything except the
	// stripped fields, which the client rebuilds from the link part
	var back EthGetLogsExResponse
	assert.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, resp.BlockHashLinkPart, back.BlockHashLinkPart)
	assert.Len(t, back.Logs, 2)
	assert.Equal(t, common.Hash{}, back.Logs[0].BlockHash)
	assert.Equal(t, resp.Logs[1].BlockHash, back.Logs[1].BlockHash)
}

func TestTraceFilterExResponseMarshalJSON(t *testing.T) {
	part, err := NewBlockHashLinkPart(testLinks(200, 1))
	assert.NoError(t, err)
	resp := TraceFilterExResponse{
		BlockHashLinkPart: part,
		Traces: []ParityTrace{
			{BlockNumber: 200, BlockHash: common.BigToHash(big.NewInt(200)), Type: "call"},
			{BlockNumber: 150, BlockHash: common.BigToHash(big.NewInt(150)), Type: "call"},
		},
	}
	raw, err := json.Marshal(resp)
	assert.NoError(t, err)

	var wire struct {
		Traces []map[string]json.RawMessage `json:"traces"`
	}
	assert.NoError(t, json.Unmarshal(raw, &wire))
	assert.NotContains(t, wire.Traces[0], "blockHash")
	assert.Contains(t, wire.Traces[1], "blockHash")

	var back TraceFilterExResponse
	assert.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, common.Hash{}, back.Traces[0].BlockHash)
	assert.Equal(t, resp.Traces[1].BlockHash, back.Traces[1].BlockHash)
}
