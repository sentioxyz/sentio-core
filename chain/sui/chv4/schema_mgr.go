package chv4

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"math/big"
	"reflect"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/objectx"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type ClickhouseSchemaMgr struct {
	tablesMeta clickhouse.TablesMeta

	balanceCtrl balanceController
}

const (
	tableNameCheckpoints  = "checkpoints"
	tableNameTransactions = "transactions"
	tableNameEvents       = "events"
	tableNameObjects      = "objects"
	tableNameBalances     = "balances"
)

func NewClickhouseSchemaMgr(
	ctrl chx.Controller,
	checkpointPartitionSize uint64,
	balanceStorePath string,
) (*ClickhouseSchemaMgr, error) {
	engine := ctrl.NewDefaultMergeTreeEngine()
	tableSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(tableSettings)
	chx.WithProjectionTableSettings(tableSettings)
	partitionBy := fmt.Sprintf("intDiv(checkpoint, %d)", checkpointPartitionSize)
	createTableSchema := func(name string, tblObj any, orderBy ...string) clickhouse.TableSchema {
		config := chx.TableConfig{
			Engine:      engine,
			PartitionBy: partitionBy,
			OrderBy:     orderBy,
			Settings:    tableSettings,
		}
		return clickhouse.BuildTable(name, tblObj, config, "")
	}
	tables := []clickhouse.TableSchema{
		createTableSchema(tableNameCheckpoints, &Checkpoint{}, "checkpoint", "checkpoint_digest"),
		createTableSchema(tableNameTransactions, &Transaction{}, "checkpoint", "tx_index", "tx_digest"),
		createTableSchema(tableNameEvents, &Event{}, "checkpoint", "tx_index", "event_index"),
		createTableSchema(tableNameObjects, &Object{}, "checkpoint", "tx_index", "object_id"),
		createTableSchema(tableNameBalances, &Balance{}, "checkpoint", "tx_index", "address"),
	}
	mgr := &ClickhouseSchemaMgr{
		tablesMeta: clickhouse.TablesMeta{
			Tables:          tables,
			LinkTableIndex:  -1,
			BlockTableIndex: -1,
		},
	}

	err := mgr.balanceCtrl.Init(ctrl, balanceStorePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init balance store with path %q", balanceStorePath)
	}
	return mgr, nil
}

func (m *ClickhouseSchemaMgr) GetTablesMeta() clickhouse.TablesMeta {
	return m.tablesMeta
}

var jsonEncoder = jsonpb.Marshaler{
	EnumsAsInts: false,
}

func mustBuildJSON[T proto.Message](title string, d T) string {
	if reflect.ValueOf(d).IsNil() {
		return "null"
	}
	s, err := jsonEncoder.MarshalToString(d)
	if err != nil {
		panic(errors.Wrapf(err, "build json for %s failed", title))
	}
	return s
}

func mustBuildJSONArray[T proto.Message](title string, d []T) []string {
	r := make([]string, len(d))
	for i, v := range d {
		r[i] = mustBuildJSON(fmt.Sprintf("%d/%d %s", i, len(d), title), v)
	}
	return r
}

var jsonDecoder = jsonpb.Unmarshaler{}

func decodeJSON[T proto.Message](d string, v T) (bool, error) {
	d = strings.TrimSpace(d)
	if d == "" || d == "null" {
		return false, nil
	}
	return true, jsonDecoder.Unmarshal(strings.NewReader(d), v)
}

