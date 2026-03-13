package aptos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type Transaction struct {
	// events and changes in CommittedTransaction are useless
	api.CommittedTransaction

	// the actually events and changes will be stored in the fields below
	// so the events will include the event index
	Events  []*EventExtend    `json:"events"`
	Changes []*WriteSetChange `json:"changes"`
}

func GetTransactionChanges(t api.CommittedTransaction) []*api.WriteSetChange {
	switch t.Type {
	case api.TransactionVariantUser:
		tx, _ := t.UserTransaction()
		return tx.Changes
	case api.TransactionVariantGenesis:
		tx, _ := t.GenesisTransaction()
		return tx.Changes
	case api.TransactionVariantBlockMetadata:
		tx, _ := t.BlockMetadataTransaction()
		return tx.Changes
	case api.TransactionVariantBlockEpilogue:
		tx, _ := t.BlockEpilogueTransaction()
		return tx.Changes
	case api.TransactionVariantStateCheckpoint:
		tx, _ := t.StateCheckpointTransaction()
		return tx.Changes
	case api.TransactionVariantValidator:
		tx, _ := t.ValidatorTransaction()
		return tx.Changes
	}
	return nil
}

func GetTransactionEvents(t api.CommittedTransaction) []*api.Event {
	switch t.Type {
	case api.TransactionVariantUser:
		tx, _ := t.UserTransaction()
		return tx.Events
	case api.TransactionVariantGenesis:
		tx, _ := t.GenesisTransaction()
		return tx.Events
	case api.TransactionVariantBlockMetadata:
		tx, _ := t.BlockMetadataTransaction()
		return tx.Events
	case api.TransactionVariantBlockEpilogue:
		tx, _ := t.BlockEpilogueTransaction()
		return tx.Events
	case api.TransactionVariantValidator:
		tx, _ := t.ValidatorTransaction()
		return tx.Events
	}
	return nil
}

func NewTransaction(raw api.CommittedTransaction) Transaction {
	t := Transaction{
		CommittedTransaction: raw,
	}
	rawEvents := GetTransactionEvents(raw)
	rawChanges := GetTransactionChanges(raw)
	t.Events = make([]*EventExtend, len(rawEvents))
	for index, ev := range rawEvents {
		t.Events[index] = &EventExtend{Event: *ev, Index: int32(index)}
	}
	t.Changes = make([]*WriteSetChange, len(rawChanges))
	for index, change := range rawChanges {
		t.Changes[index] = &WriteSetChange{WriteSetChange: *change}
	}
	return t
}

