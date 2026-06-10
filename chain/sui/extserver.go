package sui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/goccy/go-json"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

type ExtServerDimension struct {
	client *ClientPool

	// variation selects the chain-specific BCS enum layout (sui vs iota) when
	// decoding/re-encoding json-rpc transactions.
	variation types.Variation

	enableJSONRPC bool
	skipValidate  bool

	enableGrpc             bool
	loadObjectsBatchSize   int
	loadObjectsConcurrency int

	*chain.ExtServerDimension[*Slot]
}

func NewExtServerDimension(
	client *ClientPool,
	variation types.Variation,
	enableJSONRPC bool,
	skipValidate bool,
	enableGrpc bool,
	loadObjectsBatchSize int,
	loadObjectsConcurrency int,
	loadConcurrency uint,
	loadRetry int,
	validRange rg.Range,
	fallBehind time.Duration,
) *ExtServerDimension {
	if !enableJSONRPC && !enableGrpc {
		panic("both json-rpc and grpc data are disabled")
	}
	if variation == "" {
		variation = types.VariationSUI
	}
	dim := &ExtServerDimension{
		client:                 client,
		variation:              variation,
		enableJSONRPC:          enableJSONRPC,
		skipValidate:           skipValidate,
		enableGrpc:             enableGrpc,
		loadObjectsBatchSize:   loadObjectsBatchSize,
		loadObjectsConcurrency: loadObjectsConcurrency,
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

// uncompletedKinds are transaction kinds whose Go types are not yet exact, so
// slot loading skips DeriveAux/TxSanityCheck for them rather than failing.
// ConsensusCommitPrologueV4 (Sui) and ConsensusCommitPrologueV1 (IOTA) are now
// complete and validated against real testnet samples (see
// consensus_commit_prologue_test.go). V2/V3 stay here until a historical sample
// is captured to validate their round-trip.
var uncompletedKinds = map[string]bool{
	"Genesis":                   true,
	"EndOfEpochTransaction":     true,
	"ConsensusCommitPrologueV2": true,
	"ConsensusCommitPrologueV3": true,
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

func (d *ExtServerDimension) getSlot(ctx context.Context, sn uint64) (*Slot, error) {
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
			if err = types.DeriveAuxInformationFromBCSV1(tx.Transaction.Data.V1, tx.RawTransaction.Data(), d.variation); err != nil {
				return nil, errors.Wrapf(err, "derive aux information from BCS for %d/%s failed", sn, tx.Digest.String())
			}
			if err = TxSanityCheck(&tx, d.variation); err != nil {
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

func (d *ExtServerDimension) getGrpcCheckpoint(ctx context.Context, sn uint64) (*rpcv2.Checkpoint, error) {
	var resp *rpcv2.GetCheckpointResponse
	r := d.client.UseClient(
		ctx,
		fmt.Sprintf("ext.GetSlot.MainPart.grpc_GetCheckpoint/%d", sn),
		func(ctx context.Context, cli *Client) clientpool.Result {
			return cli.UseGRPCConnection(ctx, "ext.GetSlot.MainPart.grpc_GetCheckpoint",
				func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result {
					req := &rpcv2.GetCheckpointRequest{
						CheckpointId: &rpcv2.GetCheckpointRequest_SequenceNumber{SequenceNumber: sn},
						ReadMask:     &fieldmaskpb.FieldMask{Paths: []string{"*"}},
					}
					var err error
					resp, err = rpcv2.NewLedgerServiceClient(conn).GetCheckpoint(ctx, req)
					return clientpool.Result{
						Err:           err,
						BrokenForTask: err != nil, // always retry using other client
					}
				},
			)
		},
		clientpool.WithConfigFilter(ClientConfig.SupportGRPC),
	)
	if r.Err != nil {
		return nil, r.Err
	}
	return resp.GetCheckpoint(), nil
}

func (d *ExtServerDimension) getGrpcSlot(ctx context.Context, sn uint64) (*Slot, error) {
	ck, err := d.getGrpcCheckpoint(ctx, sn)
	if err != nil {
		return nil, err
	}
	s := &Slot{GrpcCheckpoint: ck}
	s.loadCheckpointInfo()
	// Although ck.GetObjects().GetObjects() have all related objects, but ck.GetObjects().GetObjects()[*].GetJson()
	// will be empty, so have to use grpc_getObjects here to fetch all related objects.
	// The fetch result will be saved in Slot.GrpcObjects
	var objReqs []*rpcv2.GetObjectRequest
	for _, obj := range ck.GetObjects().GetObjects() {
		objReqs = append(objReqs, &rpcv2.GetObjectRequest{
			ObjectId: obj.ObjectId,
			Version:  obj.Version,
		})
	}
	results, getObjErr := d.client.GetGrpcObjectsByPage(
		ctx,
		fmt.Sprintf("ext.GetSlot.ObjectsPart.grpc_BatchGetObjects/%d", sn),
		"ext.GetSlot.ObjectsPart.grpc_BatchGetObjects",
		d.loadObjectsConcurrency,
		d.loadObjectsBatchSize,
		objReqs,
	)
	if getObjErr != nil {
		return nil, getObjErr
	}
	objects := make(ObjectSet)
	for i, res := range results {
		if res.GetError() != nil {
			return nil, errors.Errorf("load object %s/%d in checkpoint %d failed: %s",
				objReqs[i].GetObjectId(), objReqs[i].GetVersion(), sn, utils.MustJSONMarshal(res.GetError()))
		}
		objects.Put(res.GetObject())
	}
	s.GrpcObjects = objects
	// fill changed object in tx effects
	for _, tx := range ck.Transactions {
		for i, co := range tx.GetEffects().GetChangedObjects() {
			changeType := GetChangeType(co)
			// output version
			if co.GetOutputVersion() == 0 {
				co.OutputVersion = tx.GetEffects().LamportVersion
			}
			if co.GetOutputVersion() == 0 {
				return nil, errors.Errorf("changed object #%d/%d in transaction %d/%s with id %s miss version",
					i, len(tx.GetEffects().GetChangedObjects()), sn, tx.GetDigest(), co.GetObjectId())
			}
			// these kinds carry no pre-object / input owner / object type to enrich
			// (e.g. accumulator writes have input_version 0 and no owner/type), mirror chv4 convert
			if changeType == types.ObjectChangeTypeUnknown ||
				changeType == types.ObjectChangeTypeAccumulatorWrite ||
				changeType == types.ObjectChangeTypeUnwrappedThenDeleted {
				continue
			}
			// input owner
			// the early data may miss co.InputOwner, in this situation we need to get the pre-owner from the pre-object
			if !changeType.IsCreated() && co.GetInputOwner() == nil {
				pre, has := objects.Get(co.GetObjectId(), co.GetInputVersion())
				if !has {
					return nil, errors.Errorf("object %s/%d of in transaction %d/%s not found in checkpoint objects",
						co.GetObjectId(), co.GetInputVersion(), sn, tx.GetDigest())
				}
				co.InputOwner = pre.GetOwner()
			}
			// object type
			if co.GetObjectType() == "" {
				if changeType.IsDeleted() {
					pre, has := objects.Get(co.GetObjectId(), co.GetInputVersion())
					if !has {
						return nil, errors.Errorf("object %s/%d of in transaction %d/%s not found in checkpoint objects",
							co.GetObjectId(), co.GetInputVersion(), sn, tx.GetDigest())
					}
					co.ObjectType = pre.ObjectType
				} else {
					cur, has := objects.Get(co.GetObjectId(), co.GetOutputVersion())
					if !has {
						return nil, errors.Errorf("object %s/%d of in transaction %d/%s not found in checkpoint objects",
							co.GetObjectId(), co.GetOutputVersion(), sn, tx.GetDigest())
					}
					co.ObjectType = cur.ObjectType
				}
			}
		}
	}
	s.removeBcs()
	return s, nil
}

func (d *ExtServerDimension) GetSlot(ctx context.Context, sn uint64) (*Slot, error) {
	var slot Slot
	if d.enableJSONRPC {
		if st, err := d.getSlot(ctx, sn); err != nil {
			return nil, err
		} else {
			slot.SlotCheckpointInfo = st.SlotCheckpointInfo
			slot.HasJSONRPCData = true
			slot.Transactions = st.Transactions
		}
	}
	if d.enableGrpc {
		if st, err := d.getGrpcSlot(ctx, sn); err != nil {
			return nil, err
		} else {
			if !d.enableJSONRPC {
				// when json-rpc is disabled, the checkpoint header info comes from grpc data
				slot.SlotCheckpointInfo = st.SlotCheckpointInfo
			}
			slot.GrpcCheckpoint = st.GrpcCheckpoint
			slot.GrpcObjects = st.GrpcObjects
		}
	}
	return &slot, nil
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

func (d *ExtServerDimension) Snapshot() any {
	sn := d.ExtServerDimension.Snapshot().(map[string]any)
	utils.MergeMap(sn, map[string]any{
		"enableJSONRPC":          d.enableJSONRPC,
		"skipValidate":           d.skipValidate,
		"enableGrpc":             d.enableGrpc,
		"loadObjectsBatchSize":   d.loadObjectsBatchSize,
		"loadObjectsConcurrency": d.loadObjectsConcurrency,
	})
	return sn
}
