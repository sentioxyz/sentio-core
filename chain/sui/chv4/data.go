package chv4

import (
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math/big"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type CheckpointIndex struct {
	Checkpoint       uint64    `clickhouse:"checkpoint" number_field:"true"`
	CheckpointDigest string    `clickhouse:"checkpoint_digest"`
	Timestamp        time.Time `clickhouse:"timestamp"`
	Epoch            uint64    `clickhouse:"epoch"`
}

type TransactionIndex struct {
	TxIndex  uint64 `clickhouse:"tx_index"`
	TxDigest string `clickhouse:"tx_digest"`
}

type Checkpoint struct {
	CheckpointIndex

	Summary   string `clickhouse:"summary"   type:"JSON"`
	Signature string `clickhouse:"signature" type:"JSON"`
	Contents  string `clickhouse:"contents"  type:"JSON"`

	TransactionCount uint64 `clickhouse:"transaction_count" required:"false"`
	EventCount       uint64 `clickhouse:"event_count"       required:"false"`
}

type Transaction struct {
	CheckpointIndex
	TransactionIndex

	Signatures     []string `clickhouse:"signatures"      type:"Array(JSON)"`
	Transaction    string   `clickhouse:"transaction"     type:"JSON"`
	Effects        string   `clickhouse:"effects"         type:"JSON"`
	Events         string   `clickhouse:"events"          type:"JSON"`
	BalanceChanges []string `clickhouse:"balance_changes" type:"Array(JSON)"`

	Kind              string   `clickhouse:"kind"                required:"false" index:"set(0)"`
	Sender            string   `clickhouse:"sender"              required:"false" index:"bloom_filter"`
	MoveCallsPackage  []string `clickhouse:"move_calls_package"  required:"false" index:"bloom_filter"`
	MoveCallsModule   []string `clickhouse:"move_calls_module"   required:"false" index:"bloom_filter"`
	MoveCallsFunction []string `clickhouse:"move_calls_function" required:"false" index:"bloom_filter"`

	Success bool `clickhouse:"success" required:"false" index:"set(0)"`

	EventsPackageID []string `clickhouse:"events_package_id" required:"false" index:"bloom_filter"`
	EventsModule    []string `clickhouse:"events_module"     required:"false" index:"bloom_filter"`
	EventsSender    []string `clickhouse:"events_sender"     required:"false" index:"bloom_filter"`
	EventsType      []string `clickhouse:"events_type"       required:"false" index:"bloom_filter"`
	EventsMainType  []string `clickhouse:"events_main_type"  required:"false" index:"bloom_filter"`

	BalanceChangesAddress  []string `clickhouse:"balance_changes_address"   required:"false" index:"bloom_filter"`
	BalanceChangesCoinType []string `clickhouse:"balance_changes_coin_type" required:"false" index:"bloom_filter"`
}

