package sui

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"strings"

	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/tidwall/sjson"
)

// ObjectChangeOwnerFilter need the objects which id in OwnerID
// and owned by the objects in OwnerID with owner type in OwnerType
type ObjectChangeOwnerFilter struct {
	OwnerID   []string `json:"owner_id"`             // empty means need nothing
	OwnerType []string `json:"owner_type,omitempty"` // empty means all owned object are not needed
}

func (f ObjectChangeOwnerFilter) Merge(a ObjectChangeOwnerFilter) (r ObjectChangeOwnerFilter) {
	r.OwnerID = set.SmartNew[string](f.OwnerID, a.OwnerID).DumpValues()
	r.OwnerType = set.SmartNew[string](f.OwnerType, a.OwnerType).DumpValues()
	return r
}

func (f ObjectChangeOwnerFilter) Checker() func(oc types.ObjectChangeExtend) bool {
	ownerIDSet := set.New[string](f.OwnerID...)
	ownerTypeSet := set.New[string](f.OwnerType...)
	return func(oc types.ObjectChangeExtend) bool {
		if ownerIDSet.Empty() {
			return false
		}
		if ownerIDSet.Contains(oc.GetObjectID()) {
			return true
		}
		ownerType, ownerID, _ := oc.Owner.GetTypeAndID()
		return ownerIDSet.Contains(ownerID) && ownerTypeSet.Contains(ownerType)
	}
}

func (f ObjectChangeOwnerFilter) CheckerGrpc() func(oc *rpcv2.ChangedObject) bool {
	ownerIDSet := set.New[string](f.OwnerID...)
	ownerTypeSet := set.New[string](f.OwnerType...)
	return func(oc *rpcv2.ChangedObject) bool {
		if ownerIDSet.Empty() {
			return false
		}
		if ownerIDSet.Contains(oc.GetObjectId()) {
			return true
		}
		if owner := oc.GetInputOwner(); owner != nil {
			if ownerIDSet.Contains(owner.GetAddress()) && ownerTypeSet.Contains(owner.GetKind().String()) {
				return true
			}
		}
		if owner := oc.GetOutputOwner(); owner != nil {
			if ownerIDSet.Contains(owner.GetAddress()) && ownerTypeSet.Contains(owner.GetKind().String()) {
				return true
			}
		}
		return false
	}
}

// ObjectChangeFilter has 3 parts, linked by OR
type ObjectChangeFilter struct {
	TypePattern move.TypeSet             // empty means no object type condition
	OwnerFilter *ObjectChangeOwnerFilter // nil means no owner condition
	ObjectIDIn  set.Set[string]          // empty means no object id condition
}

func (f *ObjectChangeFilter) UnmarshalJSON(data []byte) error {
	type payload struct {
		TypePattern move.TypeSet             `json:"type_pattern,omitempty"`
		OwnerFilter *ObjectChangeOwnerFilter `json:"owner_filter,omitempty"`
		ObjectIDIn  []string                 `json:"object_id_in,omitempty"`
	}
	var p payload
	err := json.Unmarshal(data, &p)
	if err != nil {
		return err
	}
	f.TypePattern = p.TypePattern
	f.OwnerFilter = p.OwnerFilter
	f.ObjectIDIn = set.New(p.ObjectIDIn...)
	return nil
}

func (f ObjectChangeFilter) MarshalJSON() ([]byte, error) {
	payload := struct {
		TypePattern move.TypeSet             `json:"type_pattern,omitempty"`
		OwnerFilter *ObjectChangeOwnerFilter `json:"owner_filter,omitempty"`
		ObjectIDIn  []string                 `json:"object_id_in,omitempty"`
	}{
		TypePattern: f.TypePattern,
		OwnerFilter: f.OwnerFilter,
	}
	if f.ObjectIDIn != nil {
		payload.ObjectIDIn = f.ObjectIDIn.DumpValues()
	}
	return json.Marshal(payload)
}

func (f ObjectChangeFilter) IsEmpty() bool {
	return len(f.TypePattern) == 0 && f.OwnerFilter == nil && (f.ObjectIDIn == nil || f.ObjectIDIn.Empty())
}

