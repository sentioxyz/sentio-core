package chv3

import (
	_ "embed"
	"encoding/json"
	"github.com/pkg/errors"
	"math/big"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/objectx"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type CHUTransactionBasePart struct {
	// ========================================
	// SlotCheckpointInfo
	Checkpoint            uint64    `clickhouse:"checkpoint"              number_field:"true"`
	CheckpointDigest      string    `clickhouse:"checkpoint_digest"       index:"bloom_filter"`
	CheckpointTimestampMs uint64    `clickhouse:"checkpoint_timestamp_ms" index:"minmax GRANULARITY 3"`
	CheckpointTimestamp   time.Time `clickhouse:"checkpoint_timestamp"    index:"minmax GRANULARITY 3"`
	TransactionPosition   int32     `clickhouse:"transaction_position"`

	// ========================================
	// types.TransactionResponseV1
	TimestampMs    uint64    `clickhouse:"timestamp_ms"`
	Timestamp      time.Time `clickhouse:"timestamp"`
	Digest         string    `clickhouse:"digest"          index:"bloom_filter"`
	RawTransaction string    `clickhouse:"raw_transaction" compression:"CODEC(ZSTD(1))"`
	Errors         []string  `clickhouse:"errors"`

	// ========================================
}

type CHUTransactionEffectPart struct {
	EffectsJSON                    string   `clickhouse:"effects_json" compression:"CODEC(ZSTD(1))" required:"true"`
	EffectMessageVersion           string   `clickhouse:"effects_message_version"`
	Epoch                          uint64   `clickhouse:"epoch"`
	ModifiedAtVersions             []string `clickhouse:"modified_at_versions" compression:"CODEC(ZSTD(1))"`
	EventsDigest                   string   `clickhouse:"events_digest"`
	Status                         string   `clickhouse:"status"`
	Error                          string   `clickhouse:"error"`
	GasUsedComputationCost         uint64   `clickhouse:"gas_used_computation_cost"`
	GasUsedStorageCost             uint64   `clickhouse:"gas_used_storage_cost"`
	GasUsedStorageRebate           uint64   `clickhouse:"gas_used_storage_rebate"`
	GasUsedNonRefundableStorageFee uint64   `clickhouse:"gas_used_non_refundable_storage_fee"`
	CreatedCount                   uint32   `clickhouse:"created_count"`
	MutatedCount                   uint32   `clickhouse:"mutated_count"`
	DeletedCount                   uint32   `clickhouse:"deleted_count"`
	WrappedCount                   uint32   `clickhouse:"wrapped_count"`
	UnwrappedThenDeletedCount      uint32   `clickhouse:"unwrapped_then_deleted_count"`
}

type CHUTransactionBalancePart struct {
	BalanceChangesOwner    []string `clickhouse:"balance_changes.owner"`
	BalanceChangesCoinType []string `clickhouse:"balance_changes.coin_type"`
	BalanceChangesAmount   []string `clickhouse:"balance_changes.amount"`
}

type CHUTransactionEventPart struct {
	EventsTxDigest          []string `clickhouse:"events.tx_digest"`
	EventsEventSeq          []uint64 `clickhouse:"events.event_seq"`
	EventsPackageID         []string `clickhouse:"events.package_id"         index:"bloom_filter"`
	EventsTransactionModule []string `clickhouse:"events.transaction_module" index:"bloom_filter"`
	EventsSender            []string `clickhouse:"events.sender"`
	EventsType              []string `clickhouse:"events.type"               index:"bloom_filter"`
	EventsRawType           []string `clickhouse:"events.raw_type"           index:"bloom_filter"`
	EventsFields            []string `clickhouse:"events.fields"`
}