// ToExecutedTransaction result will miss ExecutedTransaction.Objects
func (tx Transaction) ToExecutedTransaction() (*rpcv2.ExecutedTransaction, error) {
	var r rpcv2.ExecutedTransaction
	r.Digest = &tx.TxDigest
	r.Checkpoint = &tx.Checkpoint
	r.Timestamp = timestamppb.New(tx.Timestamp)
	r.Transaction = &rpcv2.Transaction{}
	if has, err := decodeJSON(tx.Transaction, r.Transaction); err != nil {
		return nil, errors.Wrapf(err, "unmarshal transaction part for tx %d/%d/%s failed",
			tx.Checkpoint, tx.TxIndex, tx.TxDigest)
	} else if !has {
		r.Transaction = nil
	}
	for i, raw := range tx.Signatures {
		sig := &rpcv2.UserSignature{}
		if has, err := decodeJSON(raw, sig); err != nil {
			return nil, errors.Wrapf(err, "unmarshal %d/%d signature for tx %d/%d/%s failed",
				i, len(tx.Signatures), tx.Checkpoint, tx.TxIndex, tx.TxDigest)
		} else if !has {
			sig = nil // unreachable
		}
		r.Signatures = append(r.Signatures, sig)
	}
	r.Effects = &rpcv2.TransactionEffects{}
	if has, err := decodeJSON(tx.Effects, r.Effects); err != nil {
		return nil, errors.Wrapf(err, "unmarshal effects part for tx %d/%d/%s failed",
			tx.Checkpoint, tx.TxIndex, tx.TxDigest)
	} else if !has {
		r.Effects = nil
	}
	r.Events = &rpcv2.TransactionEvents{}
	if has, err := decodeJSON(tx.Events, r.Events); err != nil {
		return nil, errors.Wrapf(err, "unmarshal events part for tx %d/%d/%s failed",
			tx.Checkpoint, tx.TxIndex, tx.TxDigest)
	} else if !has {
		r.Events = nil
	}
	for i, raw := range tx.BalanceChanges {
		bc := &rpcv2.BalanceChange{}
		if has, err := decodeJSON(raw, bc); err != nil {
			return nil, errors.Wrapf(err, "unmarshal %d/%d balance change for tx %d/%d/%s failed",
				i, len(tx.BalanceChanges), tx.Checkpoint, tx.TxIndex, tx.TxDigest)
		} else if !has {
			bc = nil // unreachable
		}
		r.BalanceChanges = append(r.BalanceChanges, bc)
	}
	return &r, nil
}

type Event struct {
	CheckpointIndex
	TransactionIndex
	EventIndex uint64 `clickhouse:"event_index"`

	PackageID     string `clickhouse:"package_id"      index:"bloom_filter"`
	Module        string `clickhouse:"module"          index:"bloom_filter"`
	Sender        string `clickhouse:"sender"          index:"bloom_filter"`
	EventType     string `clickhouse:"event_type"      index:"bloom_filter"`
	EventMainType string `clickhouse:"event_main_type" index:"bloom_filter"`
	JSON          string `clickhouse:"json" type:"JSON"`
}

// Object is a snapshot of the object.
// object owner and object type both can be changed
type Object struct {
	Checkpoint       uint64    `clickhouse:"checkpoint" number_field:"true" projection:"idv/3" `
	CheckpointDigest string    `clickhouse:"checkpoint_digest"`
	Timestamp        time.Time `clickhouse:"timestamp"`
	Epoch            uint64    `clickhouse:"epoch"`

	TransactionIndex

	// === basic info
	// ChangeType is the value of types.ObjectChangeType.
	// sui.ObjectChangeTypeUnwrappedThenDeleted is a special ChangeType, all properties expect ObjectID and
	// ObjectVersion will be empty.
	ChangeType    string `clickhouse:"change_type"    index:"set(0)"       projection:"idv/4"`
	ObjectID      string `clickhouse:"object_id"      index:"bloom_filter" projection:"idv/1"`
	ObjectVersion uint64 `clickhouse:"object_version" index:"minmax"       projection:"idv/2"`
	ObjectDigest  string `clickhouse:"object_digest"  index:"bloom_filter"` // will be empty for a deleted version
	OwnerKind     string `clickhouse:"owner_kind"     index:"bloom_filter"` // will be empty for a deleted version
	OwnerAddress  string `clickhouse:"owner_address"  index:"bloom_filter"` // will be empty for a deleted version
	OwnerVersion  uint64 `clickhouse:"owner_version"`                       // will be empty for a deleted version

	PreObjectVersion uint64 `clickhouse:"pre_object_version" index:"minmax"`       // will be empty for a created version
	PreDigest        string `clickhouse:"pre_digest"         index:"bloom_filter"` // will be empty for a created version
	PreOwnerKind     string `clickhouse:"pre_owner_kind"     index:"bloom_filter"` // will be empty for a created version
	PreOwnerAddress  string `clickhouse:"pre_owner_address"  index:"bloom_filter"` // will be empty for a created version
	PreOwnerVersion  uint64 `clickhouse:"pre_owner_version"`                       // will be empty for a created version

	// === type info, properties below will be a copy of the pre-version if it is a deleted versin
	ObjectType     string `clickhouse:"object_type"      index:"bloom_filter"`
	ObjectMainType string `clickhouse:"object_main_type" index:"bloom_filter"`

	// === detail info, properties below will be a copy of the pre-version if it is a deleted versin
	HasPublicTransfer bool     `clickhouse:"has_public_transfer"`
	StorageRebate     uint64   `clickhouse:"storage_rebate"`
	Package           string   `clickhouse:"package" type:"JSON"`
	Modules           []string `clickhouse:"modules"`
	JSON              string   `clickhouse:"json" type:"JSON"`
	CoinType          string   `clickhouse:"coin_type"`
	Balance           uint64   `clickhouse:"balance"`
}