func (f ObjectChangeFilter) Checker() func(oc types.ObjectChangeExtend) bool {
	var ownerChecker func(oc types.ObjectChangeExtend) bool
	if f.OwnerFilter != nil {
		ownerChecker = f.OwnerFilter.Checker()
	}
	return func(oc types.ObjectChangeExtend) bool {
		if len(f.TypePattern) > 0 {
			if oc.ObjectType == nil {
				if f.TypePattern.IncludeTypeString(nil) {
					return true
				}
			} else {
				ot, err := move.BuildType(oc.ObjectType.String())
				if err == nil && f.TypePattern.Include(ot) {
					return true
				}
			}
		}
		if ownerChecker != nil && ownerChecker(oc) {
			return true
		}
		if f.ObjectIDIn != nil && !f.ObjectIDIn.Empty() && f.ObjectIDIn.Contains(oc.GetObjectID()) {
			return true
		}
		return false
	}
}

func (f ObjectChangeFilter) CheckerGrpc() func(oc *rpcv2.ChangedObject) bool {
	var ownerChecker func(oc *rpcv2.ChangedObject) bool
	if f.OwnerFilter != nil {
		ownerChecker = f.OwnerFilter.CheckerGrpc()
	}
	return func(oc *rpcv2.ChangedObject) bool {
		if len(f.TypePattern) > 0 && f.TypePattern.IncludeTypeString(oc.ObjectType) {
			return true
		}
		if ownerChecker != nil && ownerChecker(oc) {
			return true
		}
		if f.ObjectIDIn != nil && !f.ObjectIDIn.Empty() && f.ObjectIDIn.Contains(oc.GetObjectId()) {
			return true
		}
		return false
	}
}

func (f ObjectChangeFilter) Merge(a ObjectChangeFilter) (r ObjectChangeFilter) {
	// r.TypePattern
	r.TypePattern = f.TypePattern.Merge(a.TypePattern)
	// r.OwnerFilter
	if f.OwnerFilter == nil {
		r.OwnerFilter = a.OwnerFilter
	} else if a.OwnerFilter == nil {
		r.OwnerFilter = f.OwnerFilter
	} else {
		r.OwnerFilter = utils.WrapPointer(f.OwnerFilter.Merge(*a.OwnerFilter))
	}
	// r.ObjectIDIn
	if f.ObjectIDIn == nil || f.ObjectIDIn.Empty() {
		r.ObjectIDIn = a.ObjectIDIn
	} else if a.ObjectIDIn == nil || a.ObjectIDIn.Empty() {
		r.ObjectIDIn = f.ObjectIDIn
	} else {
		r.ObjectIDIn = set.SmartNew[string](f.ObjectIDIn.DumpValues(), a.ObjectIDIn.DumpValues())
	}
	return r
}

type ExtendedGrpcTransaction struct {
	Checkpoint       uint64
	CheckpointDigest string
	TimestampMs      uint64
	Epoch            uint64

	TxIndex uint64

	// EventIndexes, when non-nil, holds the original (full-list) index of each
	// event currently in ExecutedTransaction.Events, i.e. EventIndexes[i] is the
	// on-chain position of Events[i]. It is populated by PruneGrpcTransaction when
	// events are filtered, so consumers can recover the true event index even
	// though grpc events carry no intrinsic sequence (json-rpc events have
	// id.eventSeq; grpc events do not). It is carried through the JSON wire form
	// because the super node prunes before serializing the transaction back to the
	// driver. nil means the events are the full, unfiltered list, in which case the
	// slice position already is the on-chain index.
	EventIndexes []int

	*rpcv2.ExecutedTransaction
}

type ExtendedGrpcChangedObject struct {
	Checkpoint       uint64
	CheckpointDigest string
	TimestampMs      uint64
	Epoch            uint64

	TxIndex  uint64
	TxDigest string

	*rpcv2.ChangedObject
}

// ExtendedGrpcTransaction / ExtendedGrpcChangedObject each embed a grpc proto
// message (*rpcv2.ExecutedTransaction / *rpcv2.ChangedObject). The default
// encoding/json would promote and serialize that embedded message with Go's
// reflection rules, which is wrong for protobuf (enums become numbers instead of
// their string names, oneofs / well-known types / 64-bit ints are mis-encoded).
// So we marshal the proto member through protojson (enum-as-string etc.).
//
// Both wrappers are FLATTENED: the embedded proto message's protojson fields
// (e.g. ExecutedTransaction's digest/transaction/events, ChangedObject's
// objectId/objectType/...) sit at the top level so SDK consumers can read them
// directly as the corresponding sui type. Each wrapper's own header fields are
// emitted under ext*-prefixed keys so they can never collide with proto field
// names (notably the proto's own "checkpoint").

