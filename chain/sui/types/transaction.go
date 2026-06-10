package types

import (
	"bytes"
	"io"
	"strconv"

	"github.com/fardream/go-bcs/bcs"
	"github.com/goccy/go-json"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/sui/types/serde"
)

type SharedObject struct {
	ObjectID             ObjectID               `json:"objectId"`
	InitialSharedVersion Number                 `json:"initialSharedVersion"`
	Mutability           SharedObjectMutability `json:"mutable"`
}

// SharedObjectMutability mirrors upstream `SharedObjectMutability` (sui-types
// transaction.rs). It used to be a plain `bool`, but Sui turned it into an enum
// while keeping the first two variants binary-compatible with the old bool
// (0x00/0x01). A third variant (0x02, NonExclusiveWrite) now appears in
// settlement / system transactions, so a Go `bool` can no longer round-trip the
// BCS byte-for-byte.
//
// BCS: a plain Rust enum, i.e. a ULEB128 variant index; for the 3 variants here
// that is a single byte, matching how serde encodes this `uint8`.
//
// JSON: the json-rpc reply (and what we serve) collapses this back to a bool
// exactly as upstream sui-json-rpc-types does — only Mutable is reported as
// `true`; Immutable and NonExclusiveWrite are `false`. Because that mapping is
// lossy, the exact variant for the BCS re-encode is restored from the decoded
// raw transaction in DeriveAuxInformationFromBCSV1.
type SharedObjectMutability uint8

const (
	SharedObjectImmutable         SharedObjectMutability = 0
	SharedObjectMutable           SharedObjectMutability = 1
	SharedObjectNonExclusiveWrite SharedObjectMutability = 2
)

func (m SharedObjectMutability) MarshalJSON() ([]byte, error) {
	return json.Marshal(m == SharedObjectMutable)
}

func (m *SharedObjectMutability) UnmarshalJSON(b []byte) error {
	var mutable bool
	if err := json.Unmarshal(b, &mutable); err != nil {
		return err
	}
	if mutable {
		*m = SharedObjectMutable
	} else {
		*m = SharedObjectImmutable
	}
	return nil
}

type ObjectArg struct {
	ImmOrOwnedObject *ObjectRef
	SharedObject     *SharedObject
	Receiving        *ObjectRef
}

func (s *ObjectArg) IsBcsEnum() {}

// FundsWithdrawal mirrors upstream `FundsWithdrawalArg` (sui-types
// transaction.rs), the payload of `CallArg::FundsWithdrawal`. It is a *struct*
// of three nested enums — NOT an enum itself (an earlier version mis-modeled it
// as a 3-variant enum, which mis-aligned the rest of the BCS stream and either
// failed to decode or panicked on a bogus length-prefix). FundsWithdrawal is
// Sui-only; IOTA's CallArg has no such variant.
type FundsWithdrawal struct {
	Reservation  *Reservation
	TypeArg      *WithdrawalTypeArg
	WithdrawFrom *WithdrawFrom
}

// Reservation: enum { MaxAmountU64(u64) }.
type Reservation struct {
	MaxAmountU64 *uint64
}

func (r *Reservation) IsBcsEnum() {}

// WithdrawalTypeArg: enum { Balance(TypeTag) } — the type parameter T of the
// funds accumulator `Balance<T>` being withdrawn.
type WithdrawalTypeArg struct {
	Balance *TypeTag
}

func (w *WithdrawalTypeArg) IsBcsEnum() {}

// WithdrawFrom: enum { Sender, Sponsor } — both unit variants.
type WithdrawFrom struct {
	Sender  *struct{}
	Sponsor *struct{}
}

func (w *WithdrawFrom) IsBcsEnum() {}

