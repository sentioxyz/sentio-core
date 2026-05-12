package sui

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/clientpool"
	rg "sentioxyz/sentio-core/common/range"
	"strconv"
	"time"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"

	"github.com/goccy/go-json"
)

type ExtServerDimension struct {
	client *ClientPool

	skipValidate bool

	*chain.ExtServerDimension[*Slot]
}

func NewExtServerDimension(
	client *ClientPool,
	skipValidate bool,
	loadConcurrency uint,
	loadRetry int,
	validRange rg.Range,
	fallBehind time.Duration,
) *ExtServerDimension {
	dim := &ExtServerDimension{client: client, skipValidate: skipValidate}
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

func (d *ExtServerDimension) loadCheckpoint(
	ctx context.Context,
	checkpoint uint64,
	loadTxsConcurrency int,
	loadTxsPageSize int,
	loadTxnOptions map[string]any,
) (ckpt types.CheckpointResponse, txns []types.TransactionResponseV1, err error) {
	_, logger := log.FromContext(ctx)
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlot.CkptPart/%d", checkpoint),
		func(ctx context.Context, cli *Client) clientpool.Result {
			result := cli.CallContext(
				ctx,
				&ckpt,
				"ext.GetSlot.CkptPart",
				"sui_getCheckpoint",
				strconv.FormatUint(checkpoint, 10),
			)
			result.BrokenForTask = result.Err != nil // always retry using other client
			return result
		},
		clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("sui_getCheckpoint")),
	)
	if r.Err != nil {
		logger.Errorfe(r.Err, "load checkpoint failed")
		return
	}

	txns, err = concurrency.TraverseByPage(
		ctx, loadTxsConcurrency, loadTxsPageSize, ckpt.Transactions,
		func(ctx context.Context, page concurrency.Page, digests []string) ([]types.TransactionResponseV1, error) {
			_, pageLogger := log.FromContext(ctx)
			var rawTxs []json.RawMessage
			fetchResult := d.client.UseClient(
				ctx,
				fmt.Sprintf("ext.GetSlot.TxsPart/%d/%d#%d-%d", checkpoint, page.Num, page.Start, page.End),
				func(ctx context.Context, cli *Client) clientpool.Result {
					result := cli.CallContext(
						ctx,
						&rawTxs,
						"ext.GetSlot.TxsPart",
						"sui_multiGetTransactionBlocks",
						digests,
						loadTxnOptions,
					)
					result.BrokenForTask = result.Err != nil // always retry using other client
					return result
				},
				clientpool.WithoutTags[ClientConfig](clientpool.MethodNotSupportedTag("sui_multiGetTransactionBlocks")),
			)
			if fetchResult.Err != nil {
				pageLogger.Errorfe(fetchResult.Err, "load transactions failed")
				return nil, fetchResult.Err
			}
			if len(rawTxs) != len(digests) {
				pageLogger.Errorf("result length is %d, not %d", len(rawTxs), len(digests))
				return nil, errors.Errorf("result length is %d, not %d", len(rawTxs), len(digests))
			}
			var pageTxs []types.TransactionResponseV1
			for i, rawTx := range rawTxs {
				digest := digests[i]
				var tx types.TransactionResponseV1
				if unmarshalErr := json.Unmarshal(rawTx, &tx); unmarshalErr != nil {
					pageLogger.With("digest", digest).Errore(unmarshalErr, "unmarshal tx failed")
					return nil, errors.Wrapf(unmarshalErr, "unmarshal txn %d/%s failed", checkpoint, digest)
				}
				if tx.Checkpoint.Uint64() != checkpoint {
					pageLogger.With("digest", digest).Errorf("unexpected checkpoint %d for tx %s", tx.Checkpoint.Uint64(), digest)
					return nil, errors.Errorf("txn %d/%s has unexpected checkpoint %d", checkpoint, digest, tx.Checkpoint.Uint64())
				}
				tx.CheckpointTimestampMs = &ckpt.TimestampMs
				tx.TransactionPosition = page.Start + i
				pageTxs = append(pageTxs, tx)
			}
			return pageTxs, nil
		})
	return
}

var uncompletedKinds = map[string]bool{
	"Genesis":                   true,
	"EndOfEpochTransaction":     true,
	"ConsensusCommitPrologueV3": true,
	"ConsensusCommitPrologueV4": true,
	"ConsensusCommitPrologueV1": true,
	"RandomnessStateUpdate":     true,
}

const (
	loadTxsConcurrency = 10
	loadTxsPageSize    = 50
)

var loadTxnOptions = map[string]interface{}{
	"showInput":          true,
	"showRawInput":       true,
	"showEffects":        true,
	"showEvents":         true,
	"showObjectChanges":  true,
	"showBalanceChanges": true,
}

func (d *ExtServerDimension) GetSlot(ctx context.Context, sn uint64) (*Slot, error) {
	ctx, logger := log.FromContext(ctx, "checkpoint", sn)

	ckpt, txns, err := d.loadCheckpoint(ctx, sn, loadTxsConcurrency, loadTxsPageSize, loadTxnOptions)
	if err != nil {
		return nil, err
	}

	var slotTxns []types.TransactionResponseV1
	var uncompleteTxns int
	for _, tx := range txns {
		if tx.Transaction == nil || tx.Transaction.Data == nil ||
			tx.Transaction.Data.V1 == nil || tx.Transaction.Data.V1.Kind == nil {
			// This tx should contain at least one error.
			if len(tx.Errors) == 0 {
				return nil, errors.Errorf("invalid transaction %d/%s, required fields not present", sn, tx.Digest.String())
			}
		} else if kind := tx.Transaction.Data.V1.Kind.Kind(); uncompletedKinds[kind] || d.skipValidate {
			// No decoding/sanity check on Genesis and ConsensusCommitPrologueV3 transaction.
			logger.Debugf("Skipping decoding %s transaction %s", kind, tx.Digest.String())
			uncompleteTxns++
		} else {
			if err = types.DeriveAuxInformationFromBCSV1(tx.Transaction.Data.V1, tx.RawTransaction.Data()); err != nil {
				return nil, errors.Wrapf(err, "derive aux information from BCS for %d/%s failed", sn, tx.Digest.String())
			}
			if err = TxSanityCheck(&tx); err != nil {
				return nil, errors.Wrapf(err, "sanity check for %d/%s failed", sn, tx.Digest.String())
			}
		}
		slotTxns = append(slotTxns, tx)
	}
	if uncompleteTxns > 0 {
		logger.Debugf("%d txns skipped decoding check", uncompleteTxns)
	}
	return &Slot{
		SlotCheckpointInfo: SlotCheckpointInfo{
			SequenceNumber:     sn,
			Digest:             ckpt.Digest.String(),
			TransactionDigests: ckpt.Transactions,
			TimestampMs:        ckpt.TimestampMs,
		},
		Transactions: slotTxns,
	}, nil
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

func (d *ExtServerDimension) GetSlotHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	return &Slot{SlotCheckpointInfo: SlotCheckpointInfo{SequenceNumber: sn}}, nil
}