func (t ExtendedGrpcTransaction) MarshalJSON() ([]byte, error) {
	obj := []byte("{}")
	if t.ExecutedTransaction != nil {
		b, err := protojson.Marshal(t.ExecutedTransaction)
		if err != nil {
			return nil, errors.Wrap(err, "marshal grpc ExecutedTransaction")
		}
		obj = b
	}
	var err error
	if obj, err = sjson.SetBytes(obj, "extCheckpoint", t.Checkpoint); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extCheckpointDigest", t.CheckpointDigest); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extTimestampMs", t.TimestampMs); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extEpoch", t.Epoch); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extTxIndex", t.TxIndex); err != nil {
		return nil, err
	}
	// Only emitted when events were filtered; nil (full list) is left off the wire.
	if len(t.EventIndexes) > 0 {
		if obj, err = sjson.SetBytes(obj, "extEventIndexes", t.EventIndexes); err != nil {
			return nil, err
		}
	}
	return obj, nil
}

func (t *ExtendedGrpcTransaction) UnmarshalJSON(data []byte) error {
	var aux struct {
		Checkpoint       uint64 `json:"extCheckpoint"`
		CheckpointDigest string `json:"extCheckpointDigest"`
		TimestampMs      uint64 `json:"extTimestampMs"`
		Epoch            uint64 `json:"extEpoch"`
		TxIndex          uint64 `json:"extTxIndex"`
		EventIndexes     []int  `json:"extEventIndexes,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	t.Checkpoint = aux.Checkpoint
	t.CheckpointDigest = aux.CheckpointDigest
	t.TimestampMs = aux.TimestampMs
	t.Epoch = aux.Epoch
	t.TxIndex = aux.TxIndex
	t.EventIndexes = aux.EventIndexes

	// The remaining (flattened) fields belong to the embedded ExecutedTransaction.
	// protojson here discards unknown fields, so the ext* header keys are ignored.
	msg := &rpcv2.ExecutedTransaction{}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return errors.Wrap(err, "unmarshal grpc ExecutedTransaction")
	}
	t.ExecutedTransaction = msg
	return nil
}

func (t ExtendedGrpcTransaction) GetSimpleCheckpoint() SimpleCheckpoint {
	return SimpleCheckpoint{
		Checkpoint:  t.Checkpoint,
		Digest:      t.CheckpointDigest,
		TimestampMS: t.TimestampMs,
	}
}

func (o ExtendedGrpcChangedObject) MarshalJSON() ([]byte, error) {
	obj := []byte("{}")
	if o.ChangedObject != nil {
		b, err := protojson.Marshal(o.ChangedObject)
		if err != nil {
			return nil, errors.Wrap(err, "marshal grpc ChangedObject")
		}
		obj = b
	}
	var err error
	if obj, err = sjson.SetBytes(obj, "extCheckpoint", o.Checkpoint); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extCheckpointDigest", o.CheckpointDigest); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extTimestampMs", o.TimestampMs); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extEpoch", o.Epoch); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extTxIndex", o.TxIndex); err != nil {
		return nil, err
	}
	if obj, err = sjson.SetBytes(obj, "extTxDigest", o.TxDigest); err != nil {
		return nil, err
	}
	return obj, nil
}

func (o *ExtendedGrpcChangedObject) UnmarshalJSON(data []byte) error {
	var aux struct {
		Checkpoint       uint64 `json:"extCheckpoint"`
		CheckpointDigest string `json:"extCheckpointDigest"`
		TimestampMs      uint64 `json:"extTimestampMs"`
		Epoch            uint64 `json:"extEpoch"`
		TxIndex          uint64 `json:"extTxIndex"`
		TxDigest         string `json:"extTxDigest"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	o.Checkpoint = aux.Checkpoint
	o.CheckpointDigest = aux.CheckpointDigest
	o.TimestampMs = aux.TimestampMs
	o.Epoch = aux.Epoch
	o.TxIndex = aux.TxIndex
	o.TxDigest = aux.TxDigest

	// The remaining (flattened) fields belong to the embedded ChangedObject.
	// protojson here discards unknown fields, so the ext* header keys are ignored.
	msg := &rpcv2.ChangedObject{}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return errors.Wrap(err, "unmarshal grpc ChangedObject")
	}
	o.ChangedObject = msg
	return nil
}

