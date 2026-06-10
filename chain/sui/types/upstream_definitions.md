# Upstream transaction-kind definitions (Sui & IOTA) — collection for completing `uncompletedKinds`

Authoritative field layouts for the system-transaction types that
`chain/sui/types` must model byte-exactly to empty `uncompletedKinds`. This is a
**reference snapshot** — re-derive from upstream before implementing (see
`CLAUDE.md` for how), and verify each by real-data round-trip.

Provenance (rust SDK mirrors, explicit serde):
- Sui: `MystenLabs/sui-rust-sdk` → `crates/sui-sdk-types/src/transaction/mod.rs`
  (+ `crypto/mod.rs` for `Jwk`/`JwkId`, `object.rs` for `GenesisObject`).
- IOTA: `iotaledger/iota-rust-sdk` → `crates/iota-sdk-types/src/transaction/mod.rs`
  (+ `crypto/randomness_round.rs`).

Scalar mapping → use existing sentio types: `EpochId` / `ProtocolVersion` /
`CheckpointTimestamp` / `Version` / `u64` → `Number`; `Vec<u8>` → length-prefixed
bytes; `Digest` → existing `Digest` (length-prefixed 32 bytes); `Address` /
`ObjectId` → existing `Address` / `ObjectID`. (The existing `Address`/`ObjectID`/
`Digest` already round-trip via ProgrammableTransaction — reuse them.)

`serde(with = "...ReadableDisplay" / "...ReadableBase64Encoded")` only affects
the human-readable (JSON) form; the **BCS** form is the underlying type. The
`#[non_exhaustive]` upstream enums mean new variants can appear — model unknown
variants to error, not silently pass (CLAUDE.md).

## `TransactionKind` enum — variant indices (BCS)

| idx | Sui                              | IOTA                                   |
|----:|----------------------------------|----------------------------------------|
| 0   | ProgrammableTransaction          | Programmable                           |
| 1   | ChangeEpoch (deprecated)         | Genesis                                |
| 2   | Genesis                          | ConsensusCommitPrologueV1              |
| 3   | ConsensusCommitPrologue (=V1)    | AuthenticatorStateUpdateV1Deprecated *(no payload)* |
| 4   | AuthenticatorStateUpdate         | EndOfEpoch(Vec<EndOfEpochTransactionKind>) |
| 5   | EndOfEpoch(Vec<…>)               | RandomnessStateUpdate                  |
| 6   | RandomnessStateUpdate            | —                                      |
| 7   | ConsensusCommitPrologueV2        | —                                      |
| 8   | ConsensusCommitPrologueV3        | —                                      |
| 9   | ConsensusCommitPrologueV4        | —                                      |
| 10  | ProgrammableSystemTransaction(ProgrammableTransaction) | —                |

→ tag the Go union with `enumNum[sui]=…,enumNum[iota]=…` accordingly. Note Sui's
`ProgrammableSystemTransaction` (idx 10) is **missing** in the current Go type;
the current `ConsensusCommitPrologueV1` field (Go idx 10) is wrong for both
chains and should become the IOTA-only `enumNum[iota]=2` variant.

## ConsensusCommitPrologue family

Sui keeps four structs; IOTA has a single `ConsensusCommitPrologueV1` equal in
shape to Sui's V3.

```
Sui ConsensusCommitPrologue   (idx3): epoch u64, round u64, commit_timestamp_ms u64
Sui ConsensusCommitPrologueV2 (idx7): + consensus_commit_digest Digest
Sui ConsensusCommitPrologueV3 (idx8): epoch, round, sub_dag_index Option<u64>,
                                      commit_timestamp_ms, consensus_commit_digest,
                                      consensus_determined_version_assignments
Sui ConsensusCommitPrologueV4 (idx9): …V3 + additional_state_digest Digest
IOTA ConsensusCommitPrologueV1 (idx2): epoch, round, sub_dag_index Option<u64>,
                                      commit_timestamp_ms, consensus_commit_digest,
                                      consensus_determined_version_assignments
```

Current Go gap: V3/V4 miss `consensus_determined_version_assignments`; IOTA V1
missing entirely (the Go `ConsensusCommitPrologueV1` lacks `sub_dag_index` and
`consensus_determined_version_assignments`).

## ConsensusDeterminedVersionAssignments (enum)

```
Sui:  0 CanceledTransactions   { canceled_transactions: Vec<CanceledTransaction> }
      1 CanceledTransactionsV2 { canceled_transactions: Vec<CanceledTransactionV2> }
IOTA: 0 CancelledTransactions  { cancelled_transactions: Vec<CancelledTransaction> }   (note spelling)

CanceledTransaction      = digest Digest, version_assignments Vec<VersionAssignment>
VersionAssignment        = object_id Address, version Version
CanceledTransactionV2    = digest Digest, version_assignments Vec<VersionAssignmentV2>   (Sui only)
VersionAssignmentV2      = object_id Address, start_version Version, version Version       (Sui only)
```
DIFF: Sui has the V2 cancelled-tx variant (idx 1); IOTA has only idx 0. Same BCS
index 0 has compatible payload (digest + (object_id,version) pairs) → no conflict.
Not currently modeled at all (new type).

