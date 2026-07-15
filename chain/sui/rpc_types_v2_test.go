package sui

import (
	"encoding/json"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func Test_ObjectChangeFilter(t *testing.T) {
	f := ObjectChangeFilter{
		TypePattern: move.TypeSet{move.MustBuildType("0x1::"), move.MustBuildType("0x1111::")},
		OwnerFilter: &ObjectChangeOwnerFilter{OwnerID: []string{"0x2", "0x2222"}},
		ObjectIDIn:  set.New("aa"),
	}
	b, err := json.Marshal(f)
	assert.NoError(t, err)
	assert.Equal(t, `{"type_pattern":["0x1::*::*","0x1111::*::*"],"owner_filter":{"owner_id":["0x2","0x2222"]},"object_id_in":["aa"]}`, string(b))

	var r ObjectChangeFilter
	assert.NoError(t, json.Unmarshal(b, &r))
	assert.Equal(t, f, r)

	assert.NoError(t, json.Unmarshal([]byte(`{"object_id_in":["bb",""]}`), &f))
	assert.Equal(t, 2, f.ObjectIDIn.Size())
	assert.True(t, f.ObjectIDIn.Contains("bb"))
	assert.True(t, f.ObjectIDIn.Contains(""))
	assert.Nil(t, f.TypePattern)
	assert.Nil(t, f.OwnerFilter)

	assert.NoError(t, json.Unmarshal([]byte(`{}`), &f))
	assert.Equal(t, 0, f.ObjectIDIn.Size())
	assert.Nil(t, f.TypePattern)
	assert.Nil(t, f.OwnerFilter)
}

func makeSuccessTx(sender string) types.TransactionResponseV1 {
	senderAddr := types.StrToAddressMust(sender)
	return types.TransactionResponseV1{
		Effects: &types.TransactionEffectsV1{
			Status: types.TransactionStatus{Status: types.TransactionStatusSuccess},
		},
		Transaction: &types.SenderSignedTransaction{
			Data: &types.TransactionData{
				V1: &types.TransactionDataV1{
					Sender: senderAddr,
					Kind: &types.TransactionKind{
						ProgrammableTransaction: &types.ProgrammableTransaction{},
					},
				},
			},
		},
	}
}

func makeFailedTx() types.TransactionResponseV1 {
	return types.TransactionResponseV1{
		Effects: &types.TransactionEffectsV1{
			Status: types.TransactionStatus{Status: types.TransactionStatusFailure},
		},
	}
}

func Test_ObjectChangeOwnerFilter_Checker(t *testing.T) {
	objID := types.StrToObjectIDMust("0x0001")
	ownerID := types.StrToObjectIDMust("0x0002")

	ocWithID := types.ObjectChangeExtend{
		ObjectChange: types.ObjectChange{ObjectID: &objID},
	}
	ocOwnedByOwner := types.ObjectChangeExtend{
		ObjectChange: types.ObjectChange{
			ObjectID: &objID,
			Owner:    types.BuildObjectOwner(ownerID.String(), types.OwnerTypeObject, 0),
		},
	}
	ocAddressOwner := types.ObjectChangeExtend{
		ObjectChange: types.ObjectChange{
			ObjectID: &objID,
			Owner:    types.BuildObjectOwner(ownerID.String(), types.OwnerTypeAddress, 0),
		},
	}

	// Empty filter returns false for any object
	emptyFilter := ObjectChangeOwnerFilter{}
	assert.False(t, emptyFilter.Checker()(ocWithID))

	// Match by object ID directly
	otherID := types.StrToObjectIDMust("0x9999")
	ocOtherID := types.ObjectChangeExtend{ObjectChange: types.ObjectChange{ObjectID: &otherID}}
	f := ObjectChangeOwnerFilter{OwnerID: []string{objID.String()}}
	assert.True(t, f.Checker()(ocWithID))
	assert.False(t, f.Checker()(ocOtherID))

	// Match by owner object
	f2 := ObjectChangeOwnerFilter{
		OwnerID:   []string{ownerID.String()},
		OwnerType: []string{types.OwnerTypeObject},
	}
	assert.True(t, f2.Checker()(ocOwnedByOwner))
	assert.False(t, f2.Checker()(ocAddressOwner)) // wrong owner type

	// Merge deduplicates
	a := ObjectChangeOwnerFilter{OwnerID: []string{"0x1", "0x2"}, OwnerType: []string{"object"}}
	b := ObjectChangeOwnerFilter{OwnerID: []string{"0x2", "0x3"}, OwnerType: []string{"address"}}
	merged := a.Merge(b)
	assert.Len(t, merged.OwnerID, 3)
	assert.Len(t, merged.OwnerType, 2)
}

func Test_ObjectChangeOwnerFilter_CheckerGrpc(t *testing.T) {
	objID := types.StrToObjectIDMust("0x0001").String()
	ownerID := types.StrToObjectIDMust("0x0002").String()

	changed := func(id string, kind rpcv2.Owner_OwnerKind, ownerAddr string) *rpcv2.ChangedObject {
		return &rpcv2.ChangedObject{
			ObjectId:    &id,
			OutputOwner: &rpcv2.Owner{Kind: &kind, Address: &ownerAddr},
		}
	}

	// Empty filter matches nothing
	assert.False(t, ObjectChangeOwnerFilter{}.CheckerGrpc()(changed(objID, rpcv2.Owner_OBJECT, ownerID)))

	// Match by object ID directly
	f := ObjectChangeOwnerFilter{OwnerID: []string{objID}}
	assert.True(t, f.CheckerGrpc()(changed(objID, rpcv2.Owner_ADDRESS, ownerID)))

	// The grpc contract (FilterGrpcChangedObjects) carries Owner_OwnerKind enum
	// names in OwnerType; a filter built with the json-rpc lowercase strings is
	// translated via ToGrpc before reaching the grpc interfaces.
	f2 := ObjectChangeOwnerFilter{
		OwnerID:   []string{ownerID},
		OwnerType: []string{types.OwnerTypeObject},
	}.ToGrpc()
	assert.Equal(t, []string{"OBJECT"}, f2.OwnerType)
	assert.True(t, f2.CheckerGrpc()(changed(objID, rpcv2.Owner_OBJECT, ownerID)))
	assert.False(t, f2.CheckerGrpc()(changed(objID, rpcv2.Owner_ADDRESS, ownerID))) // wrong owner type
	assert.False(t, f2.CheckerGrpc()(changed(objID, rpcv2.Owner_OBJECT, objID)))    // wrong owner id

	// ToGrpc is idempotent: enum-form values pass through unchanged
	assert.Equal(t, f2.OwnerType, f2.ToGrpc().OwnerType)

	// Input owner matches too
	inKind := rpcv2.Owner_OBJECT
	ocInput := &rpcv2.ChangedObject{
		ObjectId:   &objID,
		InputOwner: &rpcv2.Owner{Kind: &inKind, Address: &ownerID},
	}
	assert.True(t, f2.CheckerGrpc()(ocInput))

	// Address owner type
	f3 := ObjectChangeOwnerFilter{
		OwnerID:   []string{ownerID},
		OwnerType: []string{types.OwnerTypeAddress},
	}.ToGrpc()
	assert.True(t, f3.CheckerGrpc()(changed(objID, rpcv2.Owner_ADDRESS, ownerID)))
	assert.False(t, f3.CheckerGrpc()(changed(objID, rpcv2.Owner_OBJECT, ownerID)))

	// ObjectChangeFilter.ToGrpc translates the nested owner filter
	cf := ObjectChangeFilter{OwnerFilter: &ObjectChangeOwnerFilter{
		OwnerID:   []string{ownerID},
		OwnerType: []string{types.OwnerTypeObject},
	}}.ToGrpc()
	assert.Equal(t, []string{"OBJECT"}, cf.OwnerFilter.OwnerType)
}

func Test_FunctionFilter_CheckGrpcTx_Kind(t *testing.T) {
	tx := func(kind rpcv2.TransactionKind_Kind) *rpcv2.ExecutedTransaction {
		success := true
		return &rpcv2.ExecutedTransaction{
			Transaction: &rpcv2.Transaction{Kind: &rpcv2.TransactionKind{Kind: &kind}},
			Effects:     &rpcv2.TransactionEffects{Status: &rpcv2.ExecutionStatus{Success: &success}},
		}
	}

	// The grpc contract (GetGrpcTransactions) carries TransactionKind_Kind enum
	// names in FunctionFilter.Kind; a filter built with the json-rpc kind name
	// (types.TransactionKind.Kind(), what the move-call agents set) is
	// translated via ToGrpc. CheckGrpcTx compares against the kind enum, not
	// the stringified TransactionKind message.
	f := FunctionFilter{Kind: utils.WrapPointer("ProgrammableTransaction")}.ToGrpc()
	assert.Equal(t, "PROGRAMMABLE_TRANSACTION", *f.Kind)
	assert.True(t, f.CheckGrpcTx(tx(rpcv2.TransactionKind_PROGRAMMABLE_TRANSACTION)))
	assert.False(t, f.CheckGrpcTx(tx(rpcv2.TransactionKind_CHANGE_EPOCH)))

	// ToGrpc is idempotent and TransactionFilter.ToGrpc translates each entry
	tf := TransactionFilter{FunctionFilters: []FunctionFilter{
		{Kind: utils.WrapPointer("EndOfEpochTransaction")},
	}}.ToGrpc()
	assert.Equal(t, "END_OF_EPOCH", *tf.FunctionFilters[0].Kind)
	assert.Equal(t, "END_OF_EPOCH", *tf.ToGrpc().FunctionFilters[0].Kind)
	assert.True(t, tf.FunctionFilters[0].CheckGrpcTx(tx(rpcv2.TransactionKind_END_OF_EPOCH)))
}

func Test_ObjectChangeFilter_IsEmpty(t *testing.T) {
	assert.True(t, ObjectChangeFilter{}.IsEmpty())
	assert.True(t, ObjectChangeFilter{ObjectIDIn: set.New[string]()}.IsEmpty())
	assert.False(t, ObjectChangeFilter{ObjectIDIn: set.New("0x1")}.IsEmpty())
	assert.False(t, ObjectChangeFilter{TypePattern: move.TypeSet{move.MustBuildType("0x1::")}}.IsEmpty())
	assert.False(t, ObjectChangeFilter{OwnerFilter: &ObjectChangeOwnerFilter{}}.IsEmpty())
}

func Test_ObjectChangeFilter_Checker(t *testing.T) {
	objID := types.StrToObjectIDMust("0xABCD")
	coinType := types.TypeTagFromStringMust("0x2::sui::SUI")
	oc := types.ObjectChangeExtend{
		ObjectChange: types.ObjectChange{
			ObjectID:   &objID,
			ObjectType: &coinType,
		},
	}

	// Match by TypePattern
	fType := ObjectChangeFilter{TypePattern: move.TypeSet{move.MustBuildType("0x2::sui::SUI")}}
	assert.True(t, fType.Checker()(oc))

	fTypeNoMatch := ObjectChangeFilter{TypePattern: move.TypeSet{move.MustBuildType("0x3::")}}
	assert.False(t, fTypeNoMatch.Checker()(oc))

	// Match by ObjectIDIn
	fID := ObjectChangeFilter{ObjectIDIn: set.New(objID.String())}
	assert.True(t, fID.Checker()(oc))
	fIDNoMatch := ObjectChangeFilter{ObjectIDIn: set.New("0x9999")}
	assert.False(t, fIDNoMatch.Checker()(oc))

	// Match by OwnerFilter
	ownerID := types.StrToObjectIDMust("0x5555")
	ocWithOwner := types.ObjectChangeExtend{
		ObjectChange: types.ObjectChange{
			ObjectID: &objID,
			Owner:    types.BuildObjectOwner(ownerID.String(), types.OwnerTypeAddress, 0),
		},
	}
	fOwner := ObjectChangeFilter{OwnerFilter: &ObjectChangeOwnerFilter{
		OwnerID:   []string{ownerID.String()},
		OwnerType: []string{types.OwnerTypeAddress},
	}}
	assert.True(t, fOwner.Checker()(ocWithOwner))
	assert.False(t, fOwner.Checker()(oc))
}

func Test_ObjectChangeFilter_Merge(t *testing.T) {
	a := ObjectChangeFilter{
		TypePattern: move.TypeSet{move.MustBuildType("0x1::")},
		ObjectIDIn:  set.New("0xAAA"),
	}
	b := ObjectChangeFilter{
		TypePattern: move.TypeSet{move.MustBuildType("0x2::")},
		ObjectIDIn:  set.New("0xBBB"),
		OwnerFilter: &ObjectChangeOwnerFilter{OwnerID: []string{"0x1"}},
	}
	r := a.Merge(b)
	assert.Len(t, r.TypePattern, 2)
	assert.Equal(t, 2, r.ObjectIDIn.Size())
	assert.NotNil(t, r.OwnerFilter)

	// nil OwnerFilter on both sides
	r2 := a.Merge(ObjectChangeFilter{})
	assert.Equal(t, a.TypePattern, r2.TypePattern)
	assert.Nil(t, r2.OwnerFilter)
}

func Test_CommandFilter_CheckCommand(t *testing.T) {
	pkg := types.StrToObjectIDMust("0x0002")
	cmd := types.Command{
		MoveCall: &types.MoveCall{
			Package:  pkg,
			Module:   "coin",
			Function: "transfer",
		},
	}
	nonMoveCmd := types.Command{}

	// nil filter matches everything
	var nilFilter *CommandFilter
	assert.True(t, nilFilter.CheckCommand(cmd))
	assert.True(t, nilFilter.CheckCommand(nonMoveCmd))

	pkgStr := pkg.String()
	modStr := "coin"
	fnStr := "transfer"

	// all fields match
	f := &CommandFilter{CallPackage: &pkgStr, CallModule: &modStr, CallFunction: &fnStr}
	assert.True(t, f.CheckCommand(cmd))
	assert.False(t, f.CheckCommand(nonMoveCmd))

	// wrong module
	wrongMod := "token"
	fWrong := &CommandFilter{CallModule: &wrongMod}
	assert.False(t, fWrong.CheckCommand(cmd))

	// only package filter
	fPkg := &CommandFilter{CallPackage: &pkgStr}
	assert.True(t, fPkg.CheckCommand(cmd))
}

func Test_CommandFilter_Equal(t *testing.T) {
	p1 := "0x1"
	p2 := "0x2"
	f1 := &CommandFilter{CallPackage: &p1}
	f2 := &CommandFilter{CallPackage: &p1}
	f3 := &CommandFilter{CallPackage: &p2}
	var nilF *CommandFilter

	assert.True(t, f1.Equal(f2))
	assert.False(t, f1.Equal(f3))
	assert.False(t, f1.Equal(nilF))
	assert.True(t, nilF.Equal(nil))
}

func Test_CommandFilter_IsEmpty(t *testing.T) {
	var nilF *CommandFilter
	assert.True(t, nilF.IsEmpty())
	assert.True(t, (&CommandFilter{}).IsEmpty())
	p := "0x1"
	assert.False(t, (&CommandFilter{CallPackage: &p}).IsEmpty())
}

func Test_FunctionFilter_Check(t *testing.T) {
	pkg := types.StrToObjectIDMust("0x0002")
	pkgStr := pkg.String()
	senderStr := "0x0000000000000000000000000000000000000000000000000000000000000001"
	receiverStr := senderStr

	makeTxWithCall := func(sender string, cmdPkg types.ObjectID, mod, fn string) types.TransactionResponseV1 {
		tx := makeSuccessTx(sender)
		tx.Transaction.Data.V1.Kind.ProgrammableTransaction.Commands = []types.Command{
			{MoveCall: &types.MoveCall{Package: cmdPkg, Module: mod, Function: fn}},
		}
		return tx
	}

	tx := makeTxWithCall(senderStr, pkg, "coin", "transfer")

	// empty filter matches successful tx
	assert.True(t, FunctionFilter{}.Check(tx))

	// kind filter
	kindStr := "ProgrammableTransaction"
	assert.True(t, FunctionFilter{Kind: &kindStr}.Check(tx))
	wrongKind := "ChangeEpoch"
	assert.False(t, FunctionFilter{Kind: &wrongKind}.Check(tx))

	// command filter
	assert.True(t, FunctionFilter{CommandFilter: &CommandFilter{CallPackage: &pkgStr}}.Check(tx))
	wrongPkg := "0x9999"
	assert.False(t, FunctionFilter{CommandFilter: &CommandFilter{CallPackage: &wrongPkg}}.Check(tx))

	// sender filter
	assert.True(t, FunctionFilter{Sender: &senderStr}.Check(tx))
	wrongSender := "0xDEAD"
	assert.False(t, FunctionFilter{Sender: &wrongSender}.Check(tx))

	// receiver filter
	receiverAddr := types.StrToAddressMust(receiverStr)
	tx.BalanceChanges = []types.BalanceChange{
		{Owner: types.BuildObjectOwner(receiverAddr.String(), types.OwnerTypeAddress, 0)},
	}
	assert.True(t, FunctionFilter{Receiver: &receiverStr}.Check(tx))

	// failed tx
	failedTx := makeFailedTx()
	assert.False(t, FunctionFilter{}.Check(failedTx))
	assert.True(t, FunctionFilter{FailedIsOK: true}.Check(failedTx))
}

func Test_FunctionFilter_IsEmpty(t *testing.T) {
	// IsEmpty requires FailedIsOK=true and all other fields nil
	assert.True(t, FunctionFilter{FailedIsOK: true}.IsEmpty())
	// FailedIsOK=false means only successful txns pass — that's a non-trivial constraint
	assert.False(t, FunctionFilter{FailedIsOK: false}.IsEmpty())
	p := "0x1"
	assert.False(t, FunctionFilter{Sender: &p, FailedIsOK: true}.IsEmpty())
}

func Test_FunctionFilter_Equal(t *testing.T) {
	p1 := "0x1"
	p2 := "0x2"
	f1 := FunctionFilter{Sender: &p1}
	f2 := FunctionFilter{Sender: &p1}
	f3 := FunctionFilter{Sender: &p2}
	assert.True(t, f1.Equal(f2))
	assert.False(t, f1.Equal(f3))
}

func Test_EventFilterV2_CheckEvent(t *testing.T) {
	sender := "0x0000000000000000000000000000000000000000000000000000000000001234"
	ev := types.Event{
		Sender: sender,
		Type:   types.TypeTagFromStringMust("0x2::sui::SUI"),
	}

	// no filter matches anything
	assert.True(t, EventFilterV2{}.CheckEvent(ev))

	// sender filter
	assert.True(t, EventFilterV2{Sender: &sender}.CheckEvent(ev))
	wrongSender := "0x0000000000000000000000000000000000000000000000000000000000009999"
	assert.False(t, EventFilterV2{Sender: &wrongSender}.CheckEvent(ev))

	// type pattern filter
	assert.True(t, EventFilterV2{TypePattern: move.TypeSet{move.MustBuildType("0x2::sui::SUI")}}.CheckEvent(ev))
	assert.False(t, EventFilterV2{TypePattern: move.TypeSet{move.MustBuildType("0x3::")}}.CheckEvent(ev))
}

func Test_EventFilterV2_Check(t *testing.T) {
	sender := "0x0000000000000000000000000000000000000000000000000000000000001234"
	tx := makeSuccessTx(sender)
	tx.Events = []types.Event{
		{Sender: sender, Type: types.TypeTagFromStringMust("0x2::sui::SUI")},
	}

	f := EventFilterV2{Sender: &sender}
	assert.True(t, f.Check(tx))

	txNoEvents := makeSuccessTx(sender)
	assert.False(t, f.Check(txNoEvents))
}

func Test_EventFilterV2_Equal(t *testing.T) {
	s1 := "0x1"
	s2 := "0x2"
	f1 := EventFilterV2{Sender: &s1}
	f2 := EventFilterV2{Sender: &s1}
	f3 := EventFilterV2{Sender: &s2}
	assert.True(t, f1.Equal(f2))
	assert.False(t, f1.Equal(f3))
}

func Test_BuildEventChecker(t *testing.T) {
	s1 := "0xSENDER1"
	s2 := "0xSENDER2"
	checker := BuildEventChecker([]EventFilterV2{
		{Sender: &s1},
		{TypePattern: move.TypeSet{move.MustBuildType("0x2::sui::SUI")}},
	})

	assert.True(t, checker(types.Event{Sender: s1}))
	assert.True(t, checker(types.Event{Sender: s2, Type: types.TypeTagFromStringMust("0x2::sui::SUI")}))
	assert.False(t, checker(types.Event{Sender: s2, Type: types.TypeTagFromStringMust("0x3::token::T")}))
}

func Test_TransactionFilter_Check(t *testing.T) {
	sender := "0x0000000000000000000000000000000000000000000000000000000000000001"
	tx := makeSuccessTx(sender)
	tx.Events = []types.Event{
		{Sender: sender, Type: types.TypeTagFromStringMust("0x2::sui::SUI")},
	}
	failedTx := makeFailedTx()

	// failed tx rejected by default
	f := TransactionFilter{
		FunctionFilters: []FunctionFilter{{Sender: &sender}},
	}
	assert.False(t, f.Check(failedTx))
	assert.True(t, f.Check(tx))

	// FailedIsOK passes failed tx through to inner filters
	fFailed := TransactionFilter{
		FailedIsOK:      true,
		FunctionFilters: []FunctionFilter{{FailedIsOK: true}},
	}
	assert.True(t, fFailed.Check(failedTx))

	// EventFilter path
	evSender := sender
	fEvent := TransactionFilter{
		EventFilters: []EventFilterV2{{Sender: &evSender}},
	}
	assert.True(t, fEvent.Check(tx))
}

func Test_TransactionFilter_Merge(t *testing.T) {
	s1 := "0x1"
	s2 := "0x2"
	f1 := TransactionFilter{
		FunctionFilters: []FunctionFilter{{Sender: &s1}},
		EventFilters:    []EventFilterV2{{Sender: &s1}},
		FailedIsOK:      false,
	}
	f2 := TransactionFilter{
		FunctionFilters: []FunctionFilter{{Sender: &s1}, {Sender: &s2}}, // s1 is duplicate
		EventFilters:    []EventFilterV2{{Sender: &s2}},
		FailedIsOK:      true,
	}
	merged := f1.Merge(f2)
	assert.Len(t, merged.FunctionFilters, 2) // s1 deduplicated
	assert.Len(t, merged.EventFilters, 2)    // s1 and s2
	assert.True(t, merged.FailedIsOK)        // OR of both
}

func Test_TransactionFetchConfig_String(t *testing.T) {
	f := TransactionFetchConfig{NeedInputs: true, NeedEffects: false, NeedAllEvents: true}
	assert.Equal(t, "NeedInputs:true,NeedEffects:false,NeedAllEvents:true", f.String())
}

func Test_TransactionFetchConfig_Merge(t *testing.T) {
	a := TransactionFetchConfig{NeedInputs: true}
	b := TransactionFetchConfig{NeedEffects: true, NeedAllEvents: true}
	r := a.Merge(b)
	assert.True(t, r.NeedInputs)
	assert.True(t, r.NeedEffects)
	assert.True(t, r.NeedAllEvents)
}

func Test_TransactionFetchConfig_PruneTransaction(t *testing.T) {
	sender := "0x0000000000000000000000000000000000000000000000000000000000001234"
	evSender := sender
	tx := makeSuccessTx(sender)
	tx.Events = []types.Event{
		{Sender: sender, Type: types.TypeTagFromStringMust("0x2::sui::SUI")},
		{Sender: "0x0000000000000000000000000000000000000000000000000000000000009999", Type: types.TypeTagFromStringMust("0x3::token::T")},
	}

	eventFilters := []EventFilterV2{{Sender: &evSender}}

	// NeedAllEvents=false: events filtered by eventFilters
	fNoAll := TransactionFetchConfig{NeedEffects: true}
	pruned := fNoAll.PruneTransaction(tx, eventFilters)
	assert.Len(t, pruned.Events, 1)
	assert.Equal(t, sender, pruned.Events[0].Sender)
	assert.Nil(t, pruned.Transaction) // NeedInputs=false

	// NeedAllEvents=true: all events kept
	fAll := TransactionFetchConfig{NeedAllEvents: true, NeedEffects: true}
	prunedAll := fAll.PruneTransaction(tx, eventFilters)
	assert.Len(t, prunedAll.Events, 2)

	// NeedEffects=false: effects pruned to minimal fields
	fNoEffects := TransactionFetchConfig{NeedAllEvents: true}
	prunedNoEffects := fNoEffects.PruneTransaction(tx, eventFilters)
	assert.Equal(t, types.TransactionStatusSuccess, prunedNoEffects.Effects.Status.Status)
	assert.Nil(t, prunedNoEffects.Effects.GasUsed) // not preserved

	// NeedInputs=true: transaction preserved
	fWithInputs := TransactionFetchConfig{NeedInputs: true, NeedEffects: true, NeedAllEvents: true}
	prunedWithInputs := fWithInputs.PruneTransaction(tx, eventFilters)
	assert.NotNil(t, prunedWithInputs.Transaction)
}

func Test_GetLatestSimpleCheckpointResponse_CheckAPIVersion(t *testing.T) {
	same := GetLatestSimpleCheckpointResponse{APIVersion: APIVersion}
	assert.NoError(t, same.CheckAPIVersion())

	lower := GetLatestSimpleCheckpointResponse{APIVersion: APIVersion - 1}
	assert.NoError(t, lower.CheckAPIVersion())

	higher := GetLatestSimpleCheckpointResponse{APIVersion: APIVersion + 1}
	assert.Error(t, higher.CheckAPIVersion())
	assert.Contains(t, higher.CheckAPIVersion().Error(), "greater than")
}

func Test_panicThenPanic(t *testing.T) {
	fn := func() {
		defer func() {
			if panicErr := recover(); panicErr != nil {
				if err, is := panicErr.(error); is {
					panic(errors.Wrapf(err, "level2"))
				}
				panic(errors.Errorf("level2: %v", panicErr))
			}
		}()
		panic(errors.Errorf("level1"))
	}
	fn2 := func() (err error) {
		defer func() {
			if pe := recover(); pe != nil {
				var is bool
				if err, is = pe.(error); !is {
					err = errors.Errorf("got panic: %v", pe)
				}
			}
		}()
		fn()
		return nil
	}
	err := fn2()
	assert.NotNil(t, err)
	assert.Equal(t, "level2: level1", err.Error())
	log.Errorfe(err, "final error")
}