func (t ExtendedGrpcChangedObject) GetSimpleCheckpoint() SimpleCheckpoint {
	return SimpleCheckpoint{
		Checkpoint:  t.Checkpoint,
		Digest:      t.CheckpointDigest,
		TimestampMS: t.TimestampMs,
	}
}

// GrpcObjectResult wraps *rpcv2.GetObjectResult so it can cross the JSON-RPC
// boundary between the super node and the driver. GetObjectResult.Result is a
// protobuf oneof (Object | Error) and the default encoding/json cannot round-trip
// it — the driver side fails with "cannot unmarshal object into Go struct field
// GetObjectResult.Result of type rpcv2.isGetObjectResult_Result". As with
// ExtendedGrpcTransaction / ExtendedGrpcChangedObject, we route the proto member
// through protojson instead.
type GrpcObjectResult struct {
	*rpcv2.GetObjectResult
}

func (r GrpcObjectResult) MarshalJSON() ([]byte, error) {
	if r.GetObjectResult == nil {
		return []byte("null"), nil
	}
	b, err := protojson.Marshal(r.GetObjectResult)
	if err != nil {
		return nil, errors.Wrap(err, "marshal grpc GetObjectResult")
	}
	return b, nil
}

func (r *GrpcObjectResult) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		r.GetObjectResult = nil
		return nil
	}
	msg := &rpcv2.GetObjectResult{}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return errors.Wrap(err, "unmarshal grpc GetObjectResult")
	}
	r.GetObjectResult = msg
	return nil
}

// WrapGrpcObjectResults / UnwrapGrpcObjectResults convert between the raw proto
// slice used internally and the JSON-RPC-safe wrapper slice exchanged over RPC.
func WrapGrpcObjectResults(results []*rpcv2.GetObjectResult) []*GrpcObjectResult {
	wrapped := make([]*GrpcObjectResult, len(results))
	for i, r := range results {
		wrapped[i] = &GrpcObjectResult{GetObjectResult: r}
	}
	return wrapped
}

func UnwrapGrpcObjectResults(wrapped []*GrpcObjectResult) []*rpcv2.GetObjectResult {
	results := make([]*rpcv2.GetObjectResult, len(wrapped))
	for i, w := range wrapped {
		if w != nil {
			results[i] = w.GetObjectResult
		}
	}
	return results
}

// GrpcTransactionResult wraps *rpcv2.GetTransactionResult so it can cross the
// JSON-RPC boundary between the super node and the driver. Just like
// GrpcObjectResult, GetTransactionResult.Result is a protobuf oneof
// (Transaction | Error) that the default encoding/json cannot round-trip, so we
// route the proto member through protojson instead.
type GrpcTransactionResult struct {
	*rpcv2.GetTransactionResult
}

func (r GrpcTransactionResult) MarshalJSON() ([]byte, error) {
	if r.GetTransactionResult == nil {
		return []byte("null"), nil
	}
	b, err := protojson.Marshal(r.GetTransactionResult)
	if err != nil {
		return nil, errors.Wrap(err, "marshal grpc GetTransactionResult")
	}
	return b, nil
}

func (r *GrpcTransactionResult) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		r.GetTransactionResult = nil
		return nil
	}
	msg := &rpcv2.GetTransactionResult{}
	if err := protojson.Unmarshal(data, msg); err != nil {
		return errors.Wrap(err, "unmarshal grpc GetTransactionResult")
	}
	r.GetTransactionResult = msg
	return nil
}

// WrapGrpcTransactionResults / UnwrapGrpcTransactionResults convert between the
// raw proto slice used internally and the JSON-RPC-safe wrapper slice exchanged
// over RPC.
func WrapGrpcTransactionResults(results []*rpcv2.GetTransactionResult) []*GrpcTransactionResult {
	wrapped := make([]*GrpcTransactionResult, len(results))
	for i, r := range results {
		wrapped[i] = &GrpcTransactionResult{GetTransactionResult: r}
	}
	return wrapped
}

