package types

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/fardream/go-bcs/bcs"
	"github.com/goccy/go-json"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

type AuthSignInfo struct {
	Epoch      Number `json:"epoch"`
	Signature  string `json:"signature"`
	SignersMap []byte `json:"signers_map"`
}

type SharedObject struct {
	ObjectID             ObjectID `json:"objectId"`
	InitialSharedVersion Number   `json:"initialSharedVersion"`
	Mutable              bool     `json:"mutable"`
}

type ObjectArg struct {
	ImmOrOwnedObject *ObjectRef
	SharedObject     *SharedObject
	Receiving        *ObjectRef
}

func (s *ObjectArg) IsBcsEnum() {}

type FundsWithdrawal struct {
	Amount   *uint64
	CoinType *string
	Source   *string
}

func (f *FundsWithdrawal) IsBcsEnum() {}

func (f *FundsWithdrawal) UnmarshalJSON(b []byte) error {
	payload := struct {
		Reservation struct {
			MaxAmountU64 string `json:"maxAmountU64"`
		} `json:"reservation"`
		TypeArg struct {
			Balance string `json:"balance"`
		} `json:"typeArg"`
		WithdrawFrom string `json:"withdrawFrom"`
	}{}
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	if payload.Reservation.MaxAmountU64 != "" {
		amount, err := strconv.ParseUint(payload.Reservation.MaxAmountU64, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid amount %q: %w", payload.Reservation.MaxAmountU64, err)
		}
		f.Amount = &amount
	}
	if payload.TypeArg.Balance != "" {
		f.CoinType = &payload.TypeArg.Balance
	}
	if payload.WithdrawFrom != "" {
		f.Source = &payload.WithdrawFrom
	}
	return nil
}

func (f FundsWithdrawal) MarshalJSON() ([]byte, error) {
	r := map[string]any{"type": "fundsWithdrawal"}
	if f.Amount != nil {
		r["reservation"] = map[string]any{
			"maxAmountU64": strconv.FormatUint(*f.Amount, 10),
		}
	}
	if f.CoinType != nil {
		r["typeArg"] = map[string]any{
			"balance": *f.CoinType,
		}
	}
	if f.Source != nil {
		r["withdrawFrom"] = *f.Source
	}
	return json.Marshal(r)
}

type CallArg struct {
	Pure            *PureValue
	Object          *ObjectArg
	FundsWithdrawal *FundsWithdrawal
}

type callArgObjectJSON struct {
	Type       string   `json:"type"`
	ObjectType string   `json:"objectType"`
	ObjectID   ObjectID `json:"objectId"`

	*ObjectRef
	*SharedObject
}

func (s *CallArg) UnmarshalJSON(data []byte) error {
	var j callArgObjectJSON
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	switch j.Type {
	case "pure":
		return json.Unmarshal(data, &s.Pure)
	case "object":
		s.Object = &ObjectArg{}
		switch j.ObjectType {
		case "immOrOwnedObject":
			return json.Unmarshal(data, &s.Object.ImmOrOwnedObject)
		case "sharedObject":
			return json.Unmarshal(data, &s.Object.SharedObject)
		case "receiving":
			return json.Unmarshal(data, &s.Object.Receiving)
		default:
			return errors.New("invalid call arg objectType")
		}
	case "fundsWithdrawal":
		s.FundsWithdrawal = &FundsWithdrawal{}
		return json.Unmarshal(data, s.FundsWithdrawal)
	default:
		return fmt.Errorf("invalid call arg type %s", j.Type)
	}
}