func (f *FundsWithdrawal) UnmarshalJSON(b []byte) error {
	payload := struct {
		Reservation struct {
			MaxAmountU64 string `json:"maxAmountU64"`
		} `json:"reservation"`
		TypeArg struct {
			Balance *TypeTag `json:"balance"`
		} `json:"typeArg"`
		WithdrawFrom string `json:"withdrawFrom"`
	}{}
	if err := json.Unmarshal(b, &payload); err != nil {
		return err
	}
	if payload.Reservation.MaxAmountU64 != "" {
		amount, err := strconv.ParseUint(payload.Reservation.MaxAmountU64, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "invalid amount %q", payload.Reservation.MaxAmountU64)
		}
		f.Reservation = &Reservation{MaxAmountU64: &amount}
	}
	if payload.TypeArg.Balance != nil {
		f.TypeArg = &WithdrawalTypeArg{Balance: payload.TypeArg.Balance}
	}
	switch payload.WithdrawFrom {
	case "":
	case "sender", "Sender":
		f.WithdrawFrom = &WithdrawFrom{Sender: &struct{}{}}
	case "sponsor", "Sponsor":
		f.WithdrawFrom = &WithdrawFrom{Sponsor: &struct{}{}}
	default:
		return errors.Errorf("invalid withdrawFrom %q", payload.WithdrawFrom)
	}
	return nil
}

func (f FundsWithdrawal) MarshalJSON() ([]byte, error) {
	r := map[string]any{"type": "fundsWithdrawal"}
	if f.Reservation != nil && f.Reservation.MaxAmountU64 != nil {
		r["reservation"] = map[string]any{
			"maxAmountU64": strconv.FormatUint(*f.Reservation.MaxAmountU64, 10),
		}
	}
	if f.TypeArg != nil && f.TypeArg.Balance != nil {
		r["typeArg"] = map[string]any{
			"balance": f.TypeArg.Balance,
		}
	}
	if f.WithdrawFrom != nil {
		switch {
		case f.WithdrawFrom.Sender != nil:
			r["withdrawFrom"] = "sender"
		case f.WithdrawFrom.Sponsor != nil:
			r["withdrawFrom"] = "sponsor"
		}
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
		return errors.Errorf("invalid call arg type %s", j.Type)
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
		panic(errors.New("invalid CallArg"))
	}
}

func (s *CallArg) IsBcsEnum() {}

type TransactionExpiration struct {
	None        *struct{}
	Epoch       *uint64
	ValidDuring *ValidDuring
}

// ValidDuring mirrors sui's TransactionExpiration::ValidDuring (BCS variant 2).
// Field order/types match sui-types/src/transaction.rs:
//
//	ValidDuring {
//	    min_epoch: Option<EpochId>, max_epoch: Option<EpochId>,
//	    min_timestamp: Option<u64>, max_timestamp: Option<u64>,
//	    chain: ChainIdentifier, nonce: u32,
//	}
//
// ChainIdentifier wraps a CheckpointDigest(Digest([u8;32])) whose
// `#[serde_as(as = "Readable<Base58, Bytes>")]` makes BCS length-prefix the
// 32 bytes (serialize_bytes), so Chain is encoded as ULEB128(len)+bytes.
type ValidDuring struct {
	MinEpoch     *uint64
	MaxEpoch     *uint64
	MinTimestamp *uint64
	MaxTimestamp *uint64
	Chain        []byte
	Nonce        uint32
}

