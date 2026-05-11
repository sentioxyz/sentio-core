package types

import (
	"encoding/base64"
	"encoding/json"
	"reflect"

	"google.golang.org/protobuf/types/known/structpb"

	"sentioxyz/sentio-core/common/utils"
)

func (n Number) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(n.String())
}

func (d Digest) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(d.String())
}

func (o ObjectID) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(o.String())
}

func (a Address) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(a.String())
}

func (h Base58Data) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(h.String())
}

func (h Base64Data) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(h.String())
}

func (s StructTag) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(s.String())
}

func (s TypeTag) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(s.String())
}

var objectOwnerTyp = reflect.TypeOf(ObjectOwnerInternal{})

func (o ObjectOwner) MarshalStructpb() *structpb.Value {
	if o.ObjectOwnerInternal != nil {
		s := utils.ConvertToStructpb(o.ObjectOwnerInternal, objectOwnerTyp)
		return structpb.NewStructValue(s)
	}
	if o.string != nil {
		return structpb.NewStringValue(*o.string)
	}
	return structpb.NewNullValue()
}

func (s TransactionData) MarshalStructpb() *structpb.Value {
	return utils.MarshalStructpb(&txDataJSON{
		MessageVersion:    "v1",
		TransactionDataV1: s.V1,
	})
}

func (s TransactionKind) MarshalStructpb() *structpb.Value {
	var j interface{}
	switch {
	case s.ProgrammableTransaction != nil:
		j = &struct {
			Kind string `json:"kind"`
			*ProgrammableTransaction
		}{
			Kind:                    "ProgrammableTransaction",
			ProgrammableTransaction: s.ProgrammableTransaction,
		}
	case s.ChangeEpoch != nil:
		j = &struct {
			Kind string `json:"kind"`
			*ChangeEpoch
		}{
			Kind:        "ChangeEpoch",
			ChangeEpoch: s.ChangeEpoch,
		}
	case s.Genesis != nil:
		j = &struct {
			Kind string `json:"kind"`
			*Genesis
		}{
			Kind:    "Genesis",
			Genesis: s.Genesis,
		}
	case s.ConsensusCommitPrologue != nil:
		j = &struct {
			Kind string `json:"kind"`
			*ConsensusCommitPrologue
		}{
			Kind:                    "ConsensusCommitPrologue",
			ConsensusCommitPrologue: s.ConsensusCommitPrologue,
		}
	case s.ConsensusCommitPrologueV1 != nil:
		j = &struct {
			Kind string `json:"kind"`
			*ConsensusCommitPrologueV1
		}{
			Kind:                      "ConsensusCommitPrologueV1",
			ConsensusCommitPrologueV1: s.ConsensusCommitPrologueV1,
		}
	case s.ConsensusCommitPrologueV2 != nil:
		j = &struct {
			Kind string `json:"kind"`
			*ConsensusCommitPrologueV2
		}{
			Kind:                      "ConsensusCommitPrologueV2",
			ConsensusCommitPrologueV2: s.ConsensusCommitPrologueV2,
		}
	case s.ConsensusCommitPrologueV3 != nil:
		j = &struct {
			Kind string `json:"kind"`
			*ConsensusCommitPrologueV3
		}{
			Kind:                      "ConsensusCommitPrologueV3",
			ConsensusCommitPrologueV3: s.ConsensusCommitPrologueV3,
		}
	case s.ConsensusCommitPrologueV4 != nil:
		j = &struct {
			Kind string `json:"kind"`
			*ConsensusCommitPrologueV4
		}{
			Kind:                      "ConsensusCommitPrologueV4",
			ConsensusCommitPrologueV4: s.ConsensusCommitPrologueV4,
		}
	case s.AuthenticatorStateUpdate != nil:
		j = &struct {
			Kind string `json:"kind"`
			*AuthenticatorStateUpdate
		}{
			Kind:                     "AuthenticatorStateUpdate",
			AuthenticatorStateUpdate: s.AuthenticatorStateUpdate,
		}
	case s.EndOfEpochTransaction != nil:
		j = &struct {
			Kind string `json:"kind"`
			*EndOfEpochTransaction
		}{
			Kind:                  "EndOfEpochTransaction",
			EndOfEpochTransaction: s.EndOfEpochTransaction,
		}
	case s.RandomnessStateUpdate != nil:
		j = &struct {
			Kind string `json:"kind"`
			*RandomnessStateUpdate
		}{
			Kind:                  "RandomnessStateUpdate",
			RandomnessStateUpdate: s.RandomnessStateUpdate,
		}
	}
	return utils.MarshalStructpb(j)
}

func (s Argument) MarshalStructpb() *structpb.Value {
	if s.GasCoin != nil {
		return structpb.NewStringValue("GasCoin")
	}
	return utils.MarshalStructpb(&argumentJSON{
		Input:        s.Input,
		Result:       s.Result,
		NestedResult: s.NestedResult,
	})
}

func (s ArgumentsO2M) MarshalStructpb() *structpb.Value {
	j := []interface{}{s.First, s.Oprands}
	return utils.MarshalStructpb(j)
}

func (s ArgumentsM2O) MarshalStructpb() *structpb.Value {
	j := []interface{}{s.Oprands, s.Last}
	return utils.MarshalStructpb(j)
}

func (s MakeMoveVec) MarshalStructpb() *structpb.Value {
	return utils.MarshalStructpb([]interface{}{
		s.TypeTag,
		s.Args,
	})
}

func (s Publish) MarshalStructpb() *structpb.Value {
	return utils.MarshalStructpb(s.ObjectIDs)
}