func (s CallArg) MarshalJSON() ([]byte, error) {
	switch {
	case s.Pure != nil:
		return json.Marshal(s.Pure)
	case s.Object != nil:
		j := &callArgObjectJSON{
			Type:         "object",
			ObjectRef:    s.Object.ImmOrOwnedObject,
			SharedObject: s.Object.SharedObject,
			// TODO should we add receiving
		}
		switch {
		case s.Object.ImmOrOwnedObject != nil:
			j.ObjectType = "immOrOwnedObject"
			j.ObjectID = s.Object.ImmOrOwnedObject.ObjectID
		case s.Object.SharedObject != nil:
			j.ObjectType = "sharedObject"
			j.ObjectID = s.Object.SharedObject.ObjectID
		case s.Object.Receiving != nil:
			j.ObjectType = "receiving"
			j.ObjectID = s.Object.Receiving.ObjectID
		}
		return json.Marshal(j)
	case s.FundsWithdrawal != nil:
		return json.Marshal(s.FundsWithdrawal)
	default:
		panic("invalid CallArg")
	}
}

func (s *CallArg) IsBcsEnum() {}

type TransactionExpiration struct {
	None  *struct{}
	Epoch *uint64
}

func (s TransactionExpiration) MarshalBCS() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	switch {
	case s.None != nil:
		buf.Write(bcs.ULEB128Encode(0))
	case s.Epoch != nil:
		buf.Write(bcs.ULEB128Encode(1))
		serde.Encode(buf, s.Epoch)
	default:
		panic("invalid TransactionExpiration")
	}
	return buf.Bytes(), nil
}

func (s *TransactionExpiration) UnmarshalBCS(r io.Reader) (int, error) {
	enumID, _, err := bcs.ULEB128Decode[int](r)
	if err != nil {
		return 0, err
	}
	switch enumID {
	case 0:
		// None
		s.None = &struct{}{}
	case 1:
		// Epoch
		s.Epoch = new(uint64)
		err = serde.Decode(r, s.Epoch)
	}
	return 0, err
}

type GasData struct {
	Payment []ObjectRefLegacy `json:"payment"`
	Owner   Address           `json:"owner"`
	Price   Number            `json:"price"`
	Budget  Number            `json:"budget"`
}

type Genesis struct {
}

type SystemPackage struct {
	SequenceNumber Number
	Modules        [][]byte
	Dependencies   []ObjectID
}

type ChangeEpoch struct {
	Epoch                   Number          `json:"epoch"`
	ProtocolVersion         Number          `json:"-"`
	StorageCharge           Number          `json:"storage_charge"`
	ComputationCharge       Number          `json:"computation_charge"`
	StorageRebate           Number          `json:"storage_rebate"`
	NonRefundableStorageFee Number          `json:"-"`
	EpochStartTimestampMs   Number          `json:"epoch_start_timestamp_ms"`
	SystemPackages          []SystemPackage `json:"-"`
}

type ChangeEpochV2 struct {
	Epoch                   Number          `json:"epoch"`
	ProtocolVersion         Number          `json:"-"`
	StorageCharge           Number          `json:"storage_charge"`
	ComputationCharge       Number          `json:"computation_charge"`
	StorageRebate           Number          `json:"storage_rebate"`
	NonRefundableStorageFee Number          `json:"-"`
	EpochStartTimestampMs   Number          `json:"epoch_start_timestamp_ms"`
	SystemPackages          []SystemPackage `json:"-"`
	// TODO add more fields
}

type ConsensusCommitPrologue struct {
	Epoch             Number `json:"epoch"`
	Round             Number `json:"round"`
	CommitTimestampMs Number `json:"commit_timestamp_ms"`
}

type ConsensusCommitPrologueV1 struct {
	Epoch                 Number `json:"epoch"`
	Round                 Number `json:"round"`
	CommitTimestampMs     Number `json:"commit_timestamp_ms"`
	ConsensusCommitDigest Digest `json:"consensus_commit_digest"`
	// TODO add following def
	// see: https://docs.sui.io/sui-api-ref#transactionblockresponse
	//pub consensus_determined_version_assignments: ConsensusDeterminedVersionAssignments,
}

type ConsensusCommitPrologueV2 struct {
	Epoch                 Number `json:"epoch"`
	Round                 Number `json:"round"`
	CommitTimestampMs     Number `json:"commit_timestamp_ms"`
	ConsensusCommitDigest Digest `json:"consensus_commit_digest"`
}