func (obj Object) ToObjectChangeExtend() (r types.ObjectChangeExtend, err error) {
	r.Checkpoint = types.Uint64ToNumber(obj.Checkpoint)
	r.CheckpointDigest, err = types.StrToDigest(obj.CheckpointDigest)
	if err != nil {
		return r, errors.Wrapf(err, "invalid checkpoint digest %q", obj.CheckpointDigest)
	}
	r.TxIndex = int(obj.TxIndex)
	r.TxDigest, err = types.StrToDigest(obj.TxDigest)
	if err != nil {
		return r, errors.Wrapf(err, "invalid transaction digest %q", obj.TxDigest)
	}
	r.Type = types.ObjectChangeType(obj.ChangeType)
	r.Digest, err = types.StrToDigest(obj.ObjectDigest)
	if err != nil {
		return r, errors.Wrapf(err, "invalid digest %q", obj.ObjectDigest)
	}
	r.Version = types.Uint64ToNumber(obj.ObjectVersion)
	if obj.PreObjectVersion > 0 {
		r.PreviousVersion = utils.WrapPointer(types.Uint64ToNumber(obj.PreObjectVersion))
	}
	r.ObjectID = utils.WrapPointer(types.StrToObjectIDMust(obj.ObjectID))
	r.ObjectType, err = types.TypeTagFromString(obj.ObjectType)
	if err != nil {
		return r, errors.Wrapf(err, "invalid object type %q", obj.ObjectType)
	}
	if r.Type.IsDeleted() {
		r.Owner = types.BuildObjectOwner(
			obj.PreOwnerAddress,
			ownerKindToType(rpcv2.Owner_OwnerKind(rpcv2.Owner_OwnerKind_value[obj.PreOwnerKind])),
			obj.PreOwnerVersion,
		)
	} else {
		r.Owner = types.BuildObjectOwner(
			obj.OwnerAddress,
			ownerKindToType(rpcv2.Owner_OwnerKind(rpcv2.Owner_OwnerKind_value[obj.OwnerKind])),
			obj.OwnerVersion,
		)
	}
	r.Modules = obj.Modules
	if obj.ChangeType == types.ObjectChangeTypePublished {
		r.PackageID = r.ObjectID
	}
	// r.Sender && r.Recipient not set
	return r, nil
}

type Balance struct {
	Checkpoint       uint64    `clickhouse:"checkpoint" number_field:"true" projection:"holder/3"`
	CheckpointDigest string    `clickhouse:"checkpoint_digest"`
	Timestamp        time.Time `clickhouse:"timestamp"`
	Epoch            uint64    `clickhouse:"epoch"`

	TxIndex  uint64 `clickhouse:"tx_index"  projection:"holder/4"`
	TxDigest string `clickhouse:"tx_digest" projection:"holder/5"`

	Address  string   `clickhouse:"address"                        projection:"holder/1"`
	CoinType string   `clickhouse:"coin_type" index:"bloom_filter" projection:"holder/2"`
	Amount   *big.Int `clickhouse:"amount"`

	PreCheckpoint uint64   `clickhouse:"pre_checkpoint"`
	PreTxIndex    uint64   `clickhouse:"pre_tx_index"`
	PreTxDigest   string   `clickhouse:"pre_tx_digest"` // empty means no pre-transaction
	Balance       *big.Int `clickhouse:"balance" projection:"holder/6"`
}
