package evm

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type NetworkOptions struct {
	DisableTrace      bool
	SupportTraceBlock bool
	IgnoreExtraTraces bool
	IgnoreMissTraces  bool
}

type ExtServerDimension struct {
	chainID string
	opts    NetworkOptions

	client *ClientPool

	*chain.ExtServerDimension[*Slot]
}

var _ chain.Dimension[*Slot] = (*ExtServerDimension)(nil)

func NewExtServerDimension(
	client *ClientPool,
	loadConcurrency uint,
	loadRetry int,
	validRange rg.Range,
	chainID string,
	opts NetworkOptions,
	fallBehind time.Duration,
) *ExtServerDimension {
	dim := &ExtServerDimension{
		chainID: chainID,
		opts:    opts,
		client:  client,
	}
	// loadBatchSize more than 1 is meaningless
	dim.ExtServerDimension = chain.NewExtServerDimension[*Slot](
		client,
		loadConcurrency,
		1,
		loadRetry,
		validRange,
		fallBehind,
		dim)
	return dim
}

func (d *ExtServerDimension) GetSlotHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	var block RPCGetBlockResponse
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlotHeader/%d", sn),
		func(ctx context.Context, cli *Client) (r clientpool.Result) {
			block, r = cli.GetBlock(ctx, "ext.GetSlotHeader", sn, false)
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
		clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("eth_getBlockByNumber")),
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "get header for block %d (%s) failed", sn, r.ConfigName)
	}
	return &Slot{Header: block.ExtendedHeader}, nil
}

func (d *ExtServerDimension) GetSlot(ctx context.Context, sn uint64) (*Slot, error) {
	var block RPCGetBlockResponse
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlot.BlockPart/%d", sn),
		func(ctx context.Context, cli *Client) (r clientpool.Result) {
			block, r = cli.GetBlock(ctx, "ext.GetSlot.BlockPart", sn, true)
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
		clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("eth_getBlockByNumber")),
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "get header and transactions for block %d (%s) failed", sn, r.ConfigName)
	}
	st := Slot{
		Header: block.ExtendedHeader,
		Block: &RPCBlock{
			Transactions: block.Transactions,
			Uncles:       block.Uncles,
			Withdrawals:  block.Withdrawals,
		},
	}

	if err := d.loadReceipts(ctx, &st); err != nil {
		return nil, errors.Wrapf(err, "load receipts for block %d failed", sn)
	}

	if err := d.loadTraces(ctx, &st); err != nil {
		return nil, errors.Wrapf(err, "load traces for block %d failed", sn)
	}

	return &st, nil
}

func (d *ExtServerDimension) loadReceipts(ctx context.Context, st *Slot) error {
	_, logger := log.FromContext(ctx)
	blockNumber := st.Header.Number.Uint64()
	blockHash := st.Header.Hash

	// first try to use eth_getBlockReceipts
	var receipts []ExtendedReceipt
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlot.ReceiptPart/%d", blockNumber),
		func(ctx context.Context, cli *Client) clientpool.Result {
			r := cli.CallContext(
				ctx,
				&receipts,
				"ext.GetSlot.ReceiptPart",
				"eth_getBlockReceipts",
				hexutil.Uint64(blockNumber),
			)
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
		clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("eth_getBlockReceipts")),
	)
	var miss []common.Hash
	receiptDict := make(map[common.Hash]ExtendedReceipt)
	if r.Err == nil {
		method := fmt.Sprintf("eth_getBlockReceipts %d (%s)", blockNumber, r.ConfigName)
		for i, receipt := range receipts {
			if receipt.BlockHash != blockHash {
				return errors.Errorf(
					"%d/%d receipt in the result of %s has block hash %s but the block is %d/%s",
					i, len(receipts), method, receipt.BlockHash, blockNumber, blockHash)
			}
			receiptDict[receipt.TxHash] = receipt
		}
		for i, tx := range st.Block.Transactions {
			if _, has := receiptDict[tx.Hash]; !has {
				logger.Warnf("miss receipt for tx %d/%s in block %d/%s in the result of %s, "+
					"will retry using eth_getTransactionReceipt", i, tx.Hash, blockNumber, blockHash, method)
				miss = append(miss, tx.Hash)
				continue
			}
		}
	} else {
		// all transactions need to use eth_getTransactionReceipt
		for _, tx := range st.Block.Transactions {
			miss = append(miss, tx.Hash)
		}
	}

	// try to use eth_getTransactionReceipt
	for _, txHash := range miss {
		var receipt *ExtendedReceipt
		r = d.client.UseClient(
			ctx,
			fmt.Sprintf("ext.GetSlot.ReceiptPart/%d.%s", blockNumber, txHash),
			func(ctx context.Context, cli *Client) clientpool.Result {
				r1 := cli.CallContext(
					ctx,
					&receipt,
					"ext.GetSlot.ReceiptPart",
					"eth_getTransactionReceipt",
					txHash,
				)
				r1.BrokenForTask = r1.Err != nil // always retry using other client
				return r1
			},
			clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("eth_getTransactionReceipt")),
		)
		method := fmt.Sprintf("eth_getTransactionReceipt %s (%s)", txHash, r.ConfigName)
		if r.Err != nil {
			return errors.Wrapf(r.Err, "%s for block %d/%s failed", method, blockNumber, blockHash)
		}
		if receipt == nil {
			return errors.Errorf("%s for block %d/%s got nil", method, blockNumber, blockHash)
		}
		receiptDict[txHash] = *receipt
	}

	// fill st.Receipts and st.Logs
	st.Receipts = make([]ExtendedReceipt, len(st.Block.Transactions))
	st.Logs = nil
	for i, tx := range st.Block.Transactions {
		st.Receipts[i] = receiptDict[tx.Hash]
		for _, lg := range st.Receipts[i].Logs {
			st.Logs = append(st.Logs, *lg)
		}
	}
	return nil
}

