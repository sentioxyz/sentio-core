package evm

import (
	"encoding/json"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/evm"
)

// GetLogsExResponse / GetTracesExResponse are the driver-side views of the super node's
// eth_getLogsEx / trace_filterEx results. The link part carries the chain identity of a
// contiguous tail sub-range of the query, so results (INCLUDING empty ones) can be verified to
// belong to the expected chain view; entries of covered blocks arrive without blockHash and are
// backfilled from the link part.
type GetLogsExResponse struct {
	evm.BlockHashLinkPart
	Logs []types.Log `json:"logs"`
}

type GetTracesExResponse struct {
	evm.BlockHashLinkPart
	Traces []Trace `json:"traces"`
}

// headerFromLink converts one verified BlockLink into the driver's BlockHeader shape. Raw and
// TxHashes stay empty: link-derived headers identify blocks without data, which never build
// tasks that would need them.
func headerFromLink(link evm.BlockLink) BlockHeader {
	return BlockHeader{
		BlockNumber:     link.Number,
		BlockHash:       link.Hash.Hex(),
		BlockTime:       time.Unix(int64(link.Timestamp), 0),
		ParentBlockHash: link.ParentHash.Hex(),
	}
}

// verifiedLinks decodes the link part of an Ex response, validating its shape. The hash CHAIN
// needs no re-validation here — the wire format derives each block's parent from the previous
// hash in the same array, so it is chain-consistent by construction; what protects the driver
// from a wrong-fork answer are the anchor comparisons (merge across fetchers, the canonical
// header in the transfer step, and the block builder's header list).
func verifiedLinks(part evm.BlockHashLinkPart) ([]evm.BlockLink, error) {
	return part.Links()
}

// linkIndex maps a covered block number to its link.
func linkIndex(links []evm.BlockLink) map[uint64]evm.BlockLink {
	dict := make(map[uint64]evm.BlockLink, len(links))
	for _, link := range links {
		dict[link.Number] = link
	}
	return dict
}

// checkLinksReachEnd verifies that the identity coverage of an Ex response extends to the last
// requested block. The super node guards this on its side too, but an older or lagging server
// could still trim the upper part of the range (blocks it does not have yet), and treating the
// missing tail as empty blocks would be silent loss. No link at all stays accepted: that is the
// slot-cache-only super node serving a deep-history range entirely from upstream.
func checkLinksReachEnd(links []evm.BlockLink, end uint64) error {
	if len(links) > 0 && links[len(links)-1].Number < end {
		return errors.Errorf(
			"the identity coverage of the Ex response ends at block %d but block %d was requested, "+
				"the endpoint's head is behind, will retry",
			links[len(links)-1].Number, end)
	}
	return nil
}

// emptyEntriesFromLinks pre-fills the per-block result map with header-only entries for every
// covered block: a covered block with zero matching data still carries its identity, which is
// the entire point of the Ex queries — an empty result becomes verifiable instead of
// indistinguishable from silently lost data.
func emptyEntriesFromLinks(links []evm.BlockLink) map[uint64]BlockMainData {
	result := make(map[uint64]BlockMainData, len(links))
	for _, link := range links {
		header := headerFromLink(link)
		result[link.Number] = BlockMainData{Header: &header}
	}
	return result
}

// setTraceBlockHash backfills the block hash stripped by the Ex wire format into both the parsed
// fields and the Raw JSON — Raw is what gets handed to the processor, so patching only the
// struct would ship traces without blockHash downstream.
func setTraceBlockHash(trace *Trace, blockHash string) error {
	trace.BlockHash = blockHash
	var m map[string]json.RawMessage
	if err := json.Unmarshal(trace.Raw, &m); err != nil {
		return errors.Wrapf(err, "patch blockHash into trace %s failed", trace.TransactionHash)
	}
	raw, err := json.Marshal(blockHash)
	if err != nil {
		return err
	}
	m["blockHash"] = raw
	if trace.Raw, err = json.Marshal(m); err != nil {
		return errors.Wrapf(err, "patch blockHash into trace %s failed", trace.TransactionHash)
	}
	return nil
}

// checkTipCovered requires an Ex response to vouch for the tip block itself: at the tip the
// serving super node always holds the block in its latest slot cache, so a missing identity
// there means the endpoint (an older server without the over-the-head guard, or a lagging
// replica) trimmed the request, and accepting it would be silent loss.
func checkTipCovered(entries map[uint64]BlockMainData, end uint64) error {
	if entry, has := entries[end]; !has || entry.Header == nil {
		return errors.Errorf(
			"the Ex response did not cover the requested tip block %d, the endpoint's head is behind, will retry", end)
	}
	return nil
}

// checkTracesBlockHash verifies that every trace claims the expected block; an empty result
// stays unverifiable this way (trace_filter has no by-hash form), which is why the Ex variant is
// preferred wherever available.
func checkTracesBlockHash(traces []Trace, blockNumber uint64, blockHash string) error {
	if blockHash == "" {
		return nil
	}
	for i := range traces {
		if traces[i].BlockHash != blockHash {
			return errors.Errorf(
				"trace of tx %s in block %d has block hash %s but the expected block is %s, "+
					"the endpoint answered from a different fork, will retry",
				traces[i].TransactionHash, blockNumber, traces[i].BlockHash, blockHash)
		}
	}
	return nil
}

// tracesFromExForBlock extracts the traces of exactly one block from a trace_filterEx response,
// verifying the response's identity against the expected block hash (when given) — which makes
// even an EMPTY trace set verifiable — and backfilling the stripped blockHash into the traces.
func tracesFromExForBlock(resp GetTracesExResponse, blockNumber uint64, blockHash string) ([]Trace, error) {
	links, err := verifiedLinks(resp.BlockHashLinkPart)
	if err != nil {
		return nil, err
	}
	link, covered := linkIndex(links)[blockNumber]
	if !covered {
		// deep history on a slot-cache-only super node: no identity available, verify whatever
		// the traces themselves carry
		return resp.Traces, checkTracesBlockHash(resp.Traces, blockNumber, blockHash)
	}
	if blockHash != "" && link.Hash.Hex() != blockHash {
		return nil, errors.Errorf(
			"trace_filterEx answered block %d from %s but the expected block is %s, "+
				"the endpoint answered from a different fork, will retry",
			blockNumber, link.Hash, blockHash)
	}
	traces := resp.Traces
	for i := range traces {
		if err = setTraceBlockHash(&traces[i], link.Hash.Hex()); err != nil {
			return nil, err
		}
	}
	return traces, nil
}