func (m *ClickhouseSchemaMgr) convert(ctx context.Context, ck *rpcv2.Checkpoint) (
	checkpoint Checkpoint,
	txs []Transaction,
	events []Event,
	objects []Object,
	balances []Balance,
	err error,
) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			var is bool
			if err, is = panicErr.(error); !is {
				err = errors.Errorf("%v", panicErr)
			}
		}
		if err == nil {
			err = m.balanceCtrl.IncrCursor(ck.GetSequenceNumber())
		}
		if err != nil {
			_, logger := log.FromContext(ctx, "checkpoint", ck.GetSequenceNumber())
			logger.Errorfe(err, "convert checkpoint failed")
			err = errors.Wrapf(err, "convert checkpoint %d failed", ck.GetSequenceNumber())
		}
	}()
	// Normally, all checkpoints will call convert sequentially.
	// If failed midway, the segment at the tail will be restarted, and the saved balance data will need to be rollback.
	if err = m.balanceCtrl.Align(ctx, ck.GetSequenceNumber()-1); err != nil {
		err = errors.Wrapf(err, "align balance store for checkpoint %d failed", ck.GetSequenceNumber())
		return
	}
	// build dict for ck.GetObjects()
	objectDict := make(map[string]map[uint64]*rpcv2.Object)
	for _, obj := range ck.GetObjects().GetObjects() {
		utils.PutIntoK2Map(objectDict, obj.GetObjectId(), obj.GetVersion(), obj)
	}
	// === checkpoint
	ckTitle := fmt.Sprintf("of checkpoint %d", ck.GetSequenceNumber())
	checkpoint = Checkpoint{
		CheckpointIndex: CheckpointIndex{
			Checkpoint:       ck.GetSequenceNumber(),
			CheckpointDigest: ck.GetDigest(),
			Timestamp:        ck.GetSummary().GetTimestamp().AsTime(),
			Epoch:            ck.GetSummary().GetEpoch(),
		},
		Summary:          mustBuildJSON("summary "+ckTitle, ck.GetSummary()),
		Signature:        mustBuildJSON("signature "+ckTitle, ck.GetSignature()),
		Contents:         mustBuildJSON("contents "+ckTitle, ck.GetContents()),
		TransactionCount: uint64(len(ck.GetTransactions())),
	}
	// === transactions
	for i, tx := range ck.GetTransactions() {
		place := fmt.Sprintf("in %d/%d executed transaction %s %s", i, len(ck.GetTransactions()), tx.GetDigest(), ckTitle)
		chtx := Transaction{
			CheckpointIndex: checkpoint.CheckpointIndex,
			TransactionIndex: TransactionIndex{
				TxDigest: tx.GetDigest(),
				TxIndex:  uint64(i),
			},
			Signatures:     mustBuildJSONArray("signature "+place, tx.GetSignatures()),
			Transaction:    mustBuildJSON("transaction "+place, tx.GetTransaction()),
			Effects:        mustBuildJSON("effects "+place, tx.GetEffects()),
			Events:         mustBuildJSON("events "+place, tx.GetEvents()),
			BalanceChanges: mustBuildJSONArray("balance change "+place, tx.GetBalanceChanges()),
		}
		// index fields in tx.GetTransaction()
		txKind := tx.GetTransaction().GetKind().GetKind()
		if _, has := rpcv2.TransactionKind_Kind_name[int32(txKind)]; !has || txKind == rpcv2.TransactionKind_KIND_UNKNOWN {
			err = errors.Errorf("kind %d is not supported %s, may be need to upgrade proto", txKind, place)
			return
		}
		chtx.Kind = tx.GetTransaction().GetKind().GetKind().String()
		chtx.Sender = tx.GetTransaction().GetSender()
		for _, cmd := range tx.GetTransaction().GetKind().GetProgrammableTransaction().GetCommands() {
			if cmd.GetMoveCall() == nil {
				continue
			}
			chtx.MoveCallsPackage = append(chtx.MoveCallsPackage, cmd.GetMoveCall().GetPackage())
			chtx.MoveCallsModule = append(chtx.MoveCallsModule, cmd.GetMoveCall().GetModule())
			chtx.MoveCallsFunction = append(chtx.MoveCallsFunction, cmd.GetMoveCall().GetFunction())
		}
		// index fields in tx.GetEffects()
		chtx.Success = tx.GetEffects().GetStatus().GetSuccess()
		// index fields in tx.GetEvents()
		for ei, ev := range tx.GetEvents().GetEvents() {
			evTitle := fmt.Sprintf("%d/%d event %s", ei, len(tx.GetEvents().GetEvents()), place)
			evType := move.MustBuildType("type of "+evTitle, ev.GetEventType())

			chtx.EventsPackageID = append(chtx.EventsPackageID, ev.GetPackageId())
			chtx.EventsModule = append(chtx.EventsModule, ev.GetModule())
			chtx.EventsSender = append(chtx.EventsSender, ev.GetSender())
			chtx.EventsType = append(chtx.EventsType, evType.String())
			chtx.EventsMainType = append(chtx.EventsMainType, evType.Main())

			// === events
			events = append(events, Event{
				CheckpointIndex:  chtx.CheckpointIndex,
				TransactionIndex: chtx.TransactionIndex,
				EventIndex:       uint64(ei),
				PackageID:        ev.GetPackageId(),
				Module:           ev.GetModule(),
				Sender:           ev.GetSender(),
				EventType:        evType.String(),
				EventMainType:    evType.Main(),
				JSON:             mustBuildJSON("json of "+evTitle, ev.GetJson()),
			})
		}
		// index fields in tx.GetBalanceChanges()
		for bi, bc := range tx.GetBalanceChanges() {
			bcTitle := fmt.Sprintf("%d/%d balance change %s", bi, len(tx.GetBalanceChanges()), place)
			coinType := move.MustBuildType("coin type of "+bcTitle, bc.GetCoinType())
			amount, ok := new(big.Int).SetString(bc.GetAmount(), 10)
			if !ok {
				err = errors.Errorf("parse amount %q of %s failed", bc.GetAmount(), bcTitle)
				return
			}

			chtx.BalanceChangesAddress = append(chtx.BalanceChangesAddress, bc.GetAddress())
			chtx.BalanceChangesCoinType = append(chtx.BalanceChangesCoinType, coinType.String())

			// === balances
			balance := Balance{
				Checkpoint:       chtx.CheckpointIndex.Checkpoint,
				CheckpointDigest: chtx.CheckpointIndex.CheckpointDigest,
				Timestamp:        chtx.CheckpointIndex.Timestamp,
				Epoch:            chtx.CheckpointIndex.Epoch,
				TxIndex:          chtx.TransactionIndex.TxIndex,
				TxDigest:         chtx.TransactionIndex.TxDigest,
				Address:          bc.GetAddress(),
				CoinType:         coinType.String(),
				Amount:           amount,
			}
			balance.PreCheckpoint, balance.PreTxIndex, balance.PreTxDigest, balance.Balance, err = m.balanceCtrl.IncrBalance(
				bc.GetAddress(), coinType.String(), chtx.Checkpoint, chtx.TransactionIndex, amount)
			if err != nil {
				return
			}
			balances = append(balances, balance)
		}
		for _, co := range tx.GetEffects().GetChangedObjects() {
			// === objects
			changeType := sui.GetChangeType(co)
			obj := Object{
				Checkpoint:       chtx.CheckpointIndex.Checkpoint,
				CheckpointDigest: chtx.CheckpointIndex.CheckpointDigest,
				Timestamp:        chtx.CheckpointIndex.Timestamp,
				Epoch:            chtx.CheckpointIndex.Epoch,
				TransactionIndex: chtx.TransactionIndex,
				ChangeType:       string(changeType),
				ObjectID:         co.GetObjectId(),
				ObjectVersion:    co.GetOutputVersion(),
				ObjectDigest:     co.GetOutputDigest(),                   // will be empty if OutputState is DOES_NOT_EXIST
				OwnerKind:        co.GetOutputOwner().GetKind().String(), // will be empty if OutputState is DOES_NOT_EXIST
				OwnerAddress:     co.GetOutputOwner().GetAddress(),       // will be empty if OutputState is DOES_NOT_EXIST
				OwnerVersion:     co.GetOutputOwner().GetVersion(),       // will be empty if OutputState is DOES_NOT_EXIST
				PreObjectVersion: co.GetInputVersion(),                   // will be empty if InputState is DOES_NOT_EXIST
				PreDigest:        co.GetInputDigest(),                    // will be empty if InputState is DOES_NOT_EXIST
				PreOwnerKind:     co.GetInputOwner().GetKind().String(),  // will be empty if InputState is DOES_NOT_EXIST
				PreOwnerAddress:  co.GetInputOwner().GetAddress(),        // will be empty if InputState is DOES_NOT_EXIST
				PreOwnerVersion:  co.GetInputOwner().GetVersion(),        // will be empty if InputState is DOES_NOT_EXIST
				Package:          "null",
				JSON:             "null",
			}
			// if this is a delete version, co.OutputVersion may not exist,
			// we can use LamportVersion as the object version.
			// however, the early data will have co.OutputVersion but no LamportVersion.
			if obj.ObjectVersion == 0 {
				obj.ObjectVersion = tx.GetEffects().GetLamportVersion()
			}
			if obj.ObjectVersion == 0 {
				err = errors.Errorf("object %s no output version and also no lamport version %s", co.GetObjectId(), place)
				return
			}
			if changeType == types.ObjectChangeTypeUnknown ||
				changeType == types.ObjectChangeTypeAccumulatorWrite ||
				changeType == types.ObjectChangeTypeUnwrappedThenDeleted {
				// no more detail, that's it
				objects = append(objects, obj)
				continue
			}
			if !changeType.IsCreated() && co.GetInputOwner() == nil {
				// the early data may miss co.InputOwner, in this situation we need to get the pre-owner from the pre-object
				preFullObj, has := utils.GetFromK2Map(objectDict, co.GetObjectId(), co.GetInputVersion())
				if !has {
					err = errors.Errorf("object %s/%d not found in checkpoint objects", co.GetObjectId(), co.GetInputVersion())
					return
				}
				obj.PreOwnerKind = preFullObj.GetOwner().GetKind().String()
				obj.PreOwnerAddress = preFullObj.GetOwner().GetAddress()
				obj.PreOwnerVersion = preFullObj.GetOwner().GetVersion()
			}
			fullObjVersion := utils.Select(changeType.IsDeleted(), co.GetInputVersion(), obj.ObjectVersion)
			fullObj, has := utils.GetFromK2Map(objectDict, co.GetObjectId(), fullObjVersion)
			if !has {
				err = errors.Errorf("object %s/%d not found in checkpoint objects", co.GetObjectId(), fullObjVersion)
				return
			}
			objTitle := fmt.Sprintf("object %s/%d %s", obj.ObjectID, obj.ObjectVersion, place)
			objType := move.MustBuildType("type of "+objTitle, fullObj.GetObjectType())
			obj.ObjectType = objType.String()
			obj.ObjectMainType = objType.Main()
			if obj.ObjectMainType == "0x2::coin::Coin" {
				obj.CoinType = strings.TrimSuffix(strings.TrimPrefix(obj.ObjectType, "0x2::coin::Coin<"), ">")
			}
			obj.HasPublicTransfer = fullObj.GetHasPublicTransfer()
			obj.StorageRebate = fullObj.GetStorageRebate()
			obj.Package = mustBuildJSON("package of "+objTitle, fullObj.GetPackage())
			obj.Modules = utils.MapSliceNoError(fullObj.GetPackage().GetModules(), (*rpcv2.Module).GetName)
			obj.JSON = mustBuildJSON("json of "+objTitle, fullObj.GetJson())
			obj.Balance = fullObj.GetBalance()
			objects = append(objects, obj)
		}
		// --- new transaction
		txs = append(txs, chtx)
		checkpoint.EventCount += uint64(len(tx.GetEvents().GetEvents()))
	}
	return
}