func UnwrapGrpcTransactionResults(wrapped []*GrpcTransactionResult) []*rpcv2.GetTransactionResult {
	results := make([]*rpcv2.GetTransactionResult, len(wrapped))
	for i, w := range wrapped {
		if w != nil {
			results[i] = w.GetTransactionResult
		}
	}
	return results
}

// CommandFilter has 3 parts and linked AND
type CommandFilter struct {
	CallPackage  *string
	CallModule   *string
	CallFunction *string
}

func (f *CommandFilter) CheckCommand(cmd types.Command) bool {
	if f == nil {
		return true
	}
	if f.CallPackage != nil && (cmd.MoveCall == nil || cmd.MoveCall.Package.String() != *f.CallPackage) {
		return false
	}
	if f.CallModule != nil && (cmd.MoveCall == nil || cmd.MoveCall.Module != *f.CallModule) {
		return false
	}
	if f.CallFunction != nil && (cmd.MoveCall == nil || cmd.MoveCall.Function != *f.CallFunction) {
		return false
	}
	return true
}

func (f *CommandFilter) CheckGrpcCommand(cmd *rpcv2.Command) bool {
	if f == nil {
		return true
	}
	if f.CallPackage != nil && cmd.GetMoveCall().GetPackage() != *f.CallPackage {
		return false
	}
	if f.CallModule != nil && cmd.GetMoveCall().GetModule() != *f.CallModule {
		return false
	}
	if f.CallFunction != nil && (cmd.GetMoveCall().GetFunction() != *f.CallFunction) {
		return false
	}
	return true
}

func (f *CommandFilter) Equal(a *CommandFilter) bool {
	if f == nil && a == nil {
		return true
	}
	if f != nil && a != nil {
		return utils.EqualWithNil(f.CallPackage, a.CallPackage) &&
			utils.EqualWithNil(f.CallModule, a.CallModule) &&
			utils.EqualWithNil(f.CallFunction, a.CallFunction)
	}
	return false
}

func (f *CommandFilter) IsEmpty() bool {
	return f == nil || (f.CallPackage == nil && f.CallModule == nil && f.CallFunction == nil)
}

// FunctionFilter has 6 parts, and linked by AND
type FunctionFilter struct {
	Kind *string

	// txn has command match the filter
	CommandFilter *CommandFilter
	// txn have only one Signature and the signature match it
	MultiSigPublicKeyPrefix *string
	// txn sender is it
	Sender *string
	// at least one balance change record whose owner is it
	Receiver *string

	FailedIsOK bool
}

func (f FunctionFilter) Check(tx types.TransactionResponseV1) bool {
	var txV1 *types.TransactionDataV1
	if tx.Transaction != nil && tx.Transaction.Data != nil {
		txV1 = tx.Transaction.Data.V1
	}
	var ptx *types.ProgrammableTransaction
	if txV1 != nil && txV1.Kind != nil {
		ptx = txV1.Kind.Programmable()
	}

	if f.Kind != nil && (txV1 == nil || txV1.Kind == nil || txV1.Kind.Kind() != *f.Kind) {
		return false
	}
	if f.CommandFilter != nil && (ptx == nil || !utils.HasAny(ptx.Commands, f.CommandFilter.CheckCommand)) {
		return false
	}
	if f.MultiSigPublicKeyPrefix != nil {
		prefix, err := hex.DecodeString(strings.TrimPrefix(*f.MultiSigPublicKeyPrefix, "0x"))
		if err != nil {
			return false
		}
		if tx.Transaction == nil || len(tx.Transaction.TxSignatures) != 1 {
			return false
		}
		s := tx.Transaction.TxSignatures[0]
		if !types.IsMultiSigBytes(s) {
			return false
		}
		var sig *types.MultiSig
		if sig, err = types.DecodeMultiSigBytes(s); err != nil {
			return false
		}
		if !utils.HasAny(sig.PublicKey.PkMap, func(pkMap types.MultiSigPkMap) bool {
			return bytes.HasPrefix(pkMap.PubKey.ED25519[:], prefix) ||
				bytes.HasPrefix(pkMap.PubKey.Secp256k1[:], prefix) ||
				bytes.HasPrefix(pkMap.PubKey.Secp256r1[:], prefix)
		}) {
			return false
		}
	}
	if f.Sender != nil && (txV1 == nil || txV1.Sender.String() != *f.Sender) {
		return false
	}
	if f.Receiver != nil {
		if !utils.HasAny(tx.BalanceChanges, func(cc types.BalanceChange) bool {
			_, ownerID, _ := cc.Owner.GetTypeAndID()
			return ownerID == *f.Receiver
		}) {
			return false
		}
	}
	if !f.FailedIsOK && tx.Effects.Status.Status != types.TransactionStatusSuccess {
		return false
	}
	return true
}

