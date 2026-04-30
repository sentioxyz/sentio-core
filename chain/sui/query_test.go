package sui

import (
	"fmt"
	"github.com/goccy/go-json"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func TestCallFilter(t *testing.T) {
	packageID, err := types.StrToObjectID("0x4c10b61966a34d3bb5c8a8f063e6b7445fc41f93")
	if err != nil {
		panic(err)
	}
	packageID2, err := types.StrToObjectID("0x74af446d037ea7a8ec345d2bab86be1d0d8e417c")
	if err != nil {
		panic(err)
	}
	call := types.MoveCall{
		Package:  packageID,
		Module:   "mod1",
		Function: "foo",
	}

	filter := &MoveCallFilter{
		Package: &packageID2,
	}
	assert.False(t, filter.Check(call))
	filter.Package = &packageID
	assert.True(t, filter.Check(call))
	filter.Module = "mod2"
	assert.False(t, filter.Check(call))
	filter.Module = "mod1"
	assert.True(t, filter.Check(call))
	filter.Function = "bar"
	assert.False(t, filter.Check(call))
	filter.Function = "foo"
	assert.True(t, filter.Check(call))
}

func TestTransactionFilter(t *testing.T) {
	txJSON := `{
		"checkpoint": 100000,
		"checkpoint_timestamp_ms": 1680136989850,
		"digest": "3CbzFiKmzfZ8e3XjBxsFbZaKrdkdkYuEytXkyXRZJ5Mw",
		"transaction": {
			"data": {
				"messageVersion": "v1",
				"transaction": {
					"kind": "ProgrammableTransaction",
					"inputs": [
						{
							"type": "pure",
							"valueType": "u64",
							"value": null
						},
						{
							"type": "object",
							"objectType": "sharedObject",
							"objectId": "0x0000000000000000000000000000000000000000000000000000000000000005",
							"initialSharedVersion": 1,
							"mutable": true
						},
						{
							"type": "pure",
							"valueType": "address",
							"value": null
						}
					],
					"transactions": [
						{
							"SplitCoins": [
								"GasCoin",
								[
									{
										"Input": 0
									}
								]
							]
						},
						{
							"MoveCall": {
								"package": "0x0000000000000000000000000000000000000000000000000000000000000003",
								"module": "sui_system",
								"function": "request_add_stake",
								"arguments": [
									{
										"Input": 1
									},
									{
										"Result": 0
									},
									{
										"Input": 2
									}
								]
							}
						}
					]
				},
				"sender": "0xab88bd54d960499aa53bbc5810e8a2bce681a016324c4fd3c57e20d1611a541f",
				"gasData": {
					"payment": [
						{
							"objectId": "0x1f3b54ded3e8bcea7b79e4e7915d46cea8ae1c0982d7a737d6d51c4e949418f6",
							"version": 7310,
							"digest": "2wJiQ3V9nTWek7rmLW8MuFpD1PywJXW6bX9EfcL1bTWi"
						}
					],
					"owner": "0xab88bd54d960499aa53bbc5810e8a2bce681a016324c4fd3c57e20d1611a541f",
					"price": 1000,
					"budget": 10031000
				}
			},
			"txSignatures": [
				"AAvNlD7nsDhbRB05rn/zYCC42IPD178pTFWNk+lGD2+NK3hPsDyjDi7eVsLxNE+mGZddCQDhUE8r9m+gikK0XwHAIKbw8tfTEzCrY79ASNyPpmjGaXg57656yJ6qjnvgbQ=="
			]
		},
		"rawTransaction": "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",
		"effects": {
			"messageVersion": "v1",
			"status": {
				"status": "success"
			},
			"executedEpoch": "2",
			"gasUsed": {
				"computationCost": "1000000",
				"storageCost": "8871",
				"storageRebate": "9049",
				"nonRefundableStorageFee": "0"
			},
			"gasObject": {
				"owner": {
					"AddressOwner": "0xab88bd54d960499aa53bbc5810e8a2bce681a016324c4fd3c57e20d1611a541f"
				},
				"reference": {
					"objectId": "0x1f3b54ded3e8bcea7b79e4e7915d46cea8ae1c0982d7a737d6d51c4e949418f6",
					"version": 9214,
					"digest": "BHq3skwHPXPNWhZusju1NrRpfpgfkhU9yg7baPwZoJby"
				}
			},
			"eventsDigest": "7MDxvLKscABGqCvgbsKV6Xh7a3XxkiM11KWnP2xhvFcv",
			"dependencies": [
				"4UspCvV9ve69h3YxXFphPCRgwZBqwfCCKD4pEFXNjrn2",
				"4gdgtJPBCqHZASqVA9Bp5TorQ1LGdDhcg2K9kKzgHsGh",
				"9F6m9qXWwypUbuEwjDC9Wy5mwuSztdWBespXgLn9Kv4n",
				"9XQEhviXFui588NNni9M8XXJh6yygC4qAo7oHweA7T3q",
				"Cgww1sn7XViCPSdDcAPmVcARueWuexJ8af8zD842Ff43",
				"EKVPaHc3VqkFmrFNCrdDFBPQxDrnXNF7RVdu4NKHeA3w"
			]
		},
		"timestampMs": 1680136989850
	}`
	var tx *types.TransactionResponseV1
	if err := json.Unmarshal([]byte(txJSON), &tx); err != nil {
		panic(err)
	}

	query := &TransactionQuery{
		Kind:               "ProgrammableTransaction",
		FromSequenceNumber: 100000,
		ToSequenceNumber:   100000,
	}
	assert.True(t, query.CheckAndTrim(tx))
}

func TestEventFilter(t *testing.T) {
	packageID := types.StrToObjectIDMust("0x3")
	tt := types.TypeTagFromStringMust("0x3::validator::StakingRequestEvent")
	actualTT := types.TypeTagFromStringMust("0x3::validator::UnstakingRequestEvent")
	filter := &EventFilter{
		PackageID: &packageID,
		Type:      &tt,
	}
	event := types.Event{
		PackageID:         packageID,
		TransactionModule: "sui_system",
		Type:              actualTT,
	}
	assert.False(t, filter.Check(event))

	filter = &EventFilter{
		Type: &actualTT,
	}
	assert.True(t, filter.Check(event))

	filter = &EventFilter{
		Op: EventFilterOr,
		Left: &EventFilter{
			Type: &actualTT,
		},
		Right: &EventFilter{
			Type: &tt,
		},
	}
	assert.True(t, filter.Check(event))

	filter = &EventFilter{
		Op: EventFilterAnd,
		Left: &EventFilter{
			Type: &actualTT,
		},
		Right: &EventFilter{
			Type: &tt,
		},
	}
	assert.False(t, filter.Check(event))

	filter = &EventFilter{
		Op: EventFilterAnd,
		Right: &EventFilter{
			Type: &tt,
		},
	}
	assert.False(t, filter.Check(event))

	filter = &EventFilter{
		Op: EventFilterOr,
		Right: &EventFilter{
			Type: &tt,
		},
	}
	assert.True(t, filter.Check(event))
}

func TestObjectChangeQueryFilter(t *testing.T) {
	buildObjectChange := func(ckpt uint64, ownerID, ownerType, objectID, objectType string) types.ObjectChangeExtend {
		return types.ObjectChangeExtend{
			Checkpoint: types.Uint64ToNumber(ckpt),
			ObjectChange: types.ObjectChange{
				ObjectID:   utils.WrapPointer(types.StrToObjectIDMust(objectID)),
				ObjectType: utils.WrapPointer(types.TypeTagFromStringMust(objectType)),
				Owner:      types.BuildObjectOwner(ownerID, ownerType, 0),
			},
		}
	}
	ownerA := "0x3e2304d335881d9b262a0f719be00b29054786466c7ed14b896eb9f26e16532c"
	ownerB := "0x3e2304d335881d9b262a0f719be00b29054786466c7ed14b896eb9f26e16532d"
	ownerTypeA := types.OwnerTypeObject
	ownerTypeB := types.OwnerTypeAddress
	buildObjectID := func(x int) string {
		return types.StrToObjectIDMust(fmt.Sprintf("0x%x", x)).String()
	}
	raw := []types.ObjectChangeExtend{
		buildObjectChange(0, ownerA, ownerTypeA, buildObjectID(1), "0x1::Mod::TypeA"),
		buildObjectChange(1, ownerA, ownerTypeA, buildObjectID(2), "0x1::Mod::TypeB"),
		buildObjectChange(2, ownerA, ownerTypeB, buildObjectID(3), "0x1::Mod::TypeA<bool>"),
		buildObjectChange(3, ownerA, ownerTypeB, buildObjectID(4), "0x1::Mod::TypeB<bool>"),
		buildObjectChange(4, ownerB, ownerTypeA, buildObjectID(5), "0x1::Mod::TypeA"),
		buildObjectChange(5, ownerB, ownerTypeA, buildObjectID(6), "0x1::Mod::TypeB"),
		buildObjectChange(6, ownerB, ownerTypeB, buildObjectID(7), "0x1::Mod::TypeA<bool>"),
		buildObjectChange(7, ownerB, ownerTypeB, buildObjectID(8), "0x1::Mod::TypeB<bool>"),
	}

	assert.Equal(t, raw, ObjectChangeQuery{
		FromSequenceNumber: 0,
		ToSequenceNumber:   1000,
		OwnerType:          "",
		OwnerIDIn:          nil,
		ObjectIDIn:         nil,
		ObjectTypeIn:       nil,
	}.Filter(raw))

	assert.Equal(t, raw[3:8], ObjectChangeQuery{
		FromSequenceNumber: 3,
		ToSequenceNumber:   7,
		OwnerType:          "",
		OwnerIDIn:          nil,
		ObjectIDIn:         nil,
		ObjectTypeIn:       nil,
	}.Filter(raw))

	assert.Equal(t, raw[4:6], ObjectChangeQuery{
		FromSequenceNumber: 3,
		ToSequenceNumber:   7,
		OwnerType:          ownerTypeA,
		OwnerIDIn:          nil,
		ObjectIDIn:         nil,
		ObjectTypeIn:       nil,
	}.Filter(raw))

	assert.Equal(t, raw[3:4], ObjectChangeQuery{
		FromSequenceNumber: 3,
		ToSequenceNumber:   7,
		OwnerType:          "",
		OwnerIDIn:          []string{ownerA},
		ObjectIDIn:         nil,
		ObjectTypeIn:       nil,
	}.Filter(raw))

	assert.Equal(t, raw[3:6], ObjectChangeQuery{
		FromSequenceNumber: 3,
		ToSequenceNumber:   7,
		OwnerType:          "",
		OwnerIDIn:          []string{ownerA},
		ObjectIDIn:         []string{buildObjectID(4), buildObjectID(5), buildObjectID(6)},
		ObjectTypeIn:       nil,
	}.Filter(raw))

	assert.Equal(t, raw[3:7], ObjectChangeQuery{
		FromSequenceNumber: 3,
		ToSequenceNumber:   7,
		OwnerType:          "",
		OwnerIDIn:          []string{ownerA},
		ObjectIDIn:         []string{buildObjectID(5), buildObjectID(6)},
		ObjectTypeIn:       []types.TypeTag{types.TypeTagFromStringMust("0x1::Mod::TypeA")},
	}.Filter(raw))

	assert.Equal(t, []types.ObjectChangeExtend{raw[3], raw[6]}, ObjectChangeQuery{
		FromSequenceNumber: 3,
		ToSequenceNumber:   7,
		OwnerType:          ownerTypeB,
		OwnerIDIn:          []string{ownerA},
		ObjectIDIn:         []string{buildObjectID(5), buildObjectID(6)},
		ObjectTypeIn:       []types.TypeTag{types.TypeTagFromStringMust("0x1::Mod::TypeA")},
	}.Filter(raw))
}