## RandomnessStateUpdate (both — same BCS layout)

```
epoch u64, randomness_round u64, random_bytes Vec<u8>,
randomness_obj_initial_shared_version u64
```
(IOTA: `randomness_round: RandomnessRound(u64)`, `…shared_version: Version(u64)` —
same bytes.) Current Go: **empty** (`// TODO`) — add all four fields.

## GenesisTransaction  — DIFF

```
Sui : objects Vec<GenesisObject>
IOTA: objects Vec<GenesisObject>, events Vec<Event>     (IOTA has extra trailing events)
```
Current Go: empty `{}`. `GenesisObject`/`Event` are deep (object/event types) —
fetch when implementing Genesis (rare, checkpoint 0 only); lowest priority.

## AuthenticatorStateUpdate (Sui) / AuthenticatorStateUpdateV1Deprecated (IOTA)

```
Sui AuthenticatorStateUpdate (idx4):
    epoch u64, round u64, new_active_jwks Vec<ActiveJwk>,
    authenticator_obj_initial_shared_version u64
ActiveJwk = jwk_id JwkId, jwk Jwk, epoch u64
JwkId     = iss String, kid String
Jwk       = kty String, e String, n String, alg String
AuthenticatorStateExpire = min_epoch u64, authenticator_object_initial_shared_version u64
IOTA: AuthenticatorStateUpdateV1Deprecated (idx3) — NO payload (unit variant)
```
Current Go `AuthenticatorStateUpdate` has the fields and an `ActiveJwk` JSON
struct already; verify the JwkId/Jwk BCS field order matches (iss,kid / kty,e,n,alg).

## EndOfEpochTransactionKind (enum) — DIFF (disjoint variant sets)

```
Sui  (idx): 0 ChangeEpoch(ChangeEpoch)         1 AuthenticatorStateCreate
            2 AuthenticatorStateExpire(…)       3 RandomnessStateCreate
            4 DenyListStateCreate               5 BridgeStateCreate{ chain_id Digest }
            6 BridgeCommitteeInit{ bridge_object_version u64 }
            7 StoreExecutionTimeObservations(ExecutionTimeObservations)
            8 AccumulatorRootCreate             9 CoinRegistryCreate
            10 DisplayRegistryCreate            11 AddressAliasStateCreate
            12 WriteAccumulatorStorageCost{ storage_cost u64 }
IOTA (idx): 0 ChangeEpoch  1 ChangeEpochV2  2 ChangeEpochV3  3 ChangeEpochV4
```
So `EndOfEpochTransactionKind` is itself a divergent enum — needs `enumNum[sui]/
[iota]` tags too. (The current Go `EndOfEpochTransactionSingle` mixes both chains'
variants by JSON name; for BCS it needs per-chain indices.)
`ExecutionTimeObservations` = enum { 0 V1(Vec<(ExecutionTimeObservationKey,
Vec<ValidatorExecutionTimeObservation>)>) } — deep, only for Sui EndOfEpoch.

## ChangeEpoch family

```
Sui ChangeEpoch: epoch, protocol_version, storage_charge, computation_charge,
                 storage_rebate, non_refundable_storage_fee, epoch_start_timestamp_ms,
                 system_packages Vec<SystemPackage>
IOTA ChangeEpoch    = same 8 fields
IOTA ChangeEpochV2  = + computation_charge_burned (after computation_charge)
IOTA ChangeEpochV3  = …V2 + eligible_active_validators Vec<u64>
IOTA ChangeEpochV4  = …V3 + scores Vec<u64>, adjust_rewards_by_score bool
SystemPackage = version Version, modules Vec<Vec<u8>>, dependencies Vec<Address|ObjectId>
```
DIFF: IOTA's V2+ insert `computation_charge_burned` and append validator fields.
Current Go `ChangeEpoch` matches Sui (with several `json:"-"` BCS-only fields,
already handled by DeriveAux); `ChangeEpochV2` is a stub (“TODO add more fields”)
and must follow IOTA V2/V3/V4 exactly.

## Implementation priority (frequency / verifiability)

1. ConsensusCommitPrologue V1–V4 + IOTA V1 + `ProgrammableSystemTransaction` +
   `ConsensusDeterminedVersionAssignments`/Canceled(Cancelled)Transaction/
   VersionAssignment — every checkpoint, both chains, easy to sample.
2. RandomnessStateUpdate — frequent.
3. EndOfEpochTransactionKind / ChangeEpoch(V2–V4) / AuthenticatorStateUpdate —
   epoch boundaries (sample specific checkpoints).
4. Genesis (GenesisObject/Event) — checkpoint 0 only; lowest priority.