func (s TransactionExpiration) MarshalBCS() ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	switch {
	case s.None != nil:
		buf.Write(bcs.ULEB128Encode(0))
	case s.Epoch != nil:
		buf.Write(bcs.ULEB128Encode(1))
		serde.Encode(buf, s.Epoch)
	case s.ValidDuring != nil:
		buf.Write(bcs.ULEB128Encode(2))
		vd := s.ValidDuring
		for _, opt := range []*uint64{vd.MinEpoch, vd.MaxEpoch, vd.MinTimestamp, vd.MaxTimestamp} {
			if opt == nil {
				buf.Write(bcs.ULEB128Encode(0))
			} else {
				buf.Write(bcs.ULEB128Encode(1))
				serde.Encode(buf, opt)
			}
		}
		buf.Write(bcs.ULEB128Encode(len(vd.Chain)))
		buf.Write(vd.Chain)
		serde.Encode(buf, &vd.Nonce)
	default:
		panic(errors.New("invalid TransactionExpiration"))
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
	case 2:
		// ValidDuring
		vd := &ValidDuring{}
		for _, dst := range []**uint64{&vd.MinEpoch, &vd.MaxEpoch, &vd.MinTimestamp, &vd.MaxTimestamp} {
			var tag int
			if tag, _, err = bcs.ULEB128Decode[int](r); err != nil {
				return 0, err
			}
			if tag == 1 {
				v := new(uint64)
				if err = serde.Decode(r, v); err != nil {
					return 0, err
				}
				*dst = v
			}
		}
		var chainLen int
		if chainLen, _, err = bcs.ULEB128Decode[int](r); err != nil {
			return 0, err
		}
		vd.Chain = make([]byte, chainLen)
		if _, err = io.ReadFull(r, vd.Chain); err != nil {
			return 0, err
		}
		if err = serde.Decode(r, &vd.Nonce); err != nil {
			return 0, err
		}
		s.ValidDuring = vd
	default:
		return 0, errors.Errorf("unknown TransactionExpiration variant %d", enumID)
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

// ChangeEpochV2/V3/V4 are IOTA's end-of-epoch change-epoch payloads (variants 1,
// 2, 3 of EndOfEpochTransactionKind). Field order matches the IOTA staged BCS
// layout (iota-core/tests/staged/iota.yaml). protocol_version,
// non_refundable_storage_fee, system_packages and adjust_rewards_by_score are not
// in the json reply and are derived from the decoded BCS (DeriveAux).
//
// IOTA's json-rpc collapses all three into a single "ChangeEpochV2" kind with
// optional eligible_active_validators/scores, so EndOfEpochTransactionSingle's
// UnmarshalJSON disambiguates V2/V3/V4 by which of those fields are present.
type ChangeEpochV2 struct {
	Epoch                   Number          `json:"epoch"`
	ProtocolVersion         Number          `json:"-"`
	StorageCharge           Number          `json:"storage_charge"`
	ComputationCharge       Number          `json:"computation_charge"`
	ComputationChargeBurned Number          `json:"computation_charge_burned"`
	StorageRebate           Number          `json:"storage_rebate"`
	NonRefundableStorageFee Number          `json:"-"`
	EpochStartTimestampMs   Number          `json:"epoch_start_timestamp_ms"`
	SystemPackages          []SystemPackage `json:"-"`
}

type ChangeEpochV3 struct {
	Epoch                    Number          `json:"epoch"`
	ProtocolVersion          Number          `json:"-"`
	StorageCharge            Number          `json:"storage_charge"`
	ComputationCharge        Number          `json:"computation_charge"`
	ComputationChargeBurned  Number          `json:"computation_charge_burned"`
	StorageRebate            Number          `json:"storage_rebate"`
	NonRefundableStorageFee  Number          `json:"-"`
	EpochStartTimestampMs    Number          `json:"epoch_start_timestamp_ms"`
	SystemPackages           []SystemPackage `json:"-"`
	EligibleActiveValidators []Number        `json:"eligible_active_validators"`
}

type ChangeEpochV4 struct {
	Epoch                    Number          `json:"epoch"`
	ProtocolVersion          Number          `json:"-"`
	StorageCharge            Number          `json:"storage_charge"`
	ComputationCharge        Number          `json:"computation_charge"`
	ComputationChargeBurned  Number          `json:"computation_charge_burned"`
	StorageRebate            Number          `json:"storage_rebate"`
	NonRefundableStorageFee  Number          `json:"-"`
	EpochStartTimestampMs    Number          `json:"epoch_start_timestamp_ms"`
	SystemPackages           []SystemPackage `json:"-"`
	EligibleActiveValidators []Number        `json:"eligible_active_validators"`
	Scores                   []Number        `json:"scores"`
	AdjustRewardsByScore     bool            `json:"-"`
}

type ConsensusCommitPrologue struct {
	Epoch             Number `json:"epoch"`
	Round             Number `json:"round"`
	CommitTimestampMs Number `json:"commit_timestamp_ms"`
}

// ConsensusCommitPrologueV1 is IOTA's consensus commit prologue (TransactionKind
// variant index 2 on IOTA). Its layout matches Sui's V3 (sub_dag_index Option,
// consensus_commit_digest, consensus_determined_version_assignments) but it has
// NO additional_state_digest. Sui has no "ConsensusCommitPrologueV1" kind name.
type ConsensusCommitPrologueV1 struct {
	Epoch                                 Number                                 `json:"epoch"`
	Round                                 Number                                 `json:"round"`
	SubDagIndex                           *Number                                `json:"sub_dag_index" bcs:"optional"`
	CommitTimestampMs                     Number                                 `json:"commit_timestamp_ms"`
	ConsensusCommitDigest                 Digest                                 `json:"consensus_commit_digest"`
	ConsensusDeterminedVersionAssignments *ConsensusDeterminedVersionAssignments `json:"consensus_determined_version_assignments"`
}

type ConsensusCommitPrologueV2 struct {
	Epoch                 Number `json:"epoch"`
	Round                 Number `json:"round"`
	CommitTimestampMs     Number `json:"commit_timestamp_ms"`
	ConsensusCommitDigest Digest `json:"consensus_commit_digest"`
}

type ConsensusCommitPrologueV3 struct {
	Epoch                                 Number                                 `json:"epoch"`
	Round                                 Number                                 `json:"round"`
	SubDagIndex                           *Number                                `json:"sub_dag_index" bcs:"optional"`
	CommitTimestampMs                     Number                                 `json:"commit_timestamp_ms"`
	ConsensusCommitDigest                 Digest                                 `json:"consensus_commit_digest"`
	ConsensusDeterminedVersionAssignments *ConsensusDeterminedVersionAssignments `json:"consensus_determined_version_assignments"`
}

type ConsensusCommitPrologueV4 struct {
	Epoch                                 Number                                 `json:"epoch"`
	Round                                 Number                                 `json:"round"`
	SubDagIndex                           *Number                                `json:"sub_dag_index" bcs:"optional"`
	CommitTimestampMs                     Number                                 `json:"commit_timestamp_ms"`
	ConsensusCommitDigest                 Digest                                 `json:"consensus_commit_digest"`
	ConsensusDeterminedVersionAssignments *ConsensusDeterminedVersionAssignments `json:"consensus_determined_version_assignments"`
	AdditionalStateDigest                 Digest                                 `json:"additional_state_digest"`
}

// ConsensusDeterminedVersionAssignments is a BCS enum carrying shared-object
// version assignments for transactions cancelled by consensus. Sui has two
// variants (CanceledTransactions=0, CanceledTransactionsV2=1); IOTA has only the
// first (index 0). In json-rpc the variant key is spelled "Cancelled" (double l)
// even though the Rust type spells it "Canceled"; UnmarshalJSON accepts both.
type ConsensusDeterminedVersionAssignments struct {
	CanceledTransactions   *CanceledTransactions   `bcs:"enumNum[sui]=0,enumNum[iota]=0"`
	CanceledTransactionsV2 *CanceledTransactionsV2 `bcs:"enumNum[sui]=1"`
}

func (s *ConsensusDeterminedVersionAssignments) IsBcsEnum() {}

// CanceledTransactions wraps the single tuple field of the variant (Vec<CanceledTransaction>);
// a one-field struct encodes identically to the bare field in BCS.
type CanceledTransactions struct {
	Transactions []CanceledTransaction
}

type CanceledTransaction struct {
	TxDigest           Digest
	VersionAssignments []VersionAssignment
}

type VersionAssignment struct {
	ObjectID ObjectID
	Version  Number
}

type CanceledTransactionsV2 struct {
	Transactions []CanceledTransactionV2
}

type CanceledTransactionV2 struct {
	TxDigest           Digest
	VersionAssignments []VersionAssignmentV2
}

type VersionAssignmentV2 struct {
	ObjectID     ObjectID
	StartVersion Number
	Version      Number
}

func (s *ConsensusDeterminedVersionAssignments) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	for k, v := range m {
		switch k {
		case "CancelledTransactions", "CanceledTransactions":
			var arr []CanceledTransaction
			if err := json.Unmarshal(v, &arr); err != nil {
				return err
			}
			s.CanceledTransactions = &CanceledTransactions{Transactions: arr}
			return nil
		case "CancelledTransactionsV2", "CanceledTransactionsV2":
			var arr []CanceledTransactionV2
			if err := json.Unmarshal(v, &arr); err != nil {
				return err
			}
			s.CanceledTransactionsV2 = &CanceledTransactionsV2{Transactions: arr}
			return nil
		}
	}
	return errors.Errorf("unknown ConsensusDeterminedVersionAssignments variant in %s", string(data))
}

