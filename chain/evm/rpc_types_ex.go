package evm

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
)

// BlockLink is the chain identity of one block: just enough for a caller to verify which fork a
// range query's results came from.
type BlockLink struct {
	Number     uint64
	Hash       common.Hash
	ParentHash common.Hash
	Timestamp  uint64
}

// SlotBlockLink extracts the BlockLink of a cached slot.
func SlotBlockLink(st *Slot) BlockLink {
	return BlockLink{
		Number:     st.Header.Number.Uint64(),
		Hash:       st.Header.Hash,
		ParentHash: st.Header.ParentHash,
		Timestamp:  st.Header.Time,
	}
}

// BlockHashLinkPart is the chain-identity section shared by EthGetLogsExResponse and
// TraceFilterExResponse. It describes a CONTIGUOUS TAIL sub-range of the queried block range
// (from LinkFromBlock to the end): BlockHashLink[0] is the parent hash of LinkFromBlock and
// BlockHashLink[i+1] / BlockTimestamp[i] are the hash / timestamp of block LinkFromBlock+i.
// The part may cover less than the queried range (a slot-cache-only super node proxies the
// lower sub-range upstream and cannot vouch for it); an absent LinkFromBlock means no block in
// the response carries identity information.
type BlockHashLinkPart struct {
	LinkFromBlock  *hexutil.Uint64  `json:"linkFromBlock,omitempty"`
	BlockHashLink  []common.Hash    `json:"blockHashLink,omitempty"`
	BlockTimestamp []hexutil.Uint64 `json:"blockTimestamp,omitempty"`
}

// NewBlockHashLinkPart assembles the part from ascending per-block links, verifying that block
// numbers are contiguous and that each block's parent hash is the previous block's hash — a
// broken link means the source (store + cache splice) was read mid-reorg and the caller should
// fail the query instead of serving a frankenstein chain.
func NewBlockHashLinkPart(links []BlockLink) (BlockHashLinkPart, error) {
	var part BlockHashLinkPart
	if len(links) == 0 {
		return part, nil
	}
	part.LinkFromBlock = (*hexutil.Uint64)(&links[0].Number)
	part.BlockHashLink = make([]common.Hash, 0, len(links)+1)
	part.BlockHashLink = append(part.BlockHashLink, links[0].ParentHash)
	part.BlockTimestamp = make([]hexutil.Uint64, 0, len(links))
	for i, link := range links {
		if i > 0 {
			if link.Number != links[i-1].Number+1 {
				return BlockHashLinkPart{}, errors.Errorf(
					"block links are not contiguous: %d follows %d", link.Number, links[i-1].Number)
			}
			if link.ParentHash != links[i-1].Hash {
				return BlockHashLinkPart{}, errors.Errorf(
					"link mismatch between [%d:->%s] and [%d:%s->]: the blocks were read mid-reorg, please retry",
					links[i-1].Number, links[i-1].Hash, link.Number, link.ParentHash)
			}
		}
		part.BlockHashLink = append(part.BlockHashLink, link.Hash)
		part.BlockTimestamp = append(part.BlockTimestamp, hexutil.Uint64(link.Timestamp))
	}
	return part, nil
}

// Covers reports whether the part carries identity information for the given block.
func (p BlockHashLinkPart) Covers(blockNumber uint64) bool {
	return p.LinkFromBlock != nil &&
		blockNumber >= uint64(*p.LinkFromBlock) &&
		blockNumber < uint64(*p.LinkFromBlock)+uint64(len(p.BlockTimestamp))
}

// Links rebuilds the per-block links described by the part, validating the internal shape
// (len(BlockHashLink) == len(BlockTimestamp)+1). It is the inverse of NewBlockHashLinkPart and
// is what a client should use to consume the part.
func (p BlockHashLinkPart) Links() ([]BlockLink, error) {
	if p.LinkFromBlock == nil {
		if len(p.BlockHashLink) > 0 || len(p.BlockTimestamp) > 0 {
			return nil, errors.New("blockHashLink present without linkFromBlock")
		}
		return nil, nil
	}
	if len(p.BlockHashLink) != len(p.BlockTimestamp)+1 {
		return nil, errors.Errorf("blockHashLink has %d hashes but %d timestamps (want hashes = timestamps + 1)",
			len(p.BlockHashLink), len(p.BlockTimestamp))
	}
	links := make([]BlockLink, len(p.BlockTimestamp))
	for i := range links {
		links[i] = BlockLink{
			Number:     uint64(*p.LinkFromBlock) + uint64(i),
			Hash:       p.BlockHashLink[i+1],
			ParentHash: p.BlockHashLink[i],
			Timestamp:  uint64(p.BlockTimestamp[i]),
		}
	}
	return links, nil
}

// EthGetLogsExResponse is the result of eth_getLogsEx: the same logs eth_getLogs would return
// plus the chain identity of the covered blocks, so the caller can verify which fork the result
// (INCLUDING an empty result) came from. Covered logs omit blockHash/blockTimestamp on the wire —
// that information is already in the link part; logs of uncovered blocks keep their blockHash.
type EthGetLogsExResponse struct {
	BlockHashLinkPart
	Logs []types.Log `json:"logs"`
}

// TraceFilterExResponse is the result of trace_filterEx, the trace_filter counterpart of
// EthGetLogsExResponse. Covered traces omit blockHash on the wire.
type TraceFilterExResponse struct {
	BlockHashLinkPart
	Traces []ParityTrace `json:"traces"`
}

func (r EthGetLogsExResponse) MarshalJSON() ([]byte, error) {
	logs := make([]json.RawMessage, len(r.Logs))
	for i := range r.Logs {
		raw, err := json.Marshal(&r.Logs[i])
		if err != nil {
			return nil, err
		}
		if r.Covers(r.Logs[i].BlockNumber) {
			if raw, err = stripJSONFields(raw, "blockHash", "blockTimestamp"); err != nil {
				return nil, err
			}
		}
		logs[i] = raw
	}
	return json.Marshal(struct {
		BlockHashLinkPart
		Logs []json.RawMessage `json:"logs"`
	}{r.BlockHashLinkPart, logs})
}

func (r TraceFilterExResponse) MarshalJSON() ([]byte, error) {
	traces := make([]json.RawMessage, len(r.Traces))
	for i := range r.Traces {
		raw, err := json.Marshal(&r.Traces[i])
		if err != nil {
			return nil, err
		}
		if r.Covers(r.Traces[i].BlockNumber) {
			if raw, err = stripJSONFields(raw, "blockHash"); err != nil {
				return nil, err
			}
		}
		traces[i] = raw
	}
	return json.Marshal(struct {
		BlockHashLinkPart
		Traces []json.RawMessage `json:"traces"`
	}{r.BlockHashLinkPart, traces})
}

func stripJSONFields(raw json.RawMessage, fields ...string) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	for _, f := range fields {
		delete(m, f)
	}
	return json.Marshal(m)
}