func (f FunctionFilter) CheckGrpcTx(tx *rpcv2.ExecutedTransaction) bool {
	if f.Kind != nil && tx.GetTransaction().GetKind().String() != *f.Kind {
		return false
	}
	txCommands := tx.GetTransaction().GetKind().GetProgrammableTransaction().GetCommands()
	if f.CommandFilter != nil && !utils.HasAny(txCommands, f.CommandFilter.CheckGrpcCommand) {
		return false
	}
	if f.MultiSigPublicKeyPrefix != nil {
		prefix, err := hex.DecodeString(strings.TrimPrefix(*f.MultiSigPublicKeyPrefix, "0x"))
		if err != nil {
			return false
		}
		if len(tx.GetSignatures()) != 1 {
			return false
		}
		multisig := tx.GetSignatures()[0].GetMultisig()
		if multisig == nil {
			return false
		}
		if !utils.HasAny(multisig.GetCommittee().GetMembers(), func(m *rpcv2.MultisigMember) bool {
			return bytes.HasPrefix(m.GetPublicKey().GetPublicKey(), prefix)
		}) {
			return false
		}
	}
	if f.Sender != nil && tx.GetTransaction().GetSender() != *f.Sender {
		return false
	}
	if f.Receiver != nil && !utils.HasAny(tx.GetBalanceChanges(), func(bc *rpcv2.BalanceChange) bool {
		return bc.GetAddress() == *f.Receiver
	}) {
		return false
	}
	if !f.FailedIsOK && !tx.GetEffects().GetStatus().GetSuccess() {
		return false
	}
	return true
}

func (f FunctionFilter) IsEmpty() bool {
	return f.Kind == nil &&
		f.CommandFilter == nil &&
		f.MultiSigPublicKeyPrefix == nil &&
		f.Sender == nil &&
		f.Receiver == nil &&
		f.FailedIsOK
}

func (f FunctionFilter) Equal(a FunctionFilter) bool {
	return utils.EqualWithNil(f.Kind, a.Kind) &&
		f.CommandFilter.Equal(a.CommandFilter) &&
		utils.EqualWithNil(f.MultiSigPublicKeyPrefix, a.MultiSigPublicKeyPrefix) &&
		utils.EqualWithNil(f.Sender, a.Sender) &&
		utils.EqualWithNil(f.Receiver, a.Receiver) &&
		f.FailedIsOK == a.FailedIsOK
}

// EventFilterV2 has 2 parts, and linked by AND
type EventFilterV2 struct {
	TypePattern move.TypeSet // empty means any event type is OK
	Sender      *string      // empty means any event sender is OK
}

func (f EventFilterV2) CheckEvent(ev types.Event) bool {
	if f.Sender != nil && ev.Sender != *f.Sender {
		return false
	}
	if len(f.TypePattern) > 0 {
		et, err := move.BuildType(ev.Type.String())
		if err != nil {
			return false
		}
		if !f.TypePattern.Include(et) {
			return false
		}
	}
	return true
}

func (f EventFilterV2) CheckGrpcEvent(ev *rpcv2.Event) bool {
	if f.Sender != nil && ev.GetSender() != *f.Sender {
		return false
	}
	if len(f.TypePattern) > 0 {
		et, err := move.BuildType(ev.GetEventType())
		if err != nil {
			return false
		}
		if !f.TypePattern.Include(et) {
			return false
		}
	}
	return true
}

