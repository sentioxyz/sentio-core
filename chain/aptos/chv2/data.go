package chv2

import (
	"encoding/json"
	aptosSdk "github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type BlockIndex struct {
	BlockHeight    uint64    `clickhouse:"block_height"    index:"minmax" number_field:"true"`
	BlockTimestamp time.Time `clickhouse:"block_timestamp" index:"minmax"`
	BlockHash      string    `clickhouse:"block_hash"      index:"bloom_filter"`
}

type Block struct {
	BlockIndex
	Epoch                    uint64  `clickhouse:"epoch"`
	Round                    uint64  `clickhouse:"round"`
	PreviousBlockVotesBitvec string  `clickhouse:"previous_block_votes_bitvec"`
	Proposer                 *string `clickhouse:"proposer" type:"Nullable(FixedString(66))"`
	FirstVersion             uint64  `clickhouse:"first_version"`
	LastVersion              uint64  `clickhouse:"last_version"`
	TransactionsCount        int64   `clickhouse:"transactions_count"`
}

type TransactionIndex struct {
	TransactionHash    string `clickhouse:"transaction_hash"    index:"idx_tx_hash/bloom_filter" required:"true"`
	TxIndex            uint64 `clickhouse:"transaction_index"`
	TransactionVersion uint64 `clickhouse:"transaction_version" index:"idx_tx_version/minmax"    required:"true" sub_number_field:"true"`
}

type Transaction struct {
	BlockIndex
	TransactionIndex
	Type                    string    `clickhouse:"type" required:"true"`
	AccumulatorRootHash     string    `clickhouse:"accumulator_root_hash" required:"true"`
	StateChangeHash         string    `clickhouse:"state_change_hash" required:"true"`
	EventRootHash           string    `clickhouse:"event_root_hash" required:"true"`
	GasUsed                 uint64    `clickhouse:"gas_used" required:"true"`
	Success                 bool      `clickhouse:"success" required:"true"`
	VMStatus                string    `clickhouse:"vm_status" required:"true"`
	Sender                  *string   `clickhouse:"sender" type:"Nullable(FixedString(66))" required:"true"`
	SequenceNumber          uint64    `clickhouse:"sequence_number" required:"true"`
	MaxGasAmount            uint64    `clickhouse:"max_gas_amount" required:"true"`
	GasUnitPrice            uint64    `clickhouse:"gas_unit_price" required:"true"`
	Timestamp               time.Time `clickhouse:"timestamp" required:"true"`
	ExpirationTimestampSecs uint64    `clickhouse:"expiration_timestamp_secs" required:"true"`
	StateCheckpointHash     string    `clickhouse:"state_checkpoint_hash" required:"true"`
	Signature               string    `clickhouse:"signature" required:"true"`

	Payload                    string   `clickhouse:"payload" required:"true"`
	PayloadType                string   `clickhouse:"payload_type"`
	MultiSignAddress           *string  `clickhouse:"multisig_address" type:"Nullable(FixedString(66))"`
	EntryFunction              string   `clickhouse:"entry_function"`
	EntryFunctionArguments     []string `clickhouse:"entry_function_arguments"`
	EntryFunctionTypeArguments []string `clickhouse:"entry_function_type_arguments"`
	ScriptAbi                  string   `clickhouse:"script_abi"`
	ScriptBytecode             string   `clickhouse:"script_bytecode"`

	Events           []string `clickhouse:"events"            required:"true"`
	EventRawTypes    []string `clickhouse:"event_raw_type"    index:"idx_event_type/bloom_filter"`
	Changes          []string `clickhouse:"changes"           required:"true"`
	ChangeAddresses  []string `clickhouse:"change_addresses"  index:"idx_change_address/bloom_filter"`
	ResourceRawTypes []string `clickhouse:"resource_raw_type" index:"bloom_filter"`
	ResourceTypes    []string `clickhouse:"resource_type"     index:"bloom_filter"`

	EventCount            int64 `clickhouse:"event_count"`
	ChangeCount           int64 `clickhouse:"change_count"`
	ModuleChangesCount    int64 `clickhouse:"module_changes_count"`
	TableItemChangesCount int64 `clickhouse:"table_item_changes_count"`
	ResourceChangesCount  int64 `clickhouse:"resource_changes_count"`
}