func (t Transaction) MarshalJSON() ([]byte, error) {
	ret, err := json.Marshal(&t.CommittedTransaction)
	if err != nil {
		return nil, err
	}
	ret, err = sjson.SetBytes(ret, "events", t.Events)
	if err != nil {
		return nil, err
	}
	ret, err = sjson.SetBytes(ret, "changes", t.Changes)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (t *Transaction) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &t.CommittedTransaction); err != nil {
		return err
	}
	var payload struct {
		Events  []*EventExtend    `json:"events"`
		Changes []*WriteSetChange `json:"changes"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	t.Events, t.Changes = payload.Events, payload.Changes
	return nil
}

func (t Transaction) Timestamp() *uint64 {
	switch t.Type {
	case api.TransactionVariantUser:
		tx, _ := t.UserTransaction()
		return &tx.Timestamp
	case api.TransactionVariantGenesis: // always the first txn of the whole chain
		var ts uint64 = 0
		return &ts
	case api.TransactionVariantBlockMetadata:
		tx, _ := t.BlockMetadataTransaction()
		return &tx.Timestamp
	case api.TransactionVariantBlockEpilogue:
		tx, _ := t.BlockEpilogueTransaction()
		return &tx.Timestamp
	case api.TransactionVariantStateCheckpoint:
		tx, _ := t.StateCheckpointTransaction()
		return &tx.Timestamp
	case api.TransactionVariantValidator:
		tx, _ := t.ValidatorTransaction()
		return &tx.Timestamp
	default:
		return nil
	}
}

func (t Transaction) Time() *time.Time {
	ts := t.Timestamp()
	if ts == nil {
		return nil
	}
	tm := time.UnixMicro(int64(*ts))
	return &tm
}

func (t Transaction) EntryFunction() string {
	var payload *api.TransactionPayload
	switch t.Type {
	case api.TransactionVariantUser:
		tx, _ := t.UserTransaction()
		payload = tx.Payload
	case api.TransactionVariantGenesis:
		tx, _ := t.GenesisTransaction()
		payload = tx.Payload
	default:
		return ""
	}
	if payload != nil {
		switch payload.Type {
		case api.TransactionPayloadVariantEntryFunction:
			f := payload.Inner.(*api.TransactionPayloadEntryFunction)
			return f.Function
		case api.TransactionPayloadVariantMultisig:
			f := payload.Inner.(*api.TransactionPayloadMultisig)
			if f.TransactionPayload != nil && f.TransactionPayload.Type == api.TransactionPayloadVariantEntryFunction {
				p := f.TransactionPayload.Inner.(*api.TransactionPayloadEntryFunction)
				return p.Function
			}
		}
	}
	return ""
}

func (t Transaction) PayloadType() api.TransactionPayloadVariant {
	switch t.Type {
	case api.TransactionVariantUser:
		tx, _ := t.UserTransaction()
		return tx.Payload.Type
	case api.TransactionVariantGenesis:
		tx, _ := t.GenesisTransaction()
		return tx.Payload.Type
	default:
		return api.TransactionPayloadVariantUnknown
	}
}

func (t Transaction) EntryFunctionTypeArguments() []string {
	var payload *api.TransactionPayload
	switch t.Type {
	case api.TransactionVariantUser:
		tx, _ := t.UserTransaction()
		payload = tx.Payload
	case api.TransactionVariantGenesis:
		tx, _ := t.GenesisTransaction()
		payload = tx.Payload
	default:
		return nil
	}
	if payload != nil {
		switch payload.Type {
		case api.TransactionPayloadVariantEntryFunction:
			f := payload.Inner.(*api.TransactionPayloadEntryFunction)
			return f.TypeArguments
		case api.TransactionPayloadVariantMultisig:
			f := payload.Inner.(*api.TransactionPayloadMultisig)
			if f.TransactionPayload != nil && f.TransactionPayload.Type == api.TransactionPayloadVariantEntryFunction {
				p := f.TransactionPayload.Inner.(*api.TransactionPayloadEntryFunction)
				return p.TypeArguments
			}
		}
	}
	return nil
}

type EventExtend struct {
	api.Event

	Index int32 `json:"event_index"`
}

func (w *EventExtend) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &w.Event); err != nil {
		return err
	}
	var payload struct {
		Index int32 `json:"event_index"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	w.Index = payload.Index
	return nil
}

func (w EventExtend) MarshalJSON() ([]byte, error) {
	ret, err := json.Marshal(&w.Event)
	if err == nil {
		ret, err = sjson.SetBytes(ret, "event_index", w.Index)
	}
	return ret, err
}

type WriteSetChange struct {
	api.WriteSetChange
}

func (w *WriteSetChange) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &w.WriteSetChange)
}

func (w WriteSetChange) MarshalJSON() ([]byte, error) {
	return w.WriteSetChange.MarshalJSON()
}

func GetChangeAddress(w api.WriteSetChange) *aptos.AccountAddress {
	switch w.Type {
	case api.WriteSetChangeVariantWriteResource:
		inner := w.Inner.(*api.WriteSetChangeWriteResource)
		return inner.Address
	case api.WriteSetChangeVariantDeleteResource:
		inner := w.Inner.(*api.WriteSetChangeDeleteResource)
		return inner.Address
	case api.WriteSetChangeVariantWriteModule:
		inner := w.Inner.(*api.WriteSetChangeWriteModule)
		return inner.Address
	case api.WriteSetChangeVariantDeleteModule:
		inner := w.Inner.(*api.WriteSetChangeDeleteModule)
		return inner.Address
	default:
		return nil
	}
}

func GetChangeResource(w api.WriteSetChange) *api.MoveResource {
	switch w.Type {
	case api.WriteSetChangeVariantWriteResource:
		inner := w.Inner.(*api.WriteSetChangeWriteResource)
		return inner.Data
	default:
		return nil
	}
}