type CHUTransactionInputPart struct {
	TransactionJSON string `clickhouse:"transaction_json" compression:"CODEC(ZSTD(1))" required:"true"`
	IsSystemTx      uint8  `clickhouse:"is_system_tx"    index:"set(8) GRANULARITY 3"`
	IsSponsoredTx   uint8  `clickhouse:"is_sponsored_tx" index:"set(8) GRANULARITY 3"`
	// Transaction.TxSignatures
	TxSignature        []string `clickhouse:"tx_signature"`
	HasZkloginSig      uint8    `clickhouse:"has_zklogin_sig"      index:"set(8) GRANULARITY 3"`
	HasUpgradeMultisig uint8    `clickhouse:"has_upgrade_multisig" index:"set(8) GRANULARITY 3"`
	// Transaction.Data.V1
	MessageVersion string `clickhouse:"message_version"`
	// Transaction.Data.V1.Sender
	Sender string `clickhouse:"sender" index:"bloom_filter"`
	// Transaction.Data.V1.Expiration
	ExpirationEpoch *uint64 `clickhouse:"expiration_epoch"`
	// Transaction.Data.V1.GasData
	GasOwner           string   `clickhouse:"gas_owner"`
	GasPrice           uint64   `clickhouse:"gas_price" index:"minmax GRANULARITY 3"`
	GasBudget          uint64   `clickhouse:"gas_budget"`
	GasObjectsID       []string `clickhouse:"gas_objects.id"`
	GasObjectsSequence []uint64 `clickhouse:"gas_objects.sequence"`
	GasObjectsDigest   []string `clickhouse:"gas_objects.digest"`
	// Transaction.Data.V1.Kind
	Kind string `clickhouse:"kind" index:"set(0)"`
	// Transaction.Data.V1.Kind.ProgrammableTransaction
	TransactionCount uint32 `clickhouse:"transaction_count"   index:"minmax GRANULARITY 3"`
	InputCount       uint32 `clickhouse:"input_count"         index:"minmax GRANULARITY 3"`
	SharedInputCount uint32 `clickhouse:"shared_input_count"  index:"minmax GRANULARITY 3"`
	GasCoinsCount    uint32 `clickhouse:"gas_coins_count"     index:"minmax GRANULARITY 3"`
	TransfersCount   uint32 `clickhouse:"transfers_count"     index:"minmax GRANULARITY 3"`
	SplitCoinsCount  uint32 `clickhouse:"split_coins_count"   index:"minmax GRANULARITY 3"`
	MergedCoinsCount uint32 `clickhouse:"merged_coins_count"  index:"minmax GRANULARITY 3"`
	PublishCount     uint32 `clickhouse:"publish_count"       index:"minmax GRANULARITY 3"`
	UpgradeCount     uint32 `clickhouse:"upgrade_count"       index:"minmax GRANULARITY 3"`
	MoveCallsCount   uint32 `clickhouse:"move_calls_count"    index:"minmax GRANULARITY 3"`
	MakeMoveVecCount uint32 `clickhouse:"make_move_vec_count" index:"minmax GRANULARITY 3"`
	// Transaction.Data.V1.Kind.ProgrammableTransaction
	MoveCallsPackage  []string `clickhouse:"move_calls.package"`
	MoveCallsModule   []string `clickhouse:"move_calls.module"`
	MoveCallsFunction []string `clickhouse:"move_calls.function"`
}

type CHUTransaction struct {
	CHUTransactionBasePart `required:"true"`
	CHUTransactionEffectPart
	CHUTransactionEventPart   `required:"true"`
	CHUTransactionBalancePart `required:"true"`
	CHUTransactionInputPart
}

type CHUTxnExtendBase struct {
	Digest           string    `clickhouse:"digest"            index:"bloom_filter"`
	Checkpoint       uint64    `clickhouse:"checkpoint"        number_field:"true"`
	CheckpointDigest string    `clickhouse:"checkpoint_digest" index:"bloom_filter"`
	Epoch            uint64    `clickhouse:"epoch"`
	TimestampMs      uint64    `clickhouse:"timestamp_ms"      index:"minmax"`
	Timestamp        time.Time `clickhouse:"timestamp"         index:"minmax"`
}

type CHUMoveCall struct {
	CHUTxnExtendBase
	Package  string `clickhouse:"package"  index:"bloom_filter"`
	Module   string `clickhouse:"module"   index:"bloom_filter"`
	Function string `clickhouse:"function" index:"bloom_filter"`
}

type CHUEvent struct {
	CHUTxnExtendBase
	EventSeq  uint64 `clickhouse:"event_seq"  index:"minmax"`
	PackageID string `clickhouse:"package_id" index:"bloom_filter"`
	Module    string `clickhouse:"module"     index:"bloom_filter"`
	Sender    string `clickhouse:"sender"     index:"bloom_filter"`
	Type      string `clickhouse:"type"       index:"bloom_filter"`
	RawType   string `clickhouse:"raw_type"   index:"bloom_filter"`
	Fields    string `clickhouse:"fields"`
}

type CHUBalanceChange struct {
	CHUTxnExtendBase
	Owner        string   `clickhouse:"owner"         index:"bloom_filter"`
	OwnerAddress string   `clickhouse:"owner_address" index:"bloom_filter"`
	CoinType     string   `clickhouse:"coin_type"     index:"bloom_filter"`
	Amount       string   `clickhouse:"amount"`
	AmountNumber *big.Int `clickhouse:"amount_number" index:"minmax GRANULARITY 3"`
}

