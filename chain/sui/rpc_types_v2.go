package sui

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

// ObjectChangeOwnerFilter need the objects which id in OwnerID
// and owned by the objects in OwnerID with owner type in OwnerType
type ObjectChangeOwnerFilter struct {
	OwnerID   []string `json:"owner_id"`             // empty means need nothing
	OwnerType []string `json:"owner_type,omitempty"` // empty means all owned object are not needed
}

func (f ObjectChangeOwnerFilter) Merge(a ObjectChangeOwnerFilter) (r ObjectChangeOwnerFilter) {
	r.OwnerID = utils.GetOrderedMapKeys(utils.BuildSet(utils.MergeArr(f.OwnerID, a.OwnerID)))
	r.OwnerType = utils.GetOrderedMapKeys(utils.BuildSet(utils.MergeArr(f.OwnerType, a.OwnerType)))
	return r
}

func (f ObjectChangeOwnerFilter) Check(oc types.ObjectChangeExtend) bool {
	if len(f.OwnerID) == 0 {
		return false
	}
	if utils.IndexOf(f.OwnerID, oc.GetObjectID()) >= 0 {
		return true
	}
	if len(f.OwnerType) == 0 {
		return false
	}
	ownerType, ownerID, _ := oc.Owner.GetTypeAndID()
	return utils.IndexOf(f.OwnerID, ownerID) >= 0 && utils.IndexOf(f.OwnerType, ownerType) >= 0
}

// ObjectChangeFilter has 3 parts, linked by OR
type ObjectChangeFilter struct {
	TypePattern move.TypeSet             // empty means any type is OK
	OwnerFilter *ObjectChangeOwnerFilter // nil means any owner is OK
	ObjectIDIn  set.Set[string]          // empty means any object id is OK
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

func (f ObjectChangeFilter) Check(oc types.ObjectChangeExtend) bool {
	if f.IsEmpty() {
		return true
	}
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
	if f.OwnerFilter != nil && f.OwnerFilter.Check(oc) {
		return true
	}
	if f.ObjectIDIn != nil && !f.ObjectIDIn.Empty() && f.ObjectIDIn.Contains(oc.GetObjectID()) {
		return true
	}
	return false
}

func (f ObjectChangeFilter) Merge(a ObjectChangeFilter) (r ObjectChangeFilter) {
	if f.IsEmpty() {
		return f
	}
	if a.IsEmpty() {
		return a
	}
	if len(f.TypePattern) > 0 && len(a.TypePattern) > 0 {
		r.TypePattern = f.TypePattern.Merge(a.TypePattern)
	}
	if f.OwnerFilter != nil && a.OwnerFilter != nil {
		r.OwnerFilter = utils.WrapPointer(f.OwnerFilter.Merge(*a.OwnerFilter))
	}
	if f.ObjectIDIn != nil && !f.ObjectIDIn.Empty() && a.ObjectIDIn != nil && !a.ObjectIDIn.Empty() {
		r.ObjectIDIn = set.SmartNew[string](f.ObjectIDIn.DumpValues(), a.ObjectIDIn.DumpValues())
	} else {
		r.ObjectIDIn = set.New[string]()
	}
	return r
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
		ptx = txV1.Kind.ProgrammableTransaction
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

func (f EventFilterV2) Check(tx types.TransactionResponseV1) bool {
	return utils.HasAny(tx.Events, f.CheckEvent)
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