type ConsensusCommitPrologueV3 struct {
	Epoch                 Number  `json:"epoch"`
	Round                 Number  `json:"round"`
	SubDagIndex           *Number `json:"sub_dag_index"`
	CommitTimestampMs     Number  `json:"commit_timestamp_ms"`
	ConsensusCommitDigest Digest  `json:"consensus_commit_digest"`
	// TODO add following def
	//pub consensus_determined_version_assignments: ConsensusDeterminedVersionAssignments,
}

type ConsensusCommitPrologueV4 struct {
	Epoch                 Number  `json:"epoch"`
	Round                 Number  `json:"round"`
	SubDagIndex           *Number `json:"sub_dag_index"`
	CommitTimestampMs     Number  `json:"commit_timestamp_ms"`
	ConsensusCommitDigest Digest  `json:"consensus_commit_digest"`
	AdditionalStateDigest Digest  `json:"additional_state_digest"`
	// TODO add following def
	// see: https://docs.sui.io/sui-api-ref#transactionblockresponse
	//pub consensus_determined_version_assignments: ConsensusDeterminedVersionAssignments,
}

type ProgrammableTransaction struct {
	Inputs   []CallArg `json:"inputs"`
	Commands []Command `json:"transactions"`
}

type ActiveJwk struct {
	JwkID struct {
		Iss string `json:"iss"`
		Kid string `json:"kid"`
	} `json:"jwk_id"`
	Jwk struct {
		Kty string `json:"kty"`
		E   string `json:"e"`
		N   string `json:"n"`
		Alg string `json:"alg"`
	} `json:"jwk"`
	Epoch Number `json:"epoch"`
}

type AuthenticatorStateUpdate struct {
	Epoch                                Number      `json:"epoch"`
	Round                                Number      `json:"round"`
	NewActiveJwks                        []ActiveJwk `json:"new_active_jwks"`
	AuthenticatorObjInitialSharedVersion uint64      `json:"-"`
}

type AuthenticatorStateExpire struct {
	MinEpoch                             Number `json:"min_epoch"`
	AuthenticatorObjInitialSharedVersion uint64 `json:"-"`
}

// EndOfEpochTransactionSingle https://docs.sui.io/sui-api-ref#suiendofepochtransactionkind
type EndOfEpochTransactionSingle struct {
	ChangeEpoch                    *ChangeEpoch
	AuthenticatorStateCreate       *struct{}
	AuthenticatorStateExpire       *AuthenticatorStateExpire
	RandomnessStateCreate          *struct{}
	CoinDenyListStateCreate        *struct{}
	StoreExecutionTimeObservations *struct{}
	BridgeStateCreate              *string
	BridgeCommitteeUpdate          *int64
	AccumulatorRootCreate          *struct{}
	CoinRegistryCreate             *struct{}
	DisplayRegistryCreate          *struct{}
	AddressAliasStateCreate        *struct{}
	WriteAccumulatorStorageCost    *struct{}
	ChangeEpochV2                  *ChangeEpochV2 // iota has this kind of value
}

func (s *EndOfEpochTransactionSingle) IsBcsEnum() {}