func GetChangeResourceType(w api.WriteSetChange) *string {
	if res := GetChangeResource(w); res != nil {
		// address part of resource type should use short address
		resType := move.TrimTypeString(res.Type)
		return &resType
	}
	return nil
}

func (w WriteSetChange) Address() *aptos.AccountAddress {
	return GetChangeAddress(w.WriteSetChange)
}

func (w WriteSetChange) Resource() *api.MoveResource {
	return GetChangeResource(w.WriteSetChange)
}

func (w WriteSetChange) ResourceType() *string {
	return GetChangeResourceType(w.WriteSetChange)
}

type ChangeStat struct {
	MinTxVersion   uint64 `json:"min_tx_version"`
	MaxTxVersion   uint64 `json:"max_tx_version"`
	MinBlockHeight uint64 `json:"min_block_height"`
	MaxBlockHeight uint64 `json:"max_block_height"`
	Count          uint64 `json:"count"`
}

type ResourceChangeArgs struct {
	FromVersion                   uint64   `json:"fromVersion"`
	ToVersion                     uint64   `json:"toVersion"`
	Addresses                     []string `json:"address"`
	ResourceChangesMoveTypePrefix string   `json:"resourceChangesMoveTypePrefix,omitempty"`
}

func (r ResourceChangeArgs) ChangeFilter() func(wc *WriteSetChange) bool {
	addrSet := set.New[string](r.Addresses...)
	if addrSet.Contains("*") {
		addrSet.Truncate()
	}
	changeType, _ := move.BuildType(r.ResourceChangesMoveTypePrefix)
	return func(wc *WriteSetChange) bool {
		if !addrSet.Empty() {
			if addr := wc.Address(); addr == nil || !addrSet.Contains(addr.String()) {
				return false
			}
		}
		if !changeType.IsAny() {
			if !changeType.IncludeTypeString(wc.ResourceType()) {
				return false
			}
		}
		return true
	}
}

type GetFunctionsArgs struct {
	FromVersion                   uint64   `json:"fromVersion"`
	ToVersion                     uint64   `json:"toVersion"`
	Function                      string   `json:"function"`
	MatchAll                      bool     `json:"matchAll"`
	TypedArguments                []string `json:"typedArguments"`
	IncludeChanges                bool     `json:"includeChanges"`
	IncludeAllEvents              bool     `json:"includeAllEvents"`
	ResourceChangesMoveTypePrefix string   `json:"resourceChangesMoveTypePrefix,omitempty"`
	IncludeMultiSigFunc           bool     `json:"includeMultiSigFunc,omitempty"`
	IncludeFailedTransaction      bool     `json:"includeFailedTransaction,omitempty"`
	Sender                        string   `json:"sender,omitempty"`
}

func (r GetFunctionsArgs) TxnFilter() func(resp *Transaction) bool {
	var fnPattern move.Type
	if r.Function != "" {
		if strings.Index(r.Function, "::") < 0 {
			fnPattern, _ = move.BuildType(r.Function + "::")
		} else {
			fnPattern, _ = move.BuildType(r.Function)
		}
	}
	sender := move.ToShortAddress(r.Sender)
	return func(tx *Transaction) bool {
		if !r.IncludeMultiSigFunc {
			if tx.PayloadType() == api.TransactionPayloadVariantMultisig {
				return false
			}
		}
		if sender != "" {
			if userTx, err := tx.UserTransaction(); err != nil {
				// not user transaction, but only user transaction has sender.
				return false
			} else if userTx.Sender == nil || move.ToShortAddress(userTx.Sender.StringLong()) != sender {
				return false
			}
		}
		if !fnPattern.IsAny() {
			if !fnPattern.IncludeTypeString(utils.WrapPointer(tx.EntryFunction())) {
				return false
			}
		}
		if !r.MatchAll {
			if !utils.ArrEqual(tx.EntryFunctionTypeArguments(), r.TypedArguments) {
				return false
			}
		}
		return true
	}
}