func (f EventFilterV2) Check(tx types.TransactionResponseV1) bool {
	return utils.HasAny(tx.Events, f.CheckEvent)
}

func (f EventFilterV2) CheckGrpcTx(tx *rpcv2.ExecutedTransaction) bool {
	return utils.HasAny(tx.GetEvents().GetEvents(), f.CheckGrpcEvent)
}

func (f EventFilterV2) Equal(a EventFilterV2) bool {
	return f.TypePattern.Equal(a.TypePattern) && utils.EqualWithNil(f.Sender, a.Sender)
}

func BuildEventChecker(filters []EventFilterV2) func(ev types.Event) bool {
	return func(ev types.Event) bool {
		return utils.HasAny(filters, func(ff EventFilterV2) bool {
			return ff.CheckEvent(ev)
		})
	}
}

func BuildGrpcEventChecker(filters []EventFilterV2) func(ev *rpcv2.Event) bool {
	return func(ev *rpcv2.Event) bool {
		return utils.HasAny(filters, func(ff EventFilterV2) bool {
			return ff.CheckGrpcEvent(ev)
		})
	}
}

// TransactionFilter has 3 parts,
// check logic is match FailedIsOK AND match any (EventFilters + FunctionFilters)
type TransactionFilter struct {
	FunctionFilters []FunctionFilter
	EventFilters    []EventFilterV2

	FailedIsOK bool
}

func (f TransactionFilter) Check(tx types.TransactionResponseV1) bool {
	if !f.FailedIsOK && tx.Effects.Status.Status != types.TransactionStatusSuccess {
		return false
	}
	for _, ff := range f.FunctionFilters {
		if ff.Check(tx) {
			return true
		}
	}
	for _, ff := range f.EventFilters {
		if ff.Check(tx) {
			return true
		}
	}
	return false
}

func (f TransactionFilter) CheckGrpcTx(tx *rpcv2.ExecutedTransaction) bool {
	if !f.FailedIsOK && !tx.GetEffects().GetStatus().GetSuccess() {
		return false
	}
	for _, ff := range f.FunctionFilters {
		if ff.CheckGrpcTx(tx) {
			return true
		}
	}
	for _, ff := range f.EventFilters {
		if ff.CheckGrpcTx(tx) {
			return true
		}
	}
	return false
}

func (f TransactionFilter) Merge(a TransactionFilter) (r TransactionFilter) {
	for _, ef := range f.EventFilters {
		r.EventFilters = append(r.EventFilters, ef)
	}
	for _, ef := range a.EventFilters {
		if !utils.HasAny(f.EventFilters, ef.Equal) {
			r.EventFilters = append(r.EventFilters, ef)
		}
	}
	for _, ff := range f.FunctionFilters {
		r.FunctionFilters = append(r.FunctionFilters, ff)
	}
	for _, ff := range a.FunctionFilters {
		if !utils.HasAny(f.FunctionFilters, ff.Equal) {
			r.FunctionFilters = append(r.FunctionFilters, ff)
		}
	}
	r.FailedIsOK = f.FailedIsOK || a.FailedIsOK
	return
}

type TransactionFetchConfig struct {
	NeedInputs    bool `json:"needInputs"`
	NeedEffects   bool `json:"needEffects"`
	NeedAllEvents bool `json:"needAllEvents"`
}

func (f TransactionFetchConfig) String() string {
	return fmt.Sprintf("NeedInputs:%v,NeedEffects:%v,NeedAllEvents:%v", f.NeedInputs, f.NeedEffects, f.NeedAllEvents)
}

func (f TransactionFetchConfig) Merge(a TransactionFetchConfig) (r TransactionFetchConfig) {
	return TransactionFetchConfig{
		NeedInputs:    f.NeedInputs || a.NeedInputs,
		NeedEffects:   f.NeedEffects || a.NeedEffects,
		NeedAllEvents: f.NeedAllEvents || a.NeedAllEvents,
	}
}