func (s EndOfEpochTransactionSingle) buildRawStruct() any {
	var j interface{}
	switch {
	case s.AuthenticatorStateCreate != nil:
		j = "AuthenticatorStateCreate"
	case s.RandomnessStateCreate != nil:
		j = "RandomnessStateCreate"
	case s.CoinDenyListStateCreate != nil:
		j = "CoinDenyListStateCreate"
	case s.StoreExecutionTimeObservations != nil:
		j = "StoreExecutionTimeObservations"
	case s.AccumulatorRootCreate != nil:
		j = "AccumulatorRootCreate"
	case s.CoinRegistryCreate != nil:
		j = "CoinRegistryCreate"
	case s.DisplayRegistryCreate != nil:
		j = "DisplayRegistryCreate"
	case s.AddressAliasStateCreate != nil:
		j = "AddressAliasStateCreate"
	case s.WriteAccumulatorStorageCost != nil:
		j = "WriteAccumulatorStorageCost"
	case s.ChangeEpoch != nil:
		j = &struct {
			ChangeEpoch *ChangeEpoch `json:"ChangeEpoch"`
		}{
			ChangeEpoch: s.ChangeEpoch,
		}
	case s.ChangeEpochV2 != nil:
		j = &struct {
			ChangeEpochV2 *ChangeEpochV2 `json:"ChangeEpochV2"`
		}{
			ChangeEpochV2: s.ChangeEpochV2,
		}
	case s.AuthenticatorStateExpire != nil:
		j = &struct {
			AuthenticatorStateExpire *AuthenticatorStateExpire `json:"AuthenticatorStateExpire"`
		}{
			AuthenticatorStateExpire: s.AuthenticatorStateExpire,
		}
	case s.BridgeStateCreate != nil:
		j = &struct {
			BridgeStateCreate string `json:"BridgeStateCreate"`
		}{
			BridgeStateCreate: *s.BridgeStateCreate,
		}
	case s.BridgeCommitteeUpdate != nil:
		j = &struct {
			BridgeCommitteeUpdate int64 `json:"BridgeCommitteeUpdate"`
		}{
			BridgeCommitteeUpdate: *s.BridgeCommitteeUpdate,
		}
	}
	return j
}

func (s EndOfEpochTransactionSingle) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.buildRawStruct())
}

func (s *EndOfEpochTransactionSingle) UnmarshalJSON(data []byte) error {
	if data[0] == '"' {
		var str string
		_ = json.Unmarshal(data, &str)
		switch str {
		case "AuthenticatorStateCreate":
			s.AuthenticatorStateCreate = &struct{}{}
		case "RandomnessStateCreate":
			s.RandomnessStateCreate = &struct{}{}
		case "CoinDenyListStateCreate":
			s.CoinDenyListStateCreate = &struct{}{}
		case "StoreExecutionTimeObservations":
			s.StoreExecutionTimeObservations = &struct{}{}
		case "AccumulatorRootCreate":
			s.AccumulatorRootCreate = &struct{}{}
		case "CoinRegistryCreate":
			s.CoinRegistryCreate = &struct{}{}
		case "DisplayRegistryCreate":
			s.DisplayRegistryCreate = &struct{}{}
		case "AddressAliasStateCreate":
			s.AddressAliasStateCreate = &struct{}{}
		case "WriteAccumulatorStorageCost":
			s.WriteAccumulatorStorageCost = &struct{}{}
		default:
			return errors.New(fmt.Sprintf("invalid EndOfEpochTransactionSingle %q", str))
		}
		return nil
	}
	var j struct {
		ChangeEpoch              *ChangeEpoch              `json:"ChangeEpoch"`
		ChangeEpochV2            *ChangeEpochV2            `json:"ChangeEpochV2"`
		AuthenticatorStateExpire *AuthenticatorStateExpire `json:"AuthenticatorStateExpire"`
		BridgeStateCreate        *string                   `json:"BridgeStateCreate"`
		BridgeCommitteeUpdate    *int64                    `json:"BridgeCommitteeUpdate"`
	}
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	switch {
	case j.ChangeEpoch != nil:
		s.ChangeEpoch = j.ChangeEpoch
	case j.ChangeEpochV2 != nil:
		s.ChangeEpochV2 = j.ChangeEpochV2
	case j.AuthenticatorStateExpire != nil:
		s.AuthenticatorStateExpire = j.AuthenticatorStateExpire
	case j.BridgeStateCreate != nil:
		s.BridgeStateCreate = j.BridgeStateCreate
	case j.BridgeCommitteeUpdate != nil:
		s.BridgeCommitteeUpdate = j.BridgeCommitteeUpdate
	default:
		return errors.New("invalid EndOfEpochTransactionSingle")
	}
	return nil
}