type GetEventsArgs struct {
	Network                       string `json:"network,omitempty"`
	FromVersion                   uint64 `json:"fromVersion"`
	ToVersion                     uint64 `json:"toVersion"`
	Address                       string `json:"address"`
	Type                          string `json:"type"` // maybe contain arg parts like '<xxx>'
	IncludeChanges                bool   `json:"includeChanges"`
	IncludeAllEvents              bool   `json:"includeAllEvents"`
	ResourceChangesMoveTypePrefix string `json:"resourceChangesMoveTypePrefix,omitempty"`
	IncludeFailedTransaction      bool   `json:"includeFailedTransaction,omitempty"`
	AccountAddress                string `json:"accountAddress,omitempty"`
}

func (r GetEventsArgs) EventFilter() func(extend api.Event) bool {
	pattern, _ := move.BuildType(fmt.Sprintf("%s::%s", r.Address, r.Type))
	accountAddress := move.ToShortAddress(r.AccountAddress)
	return func(evt api.Event) bool {
		// check account address
		if accountAddress != "" {
			if evt.Guid == nil || evt.Guid.AccountAddress == nil {
				return false
			}
			if move.ToShortAddress(evt.Guid.AccountAddress.String()) != accountAddress {
				return false
			}
		}
		// check event type
		if !pattern.IncludeTypeString(&evt.Type) {
			return false
		}
		return true
	}
}

// ChangeFilter has 2 parts, there are linked by AND
type ChangeFilter struct {
	// empty means any address is ok
	Address set.Set[string]

	// empty means any resource type is not ok, means no change can pass this filter.
	// if any resource type is ok, ResourceTypes should be ["*"]
	ResourceTypes move.TypeSet
}

type changeFilterPayload struct {
	Address       []string     `json:"address"`
	ResourceTypes move.TypeSet `json:"resourceTypes"`
}

func (f ChangeFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(changeFilterPayload{
		Address:       f.Address.DumpValues(),
		ResourceTypes: f.ResourceTypes,
	})
}

func (f *ChangeFilter) UnmarshalJSON(b []byte) error {
	var payload changeFilterPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	f.Address = set.New(payload.Address...)
	f.ResourceTypes = payload.ResourceTypes
	return nil
}

func (f ChangeFilter) String() string {
	return fmt.Sprintf("Address:%s,ResourceTypes:[%s]",
		utils.ArrSummary(f.Address.DumpValues(), 10),
		f.ResourceTypes.String())
}

func (f ChangeFilter) Check(c WriteSetChange) bool {
	if !f.Address.Empty() {
		if addr := c.Address(); addr == nil || !f.Address.Contains(addr.String()) {
			return false
		}
	}
	return f.ResourceTypes.IncludeTypeString(c.ResourceType())
}

func MergeChangeFilters(filters ...ChangeFilter) (r ChangeFilter) {
	if len(filters) == 0 {
		panic("fs is empty")
	}
	r.Address = set.New[string]()
	for _, filter := range filters {
		if filter.Address.Empty() {
			r.Address = set.New[string]()
			break
		}
		r.Address.Add(filter.Address.DumpValues()...)
	}
	r.ResourceTypes = filters[0].ResourceTypes
	for i := 1; i < len(filters); i++ {
		r.ResourceTypes = r.ResourceTypes.Merge(filters[i].ResourceTypes)
	}
	return r
}

// EventFilter has 2 parts, there are linked by AND
type EventFilter struct {
	Type              move.Type `json:"type"`                // empty means not limit the event type
	GuiAccountAddress *string   `json:"gui_account_address"` // empty means not limit the gui account address
}

func (f *EventFilter) UnmarshalJSON(data []byte) error {
	payload := struct {
		Type              string  `json:"type"`
		GuiAccountAddress *string `json:"gui_account_address"`
	}{}
	err := json.Unmarshal(data, &payload)
	if err != nil {
		return err
	}
	f.Type, err = move.BuildType(payload.Type)
	if err != nil {
		return errors.Wrapf(err, "invalid type %q", payload.Type)
	}
	if payload.GuiAccountAddress != nil {
		f.GuiAccountAddress = utils.WrapPointer(move.ToShortAddress(*payload.GuiAccountAddress))
	}
	return nil
}