func (s ConsensusDeterminedVersionAssignments) MarshalJSON() ([]byte, error) {
	switch {
	case s.CanceledTransactions != nil:
		return json.Marshal(map[string]any{"CancelledTransactions": s.CanceledTransactions.Transactions})
	case s.CanceledTransactionsV2 != nil:
		return json.Marshal(map[string]any{"CancelledTransactionsV2": s.CanceledTransactionsV2.Transactions})
	default:
		return []byte("null"), nil
	}
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

// EndOfEpochTransactionSingle is the EndOfEpochTransactionKind enum. Variant
// indices and payloads follow the authoritative BCS layout (sui-types staged
// snapshot), NOT the Go field order. Several variants are units in the json-rpc
// reply but carry a BCS payload (StoreExecutionTimeObservations,
// WriteAccumulatorStorageCost) — those payloads are json:"-" and derived from
// the decoded BCS. IOTA uses different ChangeEpoch payloads (V2/V3/V4).
// See https://docs.sui.io/sui-api-ref#suiendofepochtransactionkind
type EndOfEpochTransactionSingle struct {
	ChangeEpoch                    *ChangeEpoch                     `bcs:"enumNum[sui]=0"`
	AuthenticatorStateCreate       *struct{}                        `bcs:"enumNum[sui]=1"`
	AuthenticatorStateExpire       *AuthenticatorStateExpire        `bcs:"enumNum[sui]=2"`
	RandomnessStateCreate          *struct{}                        `bcs:"enumNum[sui]=3"`
	CoinDenyListStateCreate        *struct{}                        `bcs:"enumNum[sui]=4"`
	BridgeStateCreate              *Digest                          `bcs:"enumNum[sui]=5"` // ChainIdentifier
	BridgeCommitteeUpdate          *uint64                          `bcs:"enumNum[sui]=6"` // BridgeCommitteeInit(SequenceNumber)
	StoreExecutionTimeObservations *StoredExecutionTimeObservations `bcs:"enumNum[sui]=7"`
	AccumulatorRootCreate          *struct{}                        `bcs:"enumNum[sui]=8"`
	CoinRegistryCreate             *struct{}                        `bcs:"enumNum[sui]=9"`
	DisplayRegistryCreate          *struct{}                        `bcs:"enumNum[sui]=10"`
	AddressAliasStateCreate        *struct{}                        `bcs:"enumNum[sui]=11"`
	WriteAccumulatorStorageCost    *WriteAccumulatorStorageCost     `bcs:"enumNum[sui]=12"`
	// IOTA's EndOfEpochTransactionKind ChangeEpoch payloads (V2/V3/V4 at iota
	// variants 1/2/3); ChangeEpoch differs from Sui's so it is sui-only above.
	ChangeEpochV2 *ChangeEpochV2 `bcs:"enumNum[sui]=13,enumNum[iota]=1"`
	ChangeEpochV3 *ChangeEpochV3 `bcs:"enumNum[iota]=2"`
	ChangeEpochV4 *ChangeEpochV4 `bcs:"enumNum[iota]=3"`
}

// StoredExecutionTimeObservations is the BCS payload of the
// StoreExecutionTimeObservations end-of-epoch variant. The json-rpc reply
// reports the variant as a bare string with no data, so the payload is json:"-"
// and derived from the decoded BCS. Layout: enum { V1(Vec<(ExecutionTimeObservationKey,
// Vec<(AuthorityPublicKeyBytes, Duration)>)>) }.
type StoredExecutionTimeObservations struct {
	V1 *StoredExecutionTimeObservationsV1 `json:"-"`
}

func (*StoredExecutionTimeObservations) IsBcsEnum() {}

type StoredExecutionTimeObservationsV1 struct {
	Observations []ExecutionTimeObservation
}

type ExecutionTimeObservation struct {
	Key          *ExecutionTimeObservationKey
	Observations []ValidatorDurationObservation
}

type ValidatorDurationObservation struct {
	Authority []byte // AuthorityPublicKeyBytes: length-prefixed bytes
	Duration  Duration
}

type Duration struct {
	Secs  uint64
	Nanos uint32
}

type ExecutionTimeObservationKey struct {
	MoveEntryPoint  *MoveEntryPoint `bcs:"enumNum[sui]=0"`
	TransferObjects *struct{}       `bcs:"enumNum[sui]=1"`
	SplitCoins      *struct{}       `bcs:"enumNum[sui]=2"`
	MergeCoins      *struct{}       `bcs:"enumNum[sui]=3"`
	Publish         *struct{}       `bcs:"enumNum[sui]=4"`
	MakeMoveVec     *struct{}       `bcs:"enumNum[sui]=5"`
	Upgrade         *struct{}       `bcs:"enumNum[sui]=6"`
}

func (*ExecutionTimeObservationKey) IsBcsEnum() {}

type MoveEntryPoint struct {
	Package       ObjectID
	Module        string
	Function      string
	TypeArguments []TypeTag // TypeInput shares TypeTag's BCS layout
}

// WriteAccumulatorStorageCost is the BCS payload of the
// WriteAccumulatorStorageCost end-of-epoch variant (a json-rpc unit variant),
// derived from BCS.
type WriteAccumulatorStorageCost struct {
	StorageCost Number `json:"-"`
}

func (s *EndOfEpochTransactionSingle) IsBcsEnum() {}

// jsonFieldPresent reports whether a captured json.RawMessage corresponds to a
// field that was present and non-null in the source object.
func jsonFieldPresent(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

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
	case s.ChangeEpochV3 != nil:
		// IOTA json-rpc reports V3 under the "ChangeEpochV2" kind name.
		j = &struct {
			ChangeEpochV3 *ChangeEpochV3 `json:"ChangeEpochV2"`
		}{
			ChangeEpochV3: s.ChangeEpochV3,
		}
	case s.ChangeEpochV4 != nil:
		// IOTA json-rpc reports V4 under the "ChangeEpochV2" kind name.
		j = &struct {
			ChangeEpochV4 *ChangeEpochV4 `json:"ChangeEpochV2"`
		}{
			ChangeEpochV4: s.ChangeEpochV4,
		}
	case s.AuthenticatorStateExpire != nil:
		j = &struct {
			AuthenticatorStateExpire *AuthenticatorStateExpire `json:"AuthenticatorStateExpire"`
		}{
			AuthenticatorStateExpire: s.AuthenticatorStateExpire,
		}
	case s.BridgeStateCreate != nil:
		j = &struct {
			BridgeStateCreate Digest `json:"BridgeStateCreate"`
		}{
			BridgeStateCreate: *s.BridgeStateCreate,
		}
	case s.BridgeCommitteeUpdate != nil:
		j = &struct {
			BridgeCommitteeUpdate uint64 `json:"BridgeCommitteeUpdate"`
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
			// payload is not in json; DeriveAux fills it from the decoded BCS.
			s.StoreExecutionTimeObservations = &StoredExecutionTimeObservations{}
		case "AccumulatorRootCreate":
			s.AccumulatorRootCreate = &struct{}{}
		case "CoinRegistryCreate":
			s.CoinRegistryCreate = &struct{}{}
		case "DisplayRegistryCreate":
			s.DisplayRegistryCreate = &struct{}{}
		case "AddressAliasStateCreate":
			s.AddressAliasStateCreate = &struct{}{}
		case "WriteAccumulatorStorageCost":
			// payload is not in json; DeriveAux fills it from the decoded BCS.
			s.WriteAccumulatorStorageCost = &WriteAccumulatorStorageCost{}
		default:
			return errors.Errorf("invalid EndOfEpochTransactionSingle %q", str)
		}
		return nil
	}
	var j struct {
		ChangeEpoch              *ChangeEpoch              `json:"ChangeEpoch"`
		ChangeEpochV2            json.RawMessage           `json:"ChangeEpochV2"`
		AuthenticatorStateExpire *AuthenticatorStateExpire `json:"AuthenticatorStateExpire"`
		BridgeStateCreate        *Digest                   `json:"BridgeStateCreate"`
		BridgeCommitteeUpdate    *uint64                   `json:"BridgeCommitteeUpdate"`
	}
	err := json.Unmarshal(data, &j)
	if err != nil {
		return err
	}
	switch {
	case j.ChangeEpoch != nil:
		s.ChangeEpoch = j.ChangeEpoch
	case jsonFieldPresent(j.ChangeEpochV2):
		// IOTA json-rpc reports BCS ChangeEpoch V2/V3/V4 all under the single
		// "ChangeEpochV2" kind, distinguished by which optional fields appear:
		// scores => V4, eligible_active_validators only => V3, neither => V2.
		var probe struct {
			Eligible json.RawMessage `json:"eligible_active_validators"`
			Scores   json.RawMessage `json:"scores"`
		}
		if err := json.Unmarshal(j.ChangeEpochV2, &probe); err != nil {
			return err
		}
		switch {
		case jsonFieldPresent(probe.Scores):
			s.ChangeEpochV4 = &ChangeEpochV4{}
			return json.Unmarshal(j.ChangeEpochV2, s.ChangeEpochV4)
		case jsonFieldPresent(probe.Eligible):
			s.ChangeEpochV3 = &ChangeEpochV3{}
			return json.Unmarshal(j.ChangeEpochV2, s.ChangeEpochV3)
		default:
			s.ChangeEpochV2 = &ChangeEpochV2{}
			return json.Unmarshal(j.ChangeEpochV2, s.ChangeEpochV2)
		}
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

// RandomnessStateUpdate mirrors sui/iota RandomnessStateUpdate. Field order/types
// match sui-types/src/transaction.rs (and the IOTA equivalent):
//
//	RandomnessStateUpdate {
//	    epoch: u64, randomness_round: u64, random_bytes: Vec<u8>,
//	    randomness_obj_initial_shared_version: SequenceNumber,
//	}
//
// randomness_obj_initial_shared_version is not part of the json-rpc reply, so it
// is derived from the decoded BCS (DeriveAuxInformationFromBCSV1).
type RandomnessStateUpdate struct {
	Epoch                             Number     `json:"epoch"`
	RandomnessRound                   Number     `json:"randomness_round"`
	RandomBytes                       Uint8Slice `json:"random_bytes"`
	RandomnessObjInitialSharedVersion uint64     `json:"-"`
}

// TransactionKind is a BCS enum whose variant indices differ between Sui and
// IOTA, so each field carries per-selector enumNum tags rather than relying on
// Go field position. See bcs_enum_selector_design.md for the full index table.
// Sui:  0 Programmable, 1 ChangeEpoch, 2 Genesis, 3 ConsensusCommitPrologue(V1),
//
//	4 AuthenticatorStateUpdate, 5 EndOfEpoch, 6 RandomnessStateUpdate,
//	7 CCPv2, 8 CCPv3, 9 CCPv4, 10 ProgrammableSystemTransaction.
//
// IOTA: 0 Programmable, 1 Genesis, 2 ConsensusCommitPrologueV1,
//
//	3 AuthenticatorStateUpdateV1(deprecated), 4 EndOfEpoch, 5 RandomnessStateUpdate.
//
// IOTA-only / Sui-only kinds whose payloads have not been verified are tagged
// for the chain we have validated; an unverified variant decodes to a loud
// "variant not defined" error rather than silently corrupting a round-trip.
type TransactionKind struct {
	ProgrammableTransaction       *ProgrammableTransaction   `bcs:"enumNum[sui]=0,enumNum[iota]=0"`
	ChangeEpoch                   *ChangeEpoch               `bcs:"enumNum[sui]=1"`
	Genesis                       *Genesis                   `bcs:"enumNum[sui]=2,enumNum[iota]=1"`
	ConsensusCommitPrologue       *ConsensusCommitPrologue   `bcs:"enumNum[sui]=3"`
	AuthenticatorStateUpdate      *AuthenticatorStateUpdate  `bcs:"enumNum[sui]=4"`
	EndOfEpochTransaction         *EndOfEpochTransaction     `bcs:"enumNum[sui]=5,enumNum[iota]=4"`
	RandomnessStateUpdate         *RandomnessStateUpdate     `bcs:"enumNum[sui]=6,enumNum[iota]=5"`
	ConsensusCommitPrologueV2     *ConsensusCommitPrologueV2 `bcs:"enumNum[sui]=7"`
	ConsensusCommitPrologueV3     *ConsensusCommitPrologueV3 `bcs:"enumNum[sui]=8"`
	ConsensusCommitPrologueV4     *ConsensusCommitPrologueV4 `bcs:"enumNum[sui]=9"`
	ProgrammableSystemTransaction *ProgrammableTransaction   `bcs:"enumNum[sui]=10"`
	ConsensusCommitPrologueV1     *ConsensusCommitPrologueV1 `bcs:"enumNum[iota]=2"`
}

func (s *TransactionKind) Kind() string {
	switch {
	case s.ProgrammableTransaction != nil:
		return "ProgrammableTransaction"
	case s.ProgrammableSystemTransaction != nil:
		// Current sui json-rpc reports the system PTB (BCS variant 10) under the
		// same "ProgrammableTransaction" kind as a regular PTB; present it the same
		// way (the variant is an internal BCS distinction, see DeriveAux).
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
		panic(errors.New("invalid TransactionKind"))
	}
}

// Programmable returns the programmable-transaction payload for both a regular
// PTB (BCS variant 0) and a system PTB (BCS variant 10,
// ProgrammableSystemTransaction), or nil for any other kind. Callers that only
// care that the tx is "programmable" (move-call filters, input/command
// extraction, etc.) should use this rather than the ProgrammableTransaction
// field, since current sui json-rpc reports both under the same kind name.
func (s *TransactionKind) Programmable() *ProgrammableTransaction {
	if s.ProgrammableTransaction != nil {
		return s.ProgrammableTransaction
	}
	return s.ProgrammableSystemTransaction
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
	case "ProgrammableSystemTransaction":
		return json.Unmarshal(data, &s.ProgrammableSystemTransaction)
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
		return json.Unmarshal(data, &s.RandomnessStateUpdate)
	default:
		return errors.Errorf("invalid tx kind %q", j.Kind)
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
	case s.ProgrammableSystemTransaction != nil:
		// Present the system PTB under the same json kind as a regular PTB (see Kind()).
		j = &struct {
			Kind string `json:"kind"`
			*ProgrammableTransaction
		}{
			Kind:                    "ProgrammableTransaction",
			ProgrammableTransaction: s.ProgrammableSystemTransaction,
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