func jsonMarshalToString(d any) (string, error) {
	b, err := json.Marshal(d)
	return string(b), err
}

func accountAddressToString(addr *aptosSdk.AccountAddress) *string {
	if addr == nil {
		return nil
	}
	s := addr.String()
	return &s
}

func stringToAccountAddress(s *string) *aptosSdk.AccountAddress {
	if s == nil {
		return nil
	}
	addr := aptosSdk.AccountAddress(common.HexToHash(*s))
	return &addr
}

func (t *Transaction) parsePayload(payload *api.TransactionPayload) (err error) {
	t.PayloadType = string(payload.Type)
	switch payload.Type {
	case api.TransactionPayloadVariantEntryFunction:
		f := payload.Inner.(*api.TransactionPayloadEntryFunction)
		t.EntryFunction = f.Function
		t.EntryFunctionTypeArguments = f.TypeArguments
		t.EntryFunctionArguments, err = utils.MapSlice(f.Arguments, jsonMarshalToString)
		if err != nil {
			return err
		}
	case api.TransactionPayloadVariantScript:
		f := payload.Inner.(*api.TransactionPayloadScript)
		t.EntryFunctionTypeArguments = f.TypeArguments
		t.EntryFunctionArguments, err = utils.MapSlice(f.Arguments, jsonMarshalToString)
		if err != nil {
			return err
		}
		t.ScriptBytecode = hexutil.Encode(f.Code.Bytecode)
		t.ScriptAbi, err = jsonMarshalToString(f.Code.Abi)
		if err != nil {
			return err
		}
	case api.TransactionPayloadVariantMultisig:
		f := payload.Inner.(*api.TransactionPayloadMultisig)
		if f.TransactionPayload != nil {
			t.MultiSignAddress = accountAddressToString(f.MultisigAddress)
			switch f.TransactionPayload.Type {
			case api.TransactionPayloadVariantEntryFunction:
				p := f.TransactionPayload.Inner.(*api.TransactionPayloadEntryFunction)
				t.EntryFunction = p.Function
				t.EntryFunctionTypeArguments = p.TypeArguments
				t.EntryFunctionArguments, err = utils.MapSlice(p.Arguments, jsonMarshalToString)
				if err != nil {
					return err
				}
			case api.TransactionPayloadVariantScript:
				p := f.TransactionPayload.Inner.(*api.TransactionPayloadScript)
				t.EntryFunctionTypeArguments = p.TypeArguments
				t.EntryFunctionArguments, err = utils.MapSlice(p.Arguments, jsonMarshalToString)
				if err != nil {
					return err
				}
				t.ScriptBytecode = hexutil.Encode(p.Code.Bytecode)
				t.ScriptAbi, err = jsonMarshalToString(p.Code.Abi)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (t *Transaction) fromRawTransaction(
	blockIndex BlockIndex,
	txIndex TransactionIndex,
	tx api.CommittedTransaction,
) (err error) {
	t.BlockIndex = blockIndex
	t.TransactionIndex = txIndex
	t.Type = string(tx.Type)

	var payload *api.TransactionPayload
	var events []*api.Event
	var changes []*api.WriteSetChange
	switch tx.Type {
	case api.TransactionVariantUser:
		userTx, _ := tx.UserTransaction()
		t.AccumulatorRootHash = userTx.AccumulatorRootHash
		t.StateChangeHash = userTx.StateChangeHash
		t.EventRootHash = userTx.EventRootHash
		t.GasUsed = userTx.GasUsed
		t.Success = userTx.Success
		t.VMStatus = userTx.VmStatus
		t.Sender = accountAddressToString(userTx.Sender)
		t.SequenceNumber = userTx.SequenceNumber
		t.MaxGasAmount = userTx.MaxGasAmount
		t.GasUnitPrice = userTx.GasUnitPrice
		t.ExpirationTimestampSecs = userTx.ExpirationTimestampSecs
		t.Timestamp = time.UnixMicro(int64(userTx.Timestamp))
		t.StateCheckpointHash = userTx.StateCheckpointHash
		if userTx.Signature != nil {
			t.Signature, err = jsonMarshalToString(userTx.Signature)
			if err != nil {
				return errors.Wrapf(err, "marshal signature of %s transaction %d failed", tx.Type, tx.Version())
			}
		}

		payload = userTx.Payload
		events = userTx.Events
		changes = userTx.Changes
	case api.TransactionVariantGenesis:
		genesisTx, _ := tx.GenesisTransaction()
		t.AccumulatorRootHash = genesisTx.AccumulatorRootHash
		t.StateChangeHash = genesisTx.StateChangeHash
		t.EventRootHash = genesisTx.EventRootHash
		t.GasUsed = genesisTx.GasUsed
		t.Success = genesisTx.Success
		t.VMStatus = genesisTx.VmStatus
		t.StateCheckpointHash = genesisTx.StateCheckpointHash

		payload = genesisTx.Payload
		events = genesisTx.Events
		changes = genesisTx.Changes
	case api.TransactionVariantBlockMetadata:
		blockMetadataTx, _ := tx.BlockMetadataTransaction()
		t.AccumulatorRootHash = blockMetadataTx.AccumulatorRootHash
		t.StateChangeHash = blockMetadataTx.StateChangeHash
		t.EventRootHash = blockMetadataTx.EventRootHash
		t.GasUsed = blockMetadataTx.GasUsed
		t.Success = blockMetadataTx.Success
		t.VMStatus = blockMetadataTx.VmStatus
		t.Timestamp = time.UnixMicro(int64(blockMetadataTx.Timestamp))
		t.StateCheckpointHash = blockMetadataTx.StateCheckpointHash

		events = blockMetadataTx.Events
		changes = blockMetadataTx.Changes
	case api.TransactionVariantBlockEpilogue:
		blockEpilogueTx, _ := tx.BlockEpilogueTransaction()
		t.AccumulatorRootHash = blockEpilogueTx.AccumulatorRootHash
		t.StateChangeHash = blockEpilogueTx.StateChangeHash
		t.EventRootHash = blockEpilogueTx.EventRootHash
		t.GasUsed = blockEpilogueTx.GasUsed
		t.Success = blockEpilogueTx.Success
		t.VMStatus = blockEpilogueTx.VmStatus
		t.Timestamp = time.UnixMicro(int64(blockEpilogueTx.Timestamp))
		t.StateCheckpointHash = blockEpilogueTx.StateCheckpointHash

		events = blockEpilogueTx.Events
		changes = blockEpilogueTx.Changes
	case api.TransactionVariantStateCheckpoint:
		stateCheckpointTx, _ := tx.StateCheckpointTransaction()
		t.AccumulatorRootHash = stateCheckpointTx.AccumulatorRootHash
		t.StateChangeHash = stateCheckpointTx.StateChangeHash
		t.EventRootHash = stateCheckpointTx.EventRootHash
		t.GasUsed = stateCheckpointTx.GasUsed
		t.Success = stateCheckpointTx.Success
		t.VMStatus = stateCheckpointTx.VmStatus
		t.Timestamp = time.UnixMicro(int64(stateCheckpointTx.Timestamp))
		t.StateCheckpointHash = stateCheckpointTx.StateCheckpointHash

		changes = stateCheckpointTx.Changes
	case api.TransactionVariantValidator:
		vTx, _ := tx.ValidatorTransaction()
		t.AccumulatorRootHash = vTx.AccumulatorRootHash
		t.StateChangeHash = vTx.StateChangeHash
		t.EventRootHash = vTx.EventRootHash
		t.GasUsed = vTx.GasUsed
		t.Success = vTx.Success
		t.VMStatus = vTx.VmStatus
		t.Timestamp = time.UnixMicro(int64(vTx.Timestamp))
		t.StateCheckpointHash = vTx.StateCheckpointHash

		events = vTx.Events
		changes = vTx.Changes
	case api.TransactionVariantUnknown:
		uTx, _ := tx.UnknownTransaction()
		if uTx.Payload != nil {
			t.Payload, err = jsonMarshalToString(uTx.Payload)
			if err != nil {
				return errors.Wrapf(err, "marshal payload of %s transaction %d failed", tx.Type, tx.Version())
			}
		}
	default:
		return errors.Errorf("unexpected %s transaction %d", tx.Type, tx.Version())
	}

	if payload != nil {
		t.Payload, err = jsonMarshalToString(payload)
		if err != nil {
			return errors.Wrapf(err, "marshal payload of %s transaction %d failed", tx.Type, tx.Version())
		}
		if err = t.parsePayload(payload); err != nil {
			return errors.Wrapf(err, "marshal payload of %s transaction %d failed", tx.Type, tx.Version())
		}
	}
	t.EventCount = int64(len(events))
	t.Events, err = utils.MapSlice(events, func(ev *api.Event) (string, error) {
		return jsonMarshalToString(ev)
	})
	if err != nil {
		return errors.Wrapf(err, "marshal events of %s transaction %d failed", tx.Type, tx.Version())
	}
	t.EventRawTypes = utils.MapSliceNoError(events, func(ev *api.Event) string {
		return move.RemoveTypeArgs(ev.Type)
	})

	t.ChangeCount = int64(len(changes))
	t.Changes, err = utils.MapSlice(changes, func(wc *api.WriteSetChange) (string, error) {
		return jsonMarshalToString(wc)
	})
	if err != nil {
		return errors.Wrapf(err, "marshal changes of %s transaction %d failed", tx.Type, tx.Version())
	}
	t.ChangeAddresses = utils.MapSliceNoError(changes, func(wc *api.WriteSetChange) string {
		addr := aptos.GetChangeAddress(wc)
		if addr == nil {
			return ""
		}
		return addr.String()
	})
	t.ResourceTypes = utils.MapSliceNoError(changes, func(wc *api.WriteSetChange) string {
		return utils.EmptyStringIfNil(aptos.GetChangeResourceType(wc))
	})
	t.ResourceRawTypes = utils.MapSliceNoError(t.ResourceTypes, move.RemoveTypeArgs)
	return nil
}

func (t *Transaction) toRawTransaction() (tx api.CommittedTransaction, err error) {
	o := api.CommittedTransaction{
		Type: api.TransactionVariant(t.Type),
	}

	// changes
	var changes []*api.WriteSetChange
	changes, err = utils.MapSlice(t.Changes, utils.StringUnmarshaler[*api.WriteSetChange])
	if err != nil {
		return
	}
	// events
	var events []*api.Event
	events, err = utils.MapSlice(t.Events, utils.StringUnmarshaler[*api.Event])
	if err != nil {
		return
	}
	// payload
	var payload *api.TransactionPayload
	if len(t.Payload) > 0 && (o.Type == api.TransactionVariantUser || o.Type == api.TransactionVariantGenesis) {
		var p api.TransactionPayload
		if err = json.Unmarshal([]byte(t.Payload), &p); err != nil {
			return
		}
		payload = &p
	}
	// signature
	var signature *api.Signature
	if len(t.Signature) > 0 {
		var s api.Signature
		if err = json.Unmarshal([]byte(t.Signature), &s); err != nil {
			return
		}
		signature = &s
	}
	// sender
	var sender *aptosSdk.AccountAddress
	if t.Sender != nil {
		sender = utils.WrapPointer(aptosSdk.AccountAddress(common.HexToHash(*t.Sender)))
	}
	// inner
	switch o.Type {
	case api.TransactionVariantUser:
		o.Inner = &api.UserTransaction{
			Version:                 t.TransactionVersion,
			Hash:                    t.TransactionHash,
			AccumulatorRootHash:     t.AccumulatorRootHash,
			StateChangeHash:         t.StateChangeHash,
			EventRootHash:           t.EventRootHash,
			GasUsed:                 t.GasUsed,
			Success:                 t.Success,
			VmStatus:                t.VMStatus,
			Changes:                 changes,
			Events:                  events,
			Sender:                  sender,
			SequenceNumber:          t.SequenceNumber,
			MaxGasAmount:            t.MaxGasAmount,
			GasUnitPrice:            t.GasUnitPrice,
			ExpirationTimestampSecs: t.ExpirationTimestampSecs,
			Payload:                 payload,
			Signature:               signature,
			Timestamp:               uint64(t.Timestamp.UnixMicro()),
			StateCheckpointHash:     t.StateCheckpointHash,
		}
	case api.TransactionVariantGenesis:
		o.Inner = &api.GenesisTransaction{
			Version:             t.TransactionVersion,
			Hash:                t.TransactionHash,
			AccumulatorRootHash: t.AccumulatorRootHash,
			StateChangeHash:     t.StateChangeHash,
			EventRootHash:       t.EventRootHash,
			GasUsed:             t.GasUsed,
			Success:             t.Success,
			VmStatus:            t.VMStatus,
			Changes:             changes,
			Events:              events,
			Payload:             payload,
			StateCheckpointHash: t.StateCheckpointHash,
		}
	case api.TransactionVariantBlockMetadata:
		o.Inner = &api.BlockMetadataTransaction{
			Version:             t.TransactionVersion,
			Hash:                t.TransactionHash,
			AccumulatorRootHash: t.AccumulatorRootHash,
			StateChangeHash:     t.StateChangeHash,
			EventRootHash:       t.EventRootHash,
			GasUsed:             t.GasUsed,
			Success:             t.Success,
			VmStatus:            t.VMStatus,
			Changes:             changes,
			Events:              events,
			Timestamp:           uint64(t.Timestamp.UnixMicro()),
			StateCheckpointHash: t.StateCheckpointHash,
		}
	case api.TransactionVariantBlockEpilogue:
		o.Inner = &api.BlockEpilogueTransaction{
			Version:             t.TransactionVersion,
			Hash:                t.TransactionHash,
			AccumulatorRootHash: t.AccumulatorRootHash,
			StateChangeHash:     t.StateChangeHash,
			EventRootHash:       t.EventRootHash,
			GasUsed:             t.GasUsed,
			Success:             t.Success,
			VmStatus:            t.VMStatus,
			Changes:             changes,
			Events:              events,
			Timestamp:           uint64(t.Timestamp.UnixMicro()),
			StateCheckpointHash: t.StateCheckpointHash,
			BlockEndInfo:        nil,
		}
	case api.TransactionVariantStateCheckpoint:
		o.Inner = &api.StateCheckpointTransaction{
			Version:             t.TransactionVersion,
			Hash:                t.TransactionHash,
			AccumulatorRootHash: t.AccumulatorRootHash,
			StateChangeHash:     t.StateChangeHash,
			EventRootHash:       t.EventRootHash,
			GasUsed:             t.GasUsed,
			Success:             t.Success,
			VmStatus:            t.VMStatus,
			Changes:             changes,
			Timestamp:           uint64(t.Timestamp.UnixMicro()),
			StateCheckpointHash: t.StateCheckpointHash,
		}
	case api.TransactionVariantValidator:
		o.Inner = &api.ValidatorTransaction{
			Version:             t.TransactionVersion,
			Hash:                t.TransactionHash,
			AccumulatorRootHash: t.AccumulatorRootHash,
			StateChangeHash:     t.StateChangeHash,
			EventRootHash:       t.EventRootHash,
			GasUsed:             t.GasUsed,
			Success:             t.Success,
			VmStatus:            t.VMStatus,
			Changes:             changes,
			Events:              events,
			Timestamp:           uint64(t.Timestamp.UnixMicro()),
			StateCheckpointHash: t.StateCheckpointHash,
		}
	default:
		inner := &api.UnknownTransaction{
			Type: string(o.Type),
		}
		if err = json.Unmarshal([]byte(t.Payload), &inner.Payload); err != nil {
			return
		}
		o.Inner = inner
	}

	return o, nil
}

type Event struct {
	BlockIndex
	TransactionIndex
	EventIndex         uint64  `clickhouse:"event_index"`
	Type               string  `clickhouse:"type"`
	GUIDAccountAddress *string `clickhouse:"guid_account_address" type:"Nullable(FixedString(66))"`
	GUIDCreateNumber   uint64  `clickhouse:"guid_create_number"`
	SequenceNumber     uint64  `clickhouse:"sequence_number"`
	Data               string  `clickhouse:"data"`
}

func (e *Event) fromRawEvent(blockIndex BlockIndex, txIndex TransactionIndex, evIndex uint64, ev api.Event) {
	e.BlockIndex = blockIndex
	e.TransactionIndex = txIndex
	e.EventIndex = evIndex
	e.Type = ev.Type
	if ev.Guid != nil {
		e.GUIDAccountAddress = accountAddressToString(ev.Guid.AccountAddress)
		e.GUIDCreateNumber = ev.Guid.CreationNumber
	}
	e.SequenceNumber = ev.SequenceNumber
	e.Data = string(ev.RawData)
}

func (e *Event) toRawEvent() api.Event {
	o := api.Event{
		Type: e.Type,
		Guid: &api.GUID{
			CreationNumber: e.GUIDCreateNumber,
			AccountAddress: stringToAccountAddress(e.GUIDAccountAddress),
		},
		SequenceNumber: e.SequenceNumber,
		RawData:        json.RawMessage(e.Data),
	}
	_ = json.Unmarshal(o.RawData, &o.Data)
	return o
}

type ChangeIndex struct {
	ChangeType  string `clickhouse:"type"`
	ChangeIndex uint64 `clickhouse:"change_index"`
}

type Change struct {
	BlockIndex
	TransactionIndex
	ChangeIndex
	StateKeyHash string  `clickhouse:"state_key_hash"`
	IsDeletion   bool    `clickhouse:"is_deletion"`
	Address      *string `clickhouse:"address" type:"Nullable(FixedString(66))" index:"bloom_filter"`
	Data         string  `clickhouse:"data"` // json of api.WriteSetChange.Inner, not include change type
}

func (c *Change) fromRawChange(
	blockIndex BlockIndex,
	txIndex TransactionIndex,
	changeIndex ChangeIndex,
	wc api.WriteSetChange,
) (err error) {
	c.BlockIndex = blockIndex
	c.TransactionIndex = txIndex
	c.ChangeIndex = changeIndex
	c.Data, err = jsonMarshalToString(wc.Inner)
	if err != nil {
		return errors.Wrapf(err, "unmarshal %d/%s change in transaction %d failed",
			changeIndex.ChangeIndex, changeIndex.ChangeType, txIndex.TransactionVersion)
	}
	switch wc.Type {
	case api.WriteSetChangeVariantWriteResource:
		wr := wc.Inner.(*api.WriteSetChangeWriteResource)
		c.StateKeyHash = wr.StateKeyHash
		c.Address = accountAddressToString(wr.Address)
	case api.WriteSetChangeVariantDeleteResource:
		wr := wc.Inner.(*api.WriteSetChangeDeleteResource)
		c.IsDeletion = true
		c.StateKeyHash = wr.StateKeyHash
		c.Address = accountAddressToString(wr.Address)
	case api.WriteSetChangeVariantWriteModule:
		wm := wc.Inner.(*api.WriteSetChangeWriteModule)
		c.StateKeyHash = wm.StateKeyHash
		c.Address = accountAddressToString(wm.Address)
	case api.WriteSetChangeVariantDeleteModule:
		wm := wc.Inner.(*api.WriteSetChangeDeleteModule)
		c.IsDeletion = true
		c.StateKeyHash = wm.StateKeyHash
		c.Address = accountAddressToString(wm.Address)
	case api.WriteSetChangeVariantWriteTableItem:
		wti := wc.Inner.(*api.WriteSetChangeWriteTableItem)
		c.StateKeyHash = wti.StateKeyHash
	case api.WriteSetChangeVariantDeleteTableItem:
		wti := wc.Inner.(*api.WriteSetChangeDeleteTableItem)
		c.IsDeletion = true
		c.StateKeyHash = wti.StateKeyHash
	}
	return nil
}

// only need c.ChangeType and c.Data
func (c *Change) toRawChange() (api.WriteSetChange, error) {
	o := api.WriteSetChange{
		Type: api.WriteSetChangeVariant(c.ChangeType),
	}
	switch o.Type {
	case api.WriteSetChangeVariantWriteResource:
		o.Inner = &api.WriteSetChangeWriteResource{}
	case api.WriteSetChangeVariantDeleteResource:
		o.Inner = &api.WriteSetChangeDeleteResource{}
	case api.WriteSetChangeVariantWriteModule:
		o.Inner = &api.WriteSetChangeWriteModule{}
	case api.WriteSetChangeVariantDeleteModule:
		o.Inner = &api.WriteSetChangeDeleteModule{}
	case api.WriteSetChangeVariantWriteTableItem:
		o.Inner = &api.WriteSetChangeWriteTableItem{}
	case api.WriteSetChangeVariantDeleteTableItem:
		o.Inner = &api.WriteSetChangeDeleteTableItem{}
	default:
		o.Inner = &api.WriteSetChangeUnknown{Type: string(o.Type)}
		o.Type = api.WriteSetChangeVariantUnknown
		err := json.Unmarshal([]byte(c.Data), &o.Inner.(*api.WriteSetChangeUnknown).Payload)
		return o, err
	}
	err := json.Unmarshal([]byte(c.Data), o.Inner)
	return o, err
}

type Module struct {
	BlockIndex
	TransactionIndex
	ChangeIndex
	Address    *string `clickhouse:"address"     index:"idx_module_address/bloom_filter" type:"Nullable(FixedString(66))"`
	ModuleName string  `clickhouse:"module_name" index:"bloom_filter"`
	Bytecode   string  `clickhouse:"module_bytecode"`
	ABI        string  `clickhouse:"abi"`
	IsDelete   bool    `clickhouse:"is_delete"`
}

func (m *Module) fromRawChange(
	blockIndex BlockIndex,
	txIndex TransactionIndex,
	changeIndex ChangeIndex,
	wc api.WriteSetChange,
) bool {
	switch wc.Type {
	case api.WriteSetChangeVariantWriteModule:
		wm := wc.Inner.(*api.WriteSetChangeWriteModule)
		m.Address = accountAddressToString(wm.Address)
		if wm.Data != nil {
			if wm.Data.Abi != nil {
				m.ModuleName = wm.Data.Abi.Name
			}
			m.Bytecode = string(wm.Data.Bytecode)
			m.ABI, _ = jsonMarshalToString(wm.Data.Abi)
		}
	case api.WriteSetChangeVariantDeleteModule:
		wm := wc.Inner.(*api.WriteSetChangeDeleteModule)
		m.Address = accountAddressToString(wm.Address)
		m.ModuleName = wm.Module
		m.IsDelete = true
	default:
		return false
	}
	m.BlockIndex = blockIndex
	m.TransactionIndex = txIndex
	m.ChangeIndex = changeIndex
	return true
}

type Resource struct {
	BlockIndex
	TransactionIndex
	ChangeIndex
	Address  *string `clickhouse:"address" type:"Nullable(FixedString(66))"`
	Type     string  `clickhouse:"resource_type" index:"bloom_filter"`
	Data     string  `clickhouse:"resource_data"`
	IsDelete bool    `clickhouse:"is_delete"`
}

func (r *Resource) fromRawChange(
	blockIndex BlockIndex,
	txIndex TransactionIndex,
	changeIndex ChangeIndex,
	wc api.WriteSetChange,
) bool {
	switch wc.Type {
	case api.WriteSetChangeVariantWriteResource:
		wr := wc.Inner.(*api.WriteSetChangeWriteResource)
		r.Address = accountAddressToString(wr.Address)
		r.Type = move.TrimTypeString(wr.Data.Type)
		r.Data, _ = jsonMarshalToString(wr.Data.Data)
	case api.WriteSetChangeVariantDeleteResource:
		wr := wc.Inner.(*api.WriteSetChangeDeleteResource)
		r.Address = accountAddressToString(wr.Address)
		r.Type = move.TrimTypeString(wr.Resource)
		r.IsDelete = true
	default:
		return false
	}
	r.BlockIndex = blockIndex
	r.TransactionIndex = txIndex
	r.ChangeIndex = changeIndex
	return true
}

type TableItem struct {
	BlockIndex
	TransactionIndex
	ChangeIndex
	Handle string `clickhouse:"table_item_handle"`
	Key    string `clickhouse:"table_item_key"`
	Value  string `clickhouse:"table_item_value"`
	Data   string `clickhouse:"table_item_data"`
}

func (t *TableItem) fromRawChange(
	blockIndex BlockIndex,
	txIndex TransactionIndex,
	changeIndex ChangeIndex,
	wc api.WriteSetChange,
) bool {
	switch wc.Type {
	case api.WriteSetChangeVariantWriteTableItem:
		wti := wc.Inner.(*api.WriteSetChangeWriteTableItem)
		t.Handle = wti.Handle
		t.Key = wti.Key
		t.Value = wti.Value
		t.Data, _ = jsonMarshalToString(wti.Data)
	case api.WriteSetChangeVariantDeleteTableItem:
		wti := wc.Inner.(*api.WriteSetChangeDeleteTableItem)
		t.Handle = wti.Handle
		t.Key = wti.Key
		t.Data, _ = jsonMarshalToString(wti.Data)
	default:
		return false
	}
	t.BlockIndex = blockIndex
	t.TransactionIndex = txIndex
	t.ChangeIndex = changeIndex
	return true
}