func (f EventFilter) String() string {
	var b bytes.Buffer
	b.WriteString("TypePattern:")
	b.WriteString(f.Type.String())
	if f.GuiAccountAddress != nil {
		b.WriteString(",GuiAccountAddress:")
		b.WriteString(*f.GuiAccountAddress)
	}
	return b.String()
}

func (f EventFilter) IsEmpty() bool {
	return f.Type.IsAny() && f.GuiAccountAddress == nil
}

func (f EventFilter) Equal(a EventFilter) bool {
	return f.Type.Equal(&a.Type) && utils.EqualWithNil(f.GuiAccountAddress, a.GuiAccountAddress)
}

func (f EventFilter) CheckEvent(ev *EventExtend) bool {
	if !f.Type.IncludeTypeString(&ev.Type) {
		return false
	}
	if f.GuiAccountAddress != nil {
		if ev.Guid == nil || ev.Guid.AccountAddress == nil {
			return false
		}
		if move.ToShortAddress(ev.Guid.AccountAddress.String()) != *f.GuiAccountAddress {
			return false
		}
	}
	return true
}

func (f EventFilter) Check(tx Transaction) bool {
	return utils.HasAny(tx.Events, f.CheckEvent)
}

func BuildEventFilter(filters []EventFilter) func(ev *EventExtend) bool {
	return func(ev *EventExtend) bool {
		return utils.HasAny(filters, func(ff EventFilter) bool {
			return ff.CheckEvent(ev)
		})
	}
}

// FunctionFilter has 3 parts, there are linked by AND
type FunctionFilter struct {
	// function condition
	FunctionPattern move.Type `json:"function"`

	// function arguments condition
	CheckTypeArguments bool     `json:"check_type_arguments"`
	TypedArguments     []string `json:"typed_arguments"`

	// sender condition
	Sender *string `json:"sender"`
}

func (f FunctionFilter) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("FunctionPattern:%s", f.FunctionPattern))
	if f.CheckTypeArguments {
		parts = append(parts, fmt.Sprintf("Args:[%s]", strings.Join(f.TypedArguments, ",")))
	}
	if f.Sender != nil {
		parts = append(parts, fmt.Sprintf("Sender:%s", *f.Sender))
	}
	return strings.Join(parts, ",")
}

func (f FunctionFilter) IsEmpty() bool {
	return f.FunctionPattern.IsAny() && !f.CheckTypeArguments && f.Sender == nil
}

func (f FunctionFilter) Equal(a FunctionFilter) bool {
	if !f.FunctionPattern.Equal(&a.FunctionPattern) {
		return false
	}
	if f.CheckTypeArguments != a.CheckTypeArguments {
		return false
	}
	if !utils.ArrEqual(f.TypedArguments, a.TypedArguments) {
		return false
	}
	if !utils.EqualWithNil(f.Sender, a.Sender) {
		return false
	}
	return true
}

func (f FunctionFilter) Check(tx Transaction) bool {
	if !f.FunctionPattern.IncludeTypeString(utils.WrapPointer(tx.EntryFunction())) {
		return false
	}
	if f.CheckTypeArguments && !utils.ArrEqual(tx.EntryFunctionTypeArguments(), f.TypedArguments) {
		return false
	}
	if f.Sender != nil {
		if userTx, err := tx.UserTransaction(); err != nil || userTx.Sender.String() != *f.Sender {
			return false
		}
	}
	return true
}

// TransactionFilter has 4 parts,
// check logic is match FailedIsOK AND match MultiSigTxnIsOK AND match any (EventFilters + FunctionFilters)
type TransactionFilter struct {
	EventFilters    []EventFilter    `json:"eventFilters"`
	FunctionFilters []FunctionFilter `json:"functionFilters"`

	FailedIsOK      bool `json:"failedIsOK"`
	MultiSigTxnIsOK bool `json:"multiSigTxnIsOK"`
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
	r.MultiSigTxnIsOK = f.MultiSigTxnIsOK || a.MultiSigTxnIsOK
	return
}