type CHUObjectChange struct {
	CHUTxnExtendBase          `required:"true"`
	Type                      string   `clickhouse:"type"            index:"set(0)"               required:"true"`
	ObjectID                  string   `clickhouse:"object_id"       index:"bloom_filter"         required:"true"`
	ObjectVersion             uint64   `clickhouse:"object_version"  index:"minmax GRANULARITY 3" required:"true"`
	ObjectPreviousVersion     *uint64  `clickhouse:"object_previous_version"                      required:"true"`
	ObjectDigest              string   `clickhouse:"object_digest"   index:"bloom_filter"         required:"true"`
	ObjectType                *string  `clickhouse:"object_type"     index:"bloom_filter"         required:"true"`
	ObjectRawType             *string  `clickhouse:"object_raw_type" index:"bloom_filter"`
	Sender                    *string  `clickhouse:"sender"                                       required:"true"`
	OwnerID                   string   `clickhouse:"owner_id"        index:"bloom_filter"`
	OwnerInitialSharedVersion uint64   `clickhouse:"owner_initial_shared_version"`
	OwnerType                 string   `clickhouse:"owner_type"      index:"set(0)"`
	Owner                     *string  `clickhouse:"owner"                                        required:"true"`
	Recipient                 *string  `clickhouse:"recipient"                                    required:"true"`
	Modules                   []string `clickhouse:"modules"                                      required:"true"`

	// properties from object detail for
	HasPublicTransfer bool   `clickhouse:"has_public_transfer"`
	CoinType          string `clickhouse:"coin_type"       index:"bloom_filter"`
	CoinBalance       uint64 `clickhouse:"coin_balance"    index:"minmax GRANULARITY 3"`
	StorageRebate     uint64 `clickhouse:"storage_rebate"`
}

type CHUObjectPosition struct {
	ObjectID      string `clickhouse:"object_id"      index:"bloom_filter"`
	ObjectVersion uint64 `clickhouse:"object_version" index:"minmax GRANULARITY 3"`
	Checkpoint    uint64 `clickhouse:"checkpoint"     index:"minmax GRANULARITY 3"`
}

func (c *CHUObjectPosition) PartitionBy() string {
	// only 256 partition in total
	return "substring(object_id, 1, 3)"
}

type CHUCheckpoint struct {
	Transactions   []CHUTransaction
	Events         []CHUEvent
	MoveCalls      []CHUMoveCall
	BalanceChanges []CHUBalanceChange
	ObjectChanges  []CHUObjectChange
	// special partition data
	ObjectPositions []CHUObjectPosition
}