func (f TransactionFetchConfig) PruneTransaction(
	tx types.TransactionResponseV1,
	eventFilters []EventFilterV2,
) types.TransactionResponseV1 {
	r := tx
	if !f.NeedAllEvents {
		r.Events = utils.FilterArr(tx.Events, BuildEventChecker(eventFilters))
	}
	if !f.NeedInputs {
		r.Transaction = nil
	}
	if !f.NeedEffects {
		r.Effects = &types.TransactionEffectsV1{
			MessageVersion:     tx.Effects.MessageVersion,
			Status:             tx.Effects.Status,
			ExecutedEpoch:      tx.Effects.ExecutedEpoch,
			GasUsed:            tx.Effects.GasUsed,
			ModifiedAtVersions: tx.Effects.ModifiedAtVersions,
			TransactionDigest:  tx.Effects.TransactionDigest,
			GasObject:          tx.Effects.GasObject,
		}
	}
	return r
}

// PruneGrpcTransaction returns a pruned copy of tx without mutating the shared/cached
// *rpcv2.ExecutedTransaction. The pruning rules mirror PruneTransaction:
//   - !NeedAllEvents: keep only the events matching eventFilters
//   - !NeedInputs: drop the transaction inputs/commands
//   - !NeedEffects: keep only the lightweight effects fields, drop the big changed-objects set
func (f TransactionFetchConfig) PruneGrpcTransaction(
	tx *ExtendedGrpcTransaction,
	eventFilters []EventFilterV2,
) *ExtendedGrpcTransaction {
	if tx == nil {
		return nil
	}
	src := tx.ExecutedTransaction
	// shallow copy so the shared/cached tx is never mutated
	pruned := &rpcv2.ExecutedTransaction{
		Digest:         src.Digest,
		Transaction:    src.Transaction,
		Signatures:     src.Signatures,
		Effects:        src.Effects,
		Events:         src.Events,
		Checkpoint:     src.Checkpoint,
		Timestamp:      src.Timestamp,
		BalanceChanges: src.BalanceChanges,
		Objects:        src.Objects,
	}
	var eventIndexes []int
	if !f.NeedAllEvents {
		checker := BuildGrpcEventChecker(eventFilters)
		full := src.GetEvents().GetEvents()
		kept := make([]*rpcv2.Event, 0, len(full))
		eventIndexes = make([]int, 0, len(full))
		for i, ev := range full {
			if checker(ev) {
				kept = append(kept, ev)
				eventIndexes = append(eventIndexes, i)
			}
		}
		pruned.Events = &rpcv2.TransactionEvents{
			Bcs:    src.GetEvents().GetBcs(),
			Digest: src.GetEvents().Digest,
			Events: kept,
		}
	}
	if !f.NeedInputs {
		pruned.Transaction = nil
	}
	if !f.NeedEffects {
		eff := src.GetEffects()
		pruned.Effects = &rpcv2.TransactionEffects{
			Bcs:               eff.GetBcs(),
			Digest:            eff.Digest,
			Version:           eff.Version,
			Status:            eff.Status,
			Epoch:             eff.Epoch,
			GasUsed:           eff.GasUsed,
			TransactionDigest: eff.TransactionDigest,
			GasObject:         eff.GasObject,
			LamportVersion:    eff.LamportVersion,
			// drop the big ChangedObjects / dependencies / consensus objects sets
		}
	}
	r := *tx
	r.ExecutedTransaction = pruned
	r.EventIndexes = eventIndexes
	return &r
}

type ObjectCreation struct {
	ObjectVersion uint64
	Checkpoint    uint64
}

type SimpleCheckpoint struct {
	Checkpoint  uint64 `json:"checkpoint"`
	Digest      string `json:"digest"`
	TimestampMS uint64 `json:"timestamp"`
}

func NewSimpleCheckpoint(slot *Slot) SimpleCheckpoint {
	return SimpleCheckpoint{
		Checkpoint:  slot.SequenceNumber,
		Digest:      slot.Digest,
		TimestampMS: slot.TimestampMs.Uint64(),
	}
}

const APIVersion = 1 // api version, if api version increased, all driver client will restart

type GetLatestSimpleCheckpointResponse struct {
	Checkpoint SimpleCheckpoint `json:"checkpoint"`
	APIVersion int              `json:"api_version"`
}

func (r GetLatestSimpleCheckpointResponse) CheckAPIVersion() error {
	if r.APIVersion <= APIVersion {
		return nil
	}
	return errors.Errorf("remote api version %d is greater than %d", r.APIVersion, APIVersion)
}