type EndOfEpochTransaction struct {
	Transactions []EndOfEpochTransactionSingle `json:"transactions"`
}

type RandomnessStateUpdate struct {
	// TODO
}

type TransactionKind struct {
	ProgrammableTransaction   *ProgrammableTransaction
	ChangeEpoch               *ChangeEpoch
	Genesis                   *Genesis
	ConsensusCommitPrologue   *ConsensusCommitPrologue
	AuthenticatorStateUpdate  *AuthenticatorStateUpdate
	EndOfEpochTransaction     *EndOfEpochTransaction
	RandomnessStateUpdate     *RandomnessStateUpdate
	ConsensusCommitPrologueV2 *ConsensusCommitPrologueV2
	ConsensusCommitPrologueV3 *ConsensusCommitPrologueV3
	ConsensusCommitPrologueV4 *ConsensusCommitPrologueV4
	ConsensusCommitPrologueV1 *ConsensusCommitPrologueV1 // iota-mainnet has this kind of tx
}

func (s *TransactionKind) Kind() string {
	switch {
	case s.ProgrammableTransaction != nil:
		return "ProgrammableTransaction"
	case s.ChangeEpoch != nil:
		return "ChangeEpoch"
	case s.Genesis != nil:
		return "Genesis"
	case s.ConsensusCommitPrologue != nil:
		return "ConsensusCommitPrologue"
	case s.ConsensusCommitPrologueV1 != nil:
		return "ConsensusCommitPrologueV1"
	case s.ConsensusCommitPrologueV2 != nil:
		return "ConsensusCommitPrologueV2"
	case s.ConsensusCommitPrologueV3 != nil:
		return "ConsensusCommitPrologueV3"
	case s.ConsensusCommitPrologueV4 != nil:
		return "ConsensusCommitPrologueV4"
	case s.AuthenticatorStateUpdate != nil:
		return "AuthenticatorStateUpdate"
	case s.EndOfEpochTransaction != nil:
		return "EndOfEpochTransaction"
	case s.RandomnessStateUpdate != nil:
		return "RandomnessStateUpdate"
	default:
		panic("invalid TransactionKind")
	}
}

type txKindJSON struct {
	Kind string `json:"kind"`
}

func (s *TransactionKind) UnmarshalJSON(data []byte) error {
	var j txKindJSON
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	switch j.Kind {
	case "ProgrammableTransaction":
		return json.Unmarshal(data, &s.ProgrammableTransaction)
	case "ChangeEpoch":
		return json.Unmarshal(data, &s.ChangeEpoch)
	case "Genesis":
		return json.Unmarshal(data, &s.Genesis)
	case "ConsensusCommitPrologue":
		return json.Unmarshal(data, &s.ConsensusCommitPrologue)
	case "ConsensusCommitPrologueV1":
		return json.Unmarshal(data, &s.ConsensusCommitPrologueV1)
	case "ConsensusCommitPrologueV2":
		return json.Unmarshal(data, &s.ConsensusCommitPrologueV2)
	case "ConsensusCommitPrologueV3":
		return json.Unmarshal(data, &s.ConsensusCommitPrologueV3)
	case "ConsensusCommitPrologueV4":
		return json.Unmarshal(data, &s.ConsensusCommitPrologueV4)
	case "AuthenticatorStateUpdate":
		return json.Unmarshal(data, &s.AuthenticatorStateUpdate)
	case "EndOfEpochTransaction":
		return json.Unmarshal(data, &s.EndOfEpochTransaction)
	case "RandomnessStateUpdate":
		s.RandomnessStateUpdate = &RandomnessStateUpdate{}
		return nil
	default:
		return fmt.Errorf("invalid tx kind %q", j.Kind)
	}
}

func (s TransactionKind) MarshalJSON() ([]byte, error) {
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
	return json.Marshal(j)
}

func (s *TransactionKind) IsBcsEnum() {}