func (m *ClickhouseSchemaMgr) Convert(ctx context.Context, slot *sui.Slot) (clickhouse.Chunk, error) {
	checkpoint, txs, events, objects, balances, err := m.convert(ctx, slot.GrpcCheckpoint)
	if err != nil {
		return clickhouse.Chunk{}, err
	}
	fieldFilter := objectx.HasTag("clickhouse")
	return clickhouse.Chunk{
		RowNum: []int{
			1,
			len(txs),
			len(events),
			len(objects),
			len(balances),
		},
		RowData: utils.MergeArr[[]any](
			[][]any{objectx.CollectFieldValues(checkpoint, fieldFilter)},
			utils.MapSliceNoError(txs, func(t Transaction) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(events, func(t Event) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(objects, func(t Object) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(balances, func(t Balance) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
		),
	}, nil
}

// ConvertConcurrency should always 1 because balance increase must be one by one.
// The cost of converting as expected is minimal and requires no concurrency.
func (m *ClickhouseSchemaMgr) ConvertConcurrency() uint {
	return 1
}

func (m *ClickhouseSchemaMgr) Done(r rg.Range) error {
	return m.balanceCtrl.Done(r)
}

func (m *ClickhouseSchemaMgr) Snapshot() any {
	return map[string]any{
		"tablesMeta":  m.tablesMeta,
		"balanceCtrl": m.balanceCtrl.Snapshot(),
	}
}