func (s Upgrade) MarshalStructpb() *structpb.Value {
	return utils.MarshalStructpb([]interface{}{
		s.TransitiveDeps,
		s.CurrentPackageObjectID,
		s.Argument,
	})
}

func (s MovePackage) MarshalStructpb() *structpb.Value {
	if s.Disassembled == nil {
		panic("disassembled is nil")
	}
	return utils.MarshalStructpb(&movePackageJSON{
		Disassembled: s.Disassembled,
	})
}

func (s CallArg) MarshalStructpb() *structpb.Value {
	switch {
	case s.Pure != nil:
		return utils.MarshalStructpb(s.Pure)
	case s.Object != nil:
		var j interface{}
		switch {
		case s.Object.ImmOrOwnedObject != nil:
			j = &struct {
				Type       string   `json:"type"`
				ObjectType string   `json:"objectType"`
				ObjectID   ObjectID `json:"objectId"`
				*ObjectRef
			}{
				Type:       "object",
				ObjectType: "immOrOwnedObject",
				ObjectID:   s.Object.ImmOrOwnedObject.ObjectID,
				ObjectRef:  s.Object.ImmOrOwnedObject,
			}
		case s.Object.SharedObject != nil:
			j = &struct {
				Type       string   `json:"type"`
				ObjectType string   `json:"objectType"`
				ObjectID   ObjectID `json:"objectId"`
				*SharedObject
			}{
				Type:         "object",
				ObjectType:   "sharedObject",
				ObjectID:     s.Object.SharedObject.ObjectID,
				SharedObject: s.Object.SharedObject,
			}
		case s.Object.Receiving != nil:
			j = &struct {
				Type       string   `json:"type"`
				ObjectType string   `json:"objectType"`
				ObjectID   ObjectID `json:"objectId"`
				*ObjectRef
			}{
				Type:       "object",
				ObjectType: "receiving",
				ObjectID:   s.Object.Receiving.ObjectID,
				ObjectRef:  s.Object.Receiving,
			}
		}
		return utils.MarshalStructpb(j)
	default:
		panic("invalid CallArg")
	}
}

func (s PureValue) MarshalStructpb() *structpb.Value {
	if s.json == nil {
		panic("no type information in PureValue")
	}
	var v interface{}
	err := json.Unmarshal(s.json.Value, &v)
	if err != nil {
		panic(err)
	}
	j := &struct {
		Type      string      `json:"type"`
		ValueType *TypeTag    `json:"valueType"`
		Value     interface{} `json:"value"`
	}{
		Type:      s.json.Type,
		ValueType: s.json.ValueType,
		Value:     v,
	}
	return utils.MarshalStructpb(j)
}

func (s Signature) MarshalStructpb() *structpb.Value {
	return structpb.NewStringValue(base64.StdEncoding.EncodeToString(s))
}

func (s Event) MarshalStructpb() *structpb.Value {
	type eventStructpb struct {
		ID                EventID                `json:"id"`
		PackageID         ObjectID               `json:"packageId"`
		TransactionModule string                 `json:"transactionModule"`
		Sender            string                 `json:"sender"`
		Type              TypeTag                `json:"type"`
		Fields            map[string]interface{} `json:"parsedJson"`
		BCS               string                 `json:"bcs"`
	}
	var fields map[string]interface{}
	if s.Fields != nil {
		if err := json.Unmarshal(s.Fields, &fields); err != nil {
			panic(err)
		}
	}
	return utils.MarshalStructpb(&eventStructpb{
		ID:                s.ID,
		PackageID:         s.PackageID,
		TransactionModule: s.TransactionModule,
		Sender:            s.Sender,
		Type:              s.Type,
		Fields:            fields,
		BCS:               s.BCS,
	})
}

func (s EndOfEpochTransactionSingle) MarshalStructpb() *structpb.Value {
	return utils.MarshalStructpb(s.buildRawStruct())
}

func init() {
	utils.RegisterSpecialType(reflect.TypeOf(Digest{}))
	utils.RegisterSpecialType(reflect.TypeOf(Number{}))
	utils.RegisterSpecialType(reflect.TypeOf(Base64Data{}))
	utils.RegisterSpecialType(reflect.TypeOf(Base58Data{}))
	utils.RegisterSpecialType(reflect.TypeOf(Address{}))
	utils.RegisterSpecialType(reflect.TypeOf(StructTag{}))
	utils.RegisterSpecialType(reflect.TypeOf(TypeTag{}))
	utils.RegisterSpecialType(reflect.TypeOf(ObjectID{}))
	utils.RegisterSpecialType(reflect.TypeOf(ObjectOwner{}))
	utils.RegisterSpecialType(reflect.TypeOf(TransactionData{}))
	utils.RegisterSpecialType(reflect.TypeOf(TransactionKind{}))
	utils.RegisterSpecialType(reflect.TypeOf(Argument{}))
	utils.RegisterSpecialType(reflect.TypeOf(ArgumentsO2M{}))
	utils.RegisterSpecialType(reflect.TypeOf(ArgumentsM2O{}))
	utils.RegisterSpecialType(reflect.TypeOf(MakeMoveVec{}))
	utils.RegisterSpecialType(reflect.TypeOf(Publish{}))
	utils.RegisterSpecialType(reflect.TypeOf(Upgrade{}))
	utils.RegisterSpecialType(reflect.TypeOf(MovePackage{}))
	utils.RegisterSpecialType(reflect.TypeOf(CallArg{}))
	utils.RegisterSpecialType(reflect.TypeOf(PureValue{}))
	utils.RegisterSpecialType(reflect.TypeOf(Signature{}))
	utils.RegisterSpecialType(reflect.TypeOf(Event{}))
	utils.RegisterSpecialType(reflect.TypeOf(EndOfEpochTransactionSingle{}))
}