var gethTracer = map[string]interface{}{
	"tracer":  "callTracer",
	"timeout": "30s",
}

func traceTxMatch(trace GethTraceBlockResult, tx RPCTransaction) bool {
	if trace.Result == nil {
		return false
	}
	if trace.TxHash != nil {
		return *trace.TxHash == tx.Hash
	}
	var score int
	if trace.Result.From != nil && *trace.Result.From == tx.From {
		score++
	}
	if tx.To == nil {
		return trace.Result.Type == "CREATE"
	} else {
		if strings.EqualFold(trace.Result.To, tx.To.String()) {
			score++
		}
	}
	if bytes.Equal(trace.Result.Input, tx.Input) {
		score++
	}
	// may be `From` not match, but `Input` and `To` are both math
	// example: opt mainnet (chainID=10) block 102
	return score >= 2
}

func (d *ExtServerDimension) loadTraces(ctx context.Context, st *Slot) error {
	if d.opts.DisableTrace {
		return nil
	}

	_, logger := log.FromContext(ctx)
	blockNumber := st.Header.Number.Uint64()
	blockHash := st.Header.Hash
	txIndexMap := make(map[common.Hash]uint64)
	for _, tx := range st.Block.Transactions {
		txIndexMap[tx.Hash] = uint64(tx.TransactionIndex)
	}

	if blockNumber == 0 {
		info, has := chains.EthChainIDToInfo[chains.ChainID(d.chainID)]
		if has && info.Variation == chains.EthVariationOptimism {
			// the genesis block of Optimistic Rollup chains is not traceable
			return nil
		}
	}

	checkSlotTraces := func(method string) error {
		for i, trace := range st.Traces {
			errMsgTitle := fmt.Sprintf("%d/%d trace in the result of %s", i, len(st.Traces), method)
			if trace.BlockHash != blockHash {
				return errors.Errorf("%s has block hash %s but the block is %d/%s",
					errMsgTitle, trace.BlockHash, blockNumber, blockHash)
			}
			if trace.BlockNumber != blockNumber {
				return errors.Errorf("%s has unexpected block number %d", errMsgTitle, trace.BlockNumber)
			}
			if trace.TransactionHash == nil {
				return errors.Errorf("%s miss transaction hash", errMsgTitle)
			}
			if txIndex, has := txIndexMap[*trace.TransactionHash]; !has {
				return errors.Errorf("%s has transaction hash %s and but the transaction does not exist in the block",
					errMsgTitle, *trace.TransactionHash)
			} else if trace.TransactionPosition != txIndex {
				return errors.Errorf("%s has transaction hash %s and index %d but the transaction index exactly is %d",
					errMsgTitle, *trace.TransactionHash, trace.TransactionPosition, txIndex)
			}
		}
		return nil
	}

	if d.chainID == string(chains.ArbitrumID) && blockNumber < 22207818 {
		// Arbitrum classic
		r := d.client.UseClient(
			ctx,
			fmt.Sprintf("ext.GetSlot.TracePart/%d", blockNumber),
			func(ctx context.Context, cli *Client) clientpool.Result {
				r := cli.CallContext(
					ctx,
					&st.Traces,
					"ext.GetSlot.TracePart",
					"arbtrace_block",
					hexutil.Uint64(blockNumber),
				)
				r.BrokenForTask = r.Err != nil // always retry using other client
				return r
			},
			clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("arbtrace_block")),
		)
		method := fmt.Sprintf("arbtrace_block %d (%s)", blockNumber, r.ConfigName)
		if r.Err == nil {
			return checkSlotTraces(method)
		}
		if blockNumber == 0 && strings.Contains(strings.ToLower(r.Err.Error()), "genesis is not traceable") {
			return nil
		}
		if blockNumber == 1 && strings.Contains(strings.ToLower(r.Err.Error()), "creating execution cursor") {
			return nil
		}
		return errors.Wrapf(r.Err, "call %s failed", method)
	}

	if d.opts.SupportTraceBlock {
		// first try to use trace_block
		r := d.client.UseClient(
			ctx,
			fmt.Sprintf("ext.GetSlot.TracePart/%d", blockNumber),
			func(ctx context.Context, cli *Client) clientpool.Result {
				r := cli.CallContext(
					ctx,
					&st.Traces,
					"ext.GetSlot.TracePart",
					"trace_block",
					hexutil.Uint64(blockNumber),
				)
				r.BrokenForTask = r.Err != nil // always retry using other client
				return r
			},
			clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("trace_block")),
		)
		method := fmt.Sprintf("trace_block %d (%s)", blockNumber, r.ConfigName)
		if r.Err == nil {
			return checkSlotTraces(method)
		}
		if blockNumber == 0 && strings.Contains(strings.ToLower(r.Err.Error()), "genesis is not traceable") {
			return nil
		}
	}

	// trace_filter failed, try to use debug_traceBlockByHash
	var gr []GethTraceBlockResult
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlot.TracePart/%d", blockNumber),
		func(ctx context.Context, cli *Client) clientpool.Result {
			r := cli.CallContext(
				ctx,
				&gr,
				"ext.GetSlot.TracePart",
				"debug_traceBlockByHash",
				blockHash,
				gethTracer,
			)
			r.BrokenForTask = r.Err != nil // always retry using other client
			return r
		},
		clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("debug_traceBlockByHash")),
	)
	method := fmt.Sprintf("debug_traceBlockByHash %d/%s (%s)", blockNumber, blockHash, r.ConfigName)
	if r.Err != nil {
		if blockNumber == 0 && strings.Contains(strings.ToLower(r.Err.Error()), "genesis is not traceable") {
			return nil
		}
		return errors.Wrapf(r.Err, "call %s failed", method)
	}
	for i, tx := range st.Block.Transactions {
		if len(gr) == 0 || !traceTxMatch(gr[0], tx) {
			if !d.opts.IgnoreMissTraces {
				return errors.Errorf("miss trace for tx %d/%s in the result of %s", i, tx.Hash, method)
			}
			logger.Warnf("miss trace for tx %d/%s in the result of %s", i, tx.Hash, method)
			continue
		}
		items := GethToParityTrace(gr[0].Result, nil)
		for _, ptr := range items {
			pr := *ptr
			// Fill in block/tx information.  These are necessary fields for ParityTrace.
			pr.BlockNumber = blockNumber
			pr.BlockHash = blockHash
			pr.TransactionHash = utils.WrapPointer(tx.Hash)
			pr.TransactionPosition = uint64(tx.TransactionIndex)
			st.Traces = append(st.Traces, pr)
		}
		gr = gr[1:]
	}
	if len(gr) > 0 {
		if !d.opts.IgnoreExtraTraces {
			return errors.Errorf("has %d extra trace in the result of %s", len(gr), method)
		}
		logger.Warnf("has %d extra trace in the result of %s", len(gr), method)
	}
	return nil
}

func (d *ExtServerDimension) GetSlots(ctx context.Context, sr rg.Range) ([]*Slot, error) {
	slots := make([]*Slot, 0, *sr.Size())
	for sn := sr.Start; sn <= *sr.End; sn++ {
		st, err := d.GetSlot(ctx, sn)
		if err != nil {
			return nil, err
		}
		slots = append(slots, st)
	}
	return slots, nil
}