func (f TransactionFilter) String() string {
	return fmt.Sprintf("EventFilters:[%s],FunctionFilters:[%s],FailedIsOK:%v,MultiSigTxnIsOK:%v",
		strings.Join(utils.MapSliceNoError(f.EventFilters, EventFilter.String), "|"),
		strings.Join(utils.MapSliceNoError(f.FunctionFilters, FunctionFilter.String), "|"),
		f.FailedIsOK,
		f.MultiSigTxnIsOK)
}

func (f TransactionFilter) Check(tx Transaction) bool {
	if !f.FailedIsOK && !tx.Success() {
		return false
	}
	if !f.MultiSigTxnIsOK && tx.PayloadType() == api.TransactionPayloadVariantMultisig {
		return false
	}
	return utils.HasAny(f.EventFilters, func(f EventFilter) bool {
		return f.Check(tx)
	}) || utils.HasAny(f.FunctionFilters, func(f FunctionFilter) bool {
		return f.Check(tx)
	})
}

type TransactionFetchConfig struct {
	NeedAllEvents       bool         `json:"needAllEvents"`
	ChangeResourceTypes move.TypeSet `json:"changeResourceTypes"` // empty means do not need any change
}

func (f TransactionFetchConfig) String() string {
	return fmt.Sprintf("NeedAllEvents:%v,ChangeResourceTypes:[%s]", f.NeedAllEvents, f.ChangeResourceTypes)
}

func (f TransactionFetchConfig) Merge(a TransactionFetchConfig) (r TransactionFetchConfig) {
	r.NeedAllEvents = f.NeedAllEvents || a.NeedAllEvents
	r.ChangeResourceTypes = f.ChangeResourceTypes.Merge(a.ChangeResourceTypes)
	return r
}

func (f TransactionFetchConfig) PruneTransaction(txn Transaction, eventFilters []EventFilter) Transaction {
	r := txn
	if !f.NeedAllEvents {
		r.Events = utils.FilterArr(txn.Events, BuildEventFilter(eventFilters))
	}
	r.Changes = utils.FilterArr(txn.Changes, func(c *WriteSetChange) bool {
		return f.ChangeResourceTypes.IncludeTypeString(c.ResourceType())
	})
	// r.Events and r.Changes should be a empty string at least, can not be nil
	if r.Events == nil {
		r.Events = make([]*EventExtend, 0)
	}
	if r.Changes == nil {
		r.Changes = make([]*WriteSetChange, 0)
	}
	return r
}

type GetResourceChangesRequest struct {
	FromVersion uint64       `json:"fromVersion"`
	ToVersion   uint64       `json:"toVersion"`
	Filter      ChangeFilter `json:"filter"`
}

type GetTransactionsRequest struct {
	FromVersion uint64            `json:"fromVersion"`
	ToVersion   uint64            `json:"toVersion"`
	Filter      TransactionFilter `json:"filter"`

	FetchConfig TransactionFetchConfig `json:"fetchConfig"`
}

const APIVersion = 1

type MinimalistTransaction struct {
	Version     uint64 `json:"version"`
	Hash        string `json:"hash"`
	TimestampMS int64  `json:"timestamp"`
}

func NewMinimalistTransaction(tx api.CommittedTransaction) MinimalistTransaction {
	// TransactionVariantGenesis txn is the first txn of the chain without timestamp
	txn := Transaction{CommittedTransaction: tx}
	var ts uint64
	if t := txn.Timestamp(); t != nil {
		ts = *t
	}
	return MinimalistTransaction{
		Version:     tx.Version(),
		Hash:        tx.Hash(),
		TimestampMS: int64(ts),
	}
}

type GetLatestMinimalistTransactionResponse struct {
	Transaction MinimalistTransaction `json:"transaction"`
	APIVersion  int                   `json:"api_version"`
}

func (r GetLatestMinimalistTransactionResponse) CheckAPIVersion() error {
	if r.APIVersion <= APIVersion {
		return nil
	}
	return errors.Errorf("remote api version %d is greater than %d", r.APIVersion, APIVersion)
}

type MinimalistTransactionWithChanges struct {
	MinimalistTransaction `json:",inline"`

	Changes []WriteSetChange `json:"changes"`
}
