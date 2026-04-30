package sui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/utils"
)

// TxSanityCheck will make sure decoded transaction is valid.
// Sui uses rust Enum extensively, which is difficult to capture or mimic in go.
// We want to make sure at least whenever there is a new Kind being added (to SingleTransactionKind or Event),
// we fail fast with a clear error message, rather than proceed with incorrect results, as rewinding already
// stored data (particularly in gcs) is often difficult if not impossible.
func TxSanityCheck(tx *types.TransactionResponseV1) error {
	if tx.Transaction.Data == nil {
		return fmt.Errorf("transaction data is nil (no transaction payload?)")
	}
	if tx.Transaction.Data.V1 == nil {
		return fmt.Errorf("transaction data v1 is nil (no transaction payload?)")
	}
	tx.Transaction.Intent = &types.EmptyIntentMessage
	encodedBCS, err := types.EncodeSenderSignedData(&types.SenderSignedData{
		Transactions: []types.SenderSignedTransaction{*tx.Transaction},
	})
	if err != nil {
		return errors.Wrap(err, "failed to encode transaction to bcs")
	}
	if !bytes.Equal(encodedBCS, tx.RawTransaction.Data()) {
		return fmt.Errorf(
			"transaction sanity check failed: encoded bcs doesn't match raw transaction %s",
			tx.Digest.String(),
		)
	}
	return nil
}

type MoveCallFilter struct {
	Package  *types.ObjectID `json:"package,omitempty"`
	Module   string          `json:"module,omitempty"`
	Function string          `json:"function,omitempty"`
}

const (
	EventFilterAnd = "And"
	EventFilterOr  = "Or"
)

type EventFilter struct {
	PackageID         *types.ObjectID `json:"packageId,omitempty"`
	TransactionModule string          `json:"transactionModule,omitempty"`
	Type              *types.TypeTag  `json:"type,omitempty"`
	Sender            string          `json:"sender,omitempty"`

	// And, Or
	Op    string       `json:"op,omitempty"`
	Left  *EventFilter `json:"left,omitempty"`
	Right *EventFilter `json:"right,omitempty"`
}

type BalanceChangeFilter struct {
	AddressOwner *types.Address `json:"addressOwner,omitempty"`
}

// TransactionQuery represents a query for transactions, which is part of sentio.xyz custom API for sui.
type TransactionQuery struct {
	// FromSequenceNumber and ToSequenceNumber must be specified.
	FromSequenceNumber uint64 `json:"fromSequenceNumber,omitempty"`
	ToSequenceNumber   uint64 `json:"toSequenceNumber,omitempty"`

	// If specified, only transactions of the kind will be returned.
	// If nil, all transactions will be returned.
	Kind string `json:"kind,omitempty"`
	// If Kind is `ProgrammableTransaction`, CallFilter can be optionally specified to further filter Move calls.
	MoveCallFilter *MoveCallFilter `json:"moveCall,omitempty"`
	// If specified, txSignature must be of type MultiSig and contains a public key that has the given prefix.
	// The param should be a hex string starting with 0x.
	MultiSigPublicKeyPrefix hexutil.Bytes `json:"multiSigPublicKeyPrefix,omitempty"`

	// If true, only events that match the filter will be returned.
	// Either case, should a transaction has no matching events, the entire transaction will be discarded.
	// EventFilter must not be nil if OnlyFilteredEvents is true.
	OnlyFilteredEvents bool         `json:"onlyFilteredEvents,omitempty"`
	EventFilter        *EventFilter `json:"eventFilter,omitempty"`

	// Control the needed fields, only event handler use these two options as true,
	// because filters of function handler need to execute after queried from chainquery
	ExcludeInputs  bool `json:"excludeInputs,omitempty"`
	ExcludeEffects bool `json:"excludeEffects,omitempty"`

	// If true, failed transactions will be included in the result.
	IncludeFailed bool `json:"includeFailed,omitempty"`

	// If specified, only transactions that have the given sender will be returned.
	Sender *types.Address `json:"sender,omitempty"`
	// If specified, only transactions that have the given receiver will be returned.
	BalanceChange *BalanceChangeFilter `json:"balanceChange,omitempty"`
}

func (q TransactionQuery) String() string {
	return utils.MustJSONMarshal(q)
}

type ObjectChangeQuery struct {
	FromSequenceNumber uint64 `json:"fromSequenceNumber,omitempty"`
	ToSequenceNumber   uint64 `json:"toSequenceNumber,omitempty"`

	OwnerType string `json:"ownerType,omitempty"`

	// conditions below are linked by OR
	OwnerIDIn    []string        `json:"ownerIDIn,omitempty"`
	ObjectIDIn   []string        `json:"objectIDIn,omitempty"`
	ObjectTypeIn []types.TypeTag `json:"objectTypeIn,omitempty"`

	// only last version is needed
	OnlyLastVersion bool `json:"onlyLastVersion,omitempty"`
}

func (q ObjectChangeQuery) String() string {
	b, _ := json.Marshal(q)
	return string(b)
}

func (q ObjectChangeQuery) Check(oc types.ObjectChangeExtend) bool {
	if ckpt := oc.Checkpoint.Uint64(); ckpt < q.FromSequenceNumber || ckpt > q.ToSequenceNumber {
		return false
	}
	ownerType, ownerID, _ := oc.Owner.GetTypeAndID()
	if q.OwnerType != "" && q.OwnerType != ownerType {
		return false
	}
	if len(q.OwnerIDIn) == 0 && len(q.ObjectIDIn) == 0 && len(q.ObjectTypeIn) == 0 {
		return true
	}
	if len(q.OwnerIDIn) > 0 && utils.IndexOf(q.OwnerIDIn, ownerID) >= 0 {
		return true
	}
	if len(q.ObjectIDIn) > 0 && utils.IndexOf(q.ObjectIDIn, oc.GetObjectID()) >= 0 {
		return true
	}
	if len(q.ObjectTypeIn) > 0 && types.AnyInclude(q.ObjectTypeIn, oc.ObjectType) {
		return true
	}
	return false
}

func (q ObjectChangeQuery) Filter(raw []types.ObjectChangeExtend) (result []types.ObjectChangeExtend) {
	return utils.FilterArr(raw, q.Check)
}

type CheckpointTime struct {
	CheckpointTime uint64
	MinTxnTime     uint64
	MaxTxnTime     uint64
}

type ObjectStat struct {
	Count            uint64
	MinObjectVersion uint64
	MinCheckpoint    uint64
	MaxObjectVersion uint64
	MaxCheckpoint    uint64
}

func (os ObjectStat) Merge(a ObjectStat) ObjectStat {
	if a.Count == 0 {
		return os
	}
	if os.Count == 0 {
		return a
	}
	return ObjectStat{
		Count:            os.Count + a.Count,
		MinObjectVersion: min(os.MinObjectVersion, a.MinObjectVersion),
		MaxObjectVersion: max(os.MaxObjectVersion, a.MaxObjectVersion),
		MinCheckpoint:    min(os.MinCheckpoint, a.MinCheckpoint),
		MaxCheckpoint:    max(os.MaxCheckpoint, a.MaxCheckpoint),
	}
}