func (u *CHUCheckpoint) Values() clickhouse.Chunk {
	fieldFilter := objectx.HasTag("clickhouse")
	return clickhouse.Chunk{
		RowNum: []int{
			len(u.Transactions),
			len(u.Events),
			len(u.MoveCalls),
			len(u.BalanceChanges),
			len(u.ObjectChanges),
			len(u.ObjectPositions),
		},
		RowData: utils.MergeArr[[]any](
			utils.MapSliceNoError(u.Transactions, func(t CHUTransaction) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(u.Events, func(t CHUEvent) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(u.MoveCalls, func(t CHUMoveCall) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(u.BalanceChanges, func(t CHUBalanceChange) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(u.ObjectChanges, func(t CHUObjectChange) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
			utils.MapSliceNoError(u.ObjectPositions, func(t CHUObjectPosition) []any {
				return objectx.CollectFieldValues(&t, fieldFilter)
			}),
		),
	}
}

func objectOwnerString(o types.ObjectOwner) string {
	b, _ := o.MarshalJSON()
	return string(b)
}

func strToObjectOwner(s string) (types.ObjectOwner, error) {
	var o types.ObjectOwner
	err := o.UnmarshalJSON([]byte(s))
	return o, err
}

func strToObjectOwnerMust(s string) types.ObjectOwner {
	o, err := strToObjectOwner(s)
	if err != nil {
		panic(err)
	}
	return o
}

func objectOwnerAddress(o *types.ObjectOwner) string {
	if o == nil || o.ObjectOwnerInternal == nil {
		return ""
	}
	if o.ObjectOwnerInternal.ObjectOwner != nil {
		return o.ObjectOwnerInternal.ObjectOwner.String()
	}
	if o.ObjectOwnerInternal.AddressOwner != nil {
		return o.ObjectOwnerInternal.AddressOwner.String()
	}
	if o.ObjectOwnerInternal.SingleOwner != nil {
		return o.ObjectOwnerInternal.SingleOwner.String()
	}
	return ""
}

func (txn CHUTransaction) BuildTransactionResponseV1() types.TransactionResponseV1 {
	r := types.TransactionResponseV1{
		CheckpointStub: types.CheckpointStub{
			Checkpoint:            types.Uint64ToNumber(txn.Checkpoint),
			CheckpointTimestampMs: utils.WrapPointer(types.Uint64ToNumber(txn.CheckpointTimestampMs)),
			TransactionPosition:   int(txn.TransactionPosition),
		},
		Digest:         types.StrToDigestMust(txn.Digest),
		RawTransaction: []byte(txn.RawTransaction),
		Errors:         txn.Errors,
		TimestampMs:    types.Uint64ToNumber(txn.TimestampMs),
	}
	if txn.TransactionJSON != "" {
		err := json.Unmarshal([]byte(txn.TransactionJSON), &r.Transaction)
		if err != nil {
			panic(errors.Wrapf(err, "unmarshal txn.TransactionJSON with digest %s failed", txn.Digest))
		}
	}
	if txn.EffectsJSON != "" {
		err := json.Unmarshal([]byte(txn.EffectsJSON), &r.Effects)
		if err != nil {
			panic(errors.Wrapf(err, "unmarshal txn.EffectsJSON with digest %s failed", txn.Digest))
		}
	}

	for i := range txn.CHUTransactionEventPart.EventsTxDigest {
		r.Events = append(r.Events, types.Event{
			ID: types.EventID{
				TxDigest: types.StrToDigestMust(txn.CHUTransactionEventPart.EventsTxDigest[i]),
				EventSeq: types.Uint64ToNumber(txn.CHUTransactionEventPart.EventsEventSeq[i]),
			},
			PackageID:         types.StrToObjectIDMust(txn.CHUTransactionEventPart.EventsPackageID[i]),
			TransactionModule: txn.CHUTransactionEventPart.EventsTransactionModule[i],
			Sender:            txn.CHUTransactionEventPart.EventsSender[i],
			Type:              types.TypeTagFromStringMust(txn.CHUTransactionEventPart.EventsType[i]),
			Fields:            json.RawMessage(txn.CHUTransactionEventPart.EventsFields[i]),
			BCS:               "",
		})
	}

	for i := range txn.CHUTransactionBalancePart.BalanceChangesOwner {
		r.BalanceChanges = append(r.BalanceChanges, types.BalanceChange{
			Owner:    utils.WrapPointer(strToObjectOwnerMust(txn.CHUTransactionBalancePart.BalanceChangesOwner[i])),
			CoinType: utils.WrapPointer(types.TypeTagFromStringMust(txn.CHUTransactionBalancePart.BalanceChangesCoinType[i])),
			Amount:   types.StringToNumber(txn.CHUTransactionBalancePart.BalanceChangesAmount[i]),
		})
	}

	// missing:
	// - r.Events[*].BCS
	// - r.ObjectChanges
	return r
}

func (oc CHUObjectChange) BuildObjectChangeExtend() (oce types.ObjectChangeExtend) {
	oce.Checkpoint = types.Uint64ToNumber(oc.Checkpoint)
	oce.CheckpointDigest = types.StrToDigestMust(oc.CheckpointDigest)
	oce.TxIndex = -1 // no transaction index information
	oce.TxDigest = types.StrToDigestMust(oc.Digest)
	oce.Type = types.ObjectChangeType(oc.Type)
	oce.Digest = types.StrToDigestMust(oc.ObjectDigest)
	oce.Version = types.Uint64ToNumber(oc.ObjectVersion)
	oce.PreviousVersion = utils.NullOrConvert(oc.ObjectPreviousVersion, types.Uint64ToNumber)
	oce.Sender = utils.NullOrFromString(oc.Sender, types.StrToAddressMust)
	oce.ObjectID = utils.WrapPointer(types.StrToObjectIDMust(oc.ObjectID))
	oce.ObjectType = utils.NullOrConvert(oc.ObjectType, types.TypeTagFromStringMust)
	oce.Recipient = utils.NullOrConvert(oc.Recipient, strToObjectOwnerMust)
	oce.Owner = utils.NullOrConvert(oc.Owner, strToObjectOwnerMust)
	oce.Modules = oc.Modules
	oce.PackageID = utils.Select(oce.Type == types.ObjectChangeTypePublished, oce.ObjectID, nil)
	return oce
}
