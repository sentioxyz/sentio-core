# CLAUDE.md — `chain/sui/types`

This package mirrors Sui/IOTA transaction & object types so they can be decoded
from and **re-encoded byte-for-byte to** BCS. Read this before changing any type
here — small mistakes silently break consensus-critical round-tripping.

## The invariant that governs everything

`TxSanityCheck` (`chain/sui/rpc_types.go`) decodes a transaction's raw BCS,
re-encodes it, and requires `bytes.Equal(reEncoded, rawTransaction)`. Therefore
every struct/enum that participates in `SenderSignedData` must match the upstream
Rust definition **exactly**: field order, field types, optionality, and enum
variant indices. "Looks decoded fine" is not enough — it must re-encode to the
identical bytes.

When a type is not yet exact, its transaction kind is added to
`uncompletedKindsByVariation` (`chain/sui/extserver.go`) so it is **skipped**
(stored from the json reply without BCS validation) rather than failing slot
loading. That skip is only safe when nothing *served* is derived from the tx BCS.
Today the only entry is `Genesis` on both chains (its object changes come from the
reply's `objectChanges`, not the tx BCS, and its `GenesisTransaction` payload is
unmodeled). Every other kind is modeled per-chain and validated by a real-data
round-trip (`transaction_kind_roundtrip_test.go` + the transactions bundle), so a
BCS mismatch there must halt slot loading, not be skipped. See
`bcs_enum_selector_design.md` for the dual-chain mechanism.

## BCS facts you must keep in mind

- **Non-self-describing.** An `enum` is `ULEB128(variant_index) + payload`. There
  is no field name on the wire; the decoder infers the variant from the index.
- **`Option<T>`** = 1 byte (`0`=None, `1`=Some) then `T` if Some.
- **`Vec<T>` / `vec<u8>`** = `ULEB128(len)` then elements.
- **Fixed array `[u8; N]`** = N raw bytes, **no** length prefix… *unless* the
  field carries `#[serde_as(as = "... Bytes ...")]`, which routes through
  `serialize_bytes` and **does** add a ULEB128 length prefix. This bit us with
  `Digest`/`ChainIdentifier` (`Readable<Base58, Bytes>` ⇒ `0x20` + 32 bytes).
  **Always confirm against real bytes, not prose.**
- **`#[serde(with = "...ReadableDisplay")]`** etc. only change the *human-readable*
  (JSON) form; in BCS the field serializes as its underlying numeric/byte type.
- Integers are little-endian; `u64`→8 bytes, `u32`→4 bytes.

## How this maps to the Go code

- `serde` (`chain/sui/types/serde`) does reflection-based BCS. Enums are Go
  structs of pointer/interface fields implementing `IsBcsEnum()`; by default the
  **field position is the variant index**.
- A type may instead implement `MarshalBCS`/`UnmarshalBCS` for hand-written BCS
  (e.g. `TransactionExpiration`, `IntentMessage`). Use this when the reflection
  default can't express the layout.
- `json:"-"` hides a field from JSON while keeping it in BCS — common for fields
  the platform doesn't expose but BCS round-trip still requires (e.g.
  `TransactionDataV1.Expiration`).

## Where the upstream definitions live

- **Sui** (`MystenLabs/sui`): `crates/sui-types/src/transaction.rs`,
  `crates/sui-types/src/digests.rs`, `crates/sui-types/src/authenticator_state.rs`,
  `crates/sui-types/src/messages_consensus.rs`. The leaner SDK mirror
  (`MystenLabs/sui-rust-sdk`, crate `sui-sdk-types`) is often easier to read.
- **IOTA** (`iotaledger/iota-rust-sdk`): crate `iota-sdk-types`, e.g.
  `crates/iota-sdk-types/src/transaction/mod.rs`,
  `crates/iota-sdk-types/src/digest.rs`,
  `crates/iota-sdk-types/src/crypto/randomness_round.rs`. The full node
  (`iotaledger/iota`) crate `iota-types` re-exports these.
- The grpc protos in `github.com/sentioxyz/sui-apis` (`sui/rpc/v2`) reflect the
  *current* shape and are a good cross-check for which fields exist (but proto
  field numbers ≠ BCS variant indices; proto enums are 1-offset because of an
  `UNKNOWN = 0`).
- **Most authoritative for exact BCS layout**: the serde-reflection staged
  snapshots — `MystenLabs/sui` `crates/sui-types/tests/snapshots/format__sui.yaml.snap`
  and `iotaledger/iota` `crates/iota-core/tests/staged/iota.yaml`. They give every
  enum's exact variant index and every struct's field order/types (e.g. the
  `EndOfEpochTransactionKind` index order, which does NOT match the json-rpc-types
  enum order). Prefer these over reading `.rs` declaration order when in doubt.
- For **JSON field completeness**, cross-check the json-rpc-types structs
  (`crates/sui-json-rpc-types/src/sui_transaction.rs`, iota equivalent) — note
  they may rename/merge variants vs BCS (e.g. IOTA collapses ChangeEpoch
  V2/V3/V4 into one `"ChangeEpochV2"` json kind).

Fetch source without auth via the GitHub raw URL or `gh api repos/<org>/<repo>/contents/<path> --jq .content | base64 -d`.

## Sui and IOTA are *variations* of one chain type

Treat Sui and IOTA as two variations of the `sui` chain type (like the EVM
variations). They may **differ** but must not **conflict** — same position, same
type. "Position" = numeric variant index for BCS, key name for JSON. See
`bcs_enum_selector_design.md` §1.2. Assume no conflict by default; only add
chain-specific handling if a real one appears, and document it.

## Two wire paths: BCS *and* JSON

A transaction reaches `TransactionResponseV1` two ways:

- **BCS** — `rawTransaction` is BCS-decoded by `serde` and must re-encode
  byte-for-byte (`TxSanityCheck`). Strict, and the index *positions* genuinely
  differ per variation, so this path needs the per-selector `enumNum` mechanism
  (the decoder/encoder selector). This is what `uncompletedKindsByVariation` guards.
- **JSON** — the json-rpc reply is `json.Unmarshal`ed into the same types
  (`getSlot`), dispatching on a `kind` discriminator (`TransactionKind.UnmarshalJSON`).
  Keyed by names, which don't collide between variations, and `encoding/json` is
  lenient. So a single **union struct** (dispatch = the union of both variations'
  kind names; a shared kind = one identically-typed field set) decodes both —
  **no JSON selector needed**. Only if a real key/type conflict ever appears do
  you inject a chain-aware entry point (see §9 fallback).

When you complete a kind, verify the **BCS** round-trip (selector) and make sure
the **JSON** union covers both variations' kinds/fields, against real sui/iota
replies.

## Sui and IOTA are NOT the same enum layout (BCS)

`TransactionKind` and `EndOfEpochTransactionKind` have **different variant orders
and payloads** on Sui vs IOTA — see the table in `bcs_enum_selector_design.md`.
Do not assume a single position-based struct can serve both, and do not assume
the Go field order equals the BCS variant index. This is handled by per-field
`bcs:"enumNum[sui]=..,enumNum[iota]=.."` tags + a selector on the decoder/encoder
(`NewDecoderForSelector`/`NewEncoderForSelector`), which is implemented and in
use; the selector comes from `ExtServerDimension.variation`. Every divergent kind
is tagged per chain and validated, so always set the index from the authoritative
BCS source (the staged snapshot, below), never from Go field position.

## Auditing for upstream drift — compare every field, not just enum variants

When a transaction fails to round-trip, the instinct is to check enum variant
indices. That is necessary but **not sufficient**. Upstream drift hides in
struct fields just as often:

- a field's *type* changes while its position stays (e.g. `SharedObject.mutable`
  went `bool` → the `SharedObjectMutability` enum; `0x00`/`0x01` stayed
  compatible but a new `0x02` broke the bool);
- a payload's *shape* changes (e.g. `CallArg::FundsWithdrawal` is a `struct` of
  three nested enums, not an enum — mis-modeling it as an enum mis-aligned the
  whole stream and either errored or panicked on a bogus length-prefix);
- a struct gains a trailing field; a `bool` becomes an enum that is bool-compatible
  only for its first two variants.

So a real audit walks the **entire BCS-reachable graph** from `SenderSignedData`
and compares *every type* — struct field order **and** field types, enum variant
indices, custom `MarshalBCS` codecs (`TypeTag`, `Argument`, `IntentMessage`,
`MovePackage`, `TransactionExpiration`) — against the staged snapshot, field by
field. The snapshot is the complete reference: `STRUCT` entries give field order
and types; `ENUM` entries give variant indices; `NEWTYPE`/`TUPLE`/`SEQ`/`OPTION`/
`MAP`/`BYTES` give the exact wire shape (and a nested `TUPLE` is just its elements
concatenated, so it is BCS-equivalent to a flat struct of the same fields).

The reachable set (audit all of these, both chains): `SenderSignedData` →
`SenderSignedTransaction` (`IntentMessage` + `TransactionData` + `[]Signature`) →
`TransactionDataV1` → `{TransactionKind, GasData, TransactionExpiration}` → every
`TransactionKind` payload (`ProgrammableTransaction`, `ChangeEpoch[V2..V4]`,
`ConsensusCommitPrologue[V1..V4]`, `AuthenticatorStateUpdate`→`ActiveJwk`,
`EndOfEpochTransaction`→`EndOfEpochTransactionSingle` payloads,
`RandomnessStateUpdate`) → `CallArg`/`ObjectArg`/`FundsWithdrawal`, `Command`/
`ProgrammableMoveCall`/`Argument`, `TypeTag`/`StructTag`/`MovePackage`,
`ConsensusDeterminedVersionAssignments` payloads, the execution-time-observation
types. Everything else on `TransactionResponseV1` (effects, objectChanges,
events, balanceChanges) comes from the **json reply, not `rawTransaction`**, so it
is out of the BCS round-trip and not part of this audit.

To fetch the snapshots: `gh api repos/MystenLabs/sui/contents/crates/sui-types/tests/snapshots/format__sui.yaml.snap -q .content | base64 -d`
and `gh api repos/iotaledger/iota/contents/crates/iota-core/tests/staged/iota.yaml -q .content | base64 -d`.

## The JSON path has its own authority — audit it separately from BCS

`rawTransaction` (BCS) and the json reply are two independent wire formats; a
field can be right in one and wrong in the other (the `CancelledTransactionsV2`
bug round-tripped fine in BCS but the json shape was an object where the reply is
a positional tuple). So audit JSON against its own source of truth — **NOT** the
BCS snapshot:

- **Sui**: `crates/sui-json-rpc-types/src/sui_transaction.rs` (`Sui*` types) plus,
  for newer system kinds it embeds directly, the core serde in
  `sui-types/src/messages_consensus.rs`.
- **IOTA**: `crates/iota-json-rpc-types/src/iota_transaction.rs` (`Iota*` types).

Walk the whole `TransactionDataV1` json subtree (transaction/sender/gasData; the
kind and every payload; PTB inputs/commands; cancelled-tx assignments; JWKs) and
compare each field's *serde shape*, not just enum variants. Gotchas that bit us
or nearly did:

- **`rename_all` is per-type, not uniform.** `SuiTransactionBlockDataV1` is
  camelCase (`gasData`), but `SuiChangeEpoch` / `SuiConsensusCommitPrologue*` keep
  **snake_case** (`storage_charge`, `commit_timestamp_ms`). `SuiCallArg` /
  `SuiObjectArg` / `SuiPureValue` are camelCase (`valueType`, `initialSharedVersion`).
- **u64 is a json string or number depending on serde_as.** `BigInt<u64>` /
  `DisplayFromStr` ⇒ **string** (model as `Number`). A plain `SequenceNumber`
  with no `serde_as` ⇒ **number** (`SuiObjectRef.version` in gasData,
  `BridgeCommitteeUpdate`, the versions inside cancelled-tx tuples — model as
  `uint64` / emit via `BigInt()`). `AsSequenceNumber` ⇒ **string** (`SuiObjectArg`
  versions). So the same logical "version" is a number in one place and a string
  in another — don't assume.
- **Enum tagging differs.** Internally tagged: `TransactionKind` (`tag="kind"`),
  `SuiCallArg` (`tag="type"`), `SuiObjectArg` (`tag="objectType"`). Externally
  tagged / default: `SuiCommand`, `SuiArgument`, `SuiEndOfEpochTransactionKind`,
  `ConsensusDeterminedVersionAssignments` — units serialize as a **bare string**
  (`"GasCoin"`, `"AuthenticatorStateCreate"`), payloads as `{"Variant": ...}`,
  and several carry **positional-tuple** payloads (`SuiCommand::SplitCoins` etc.,
  cancelled-tx assignments) that need hand-written tuple (un)marshal.
- **json drops fields BCS keeps.** `SuiChangeEpoch` omits protocol_version /
  non_refundable_storage_fee / system_packages; `SuiCommand::Publish`/`Upgrade`
  drop the module bytecodes; `TransactionDataV1` json has no `expiration`; the
  `*_obj_initial_shared_version` and V4 `adjust_rewards_by_score` are BCS-only.
  Mark these `json:"-"` and fill them from BCS in `DeriveAuxInformationFromBCSV1`.
- **json also collapses BCS distinctions.** IOTA reports BCS ChangeEpoch V2/V3/V4
  all under the one `"ChangeEpochV2"` json kind (disambiguated by which optional
  fields appear); `SharedObjectMutability` collapses to a bool.

The kind round-trip test (`transaction_kind_roundtrip_test.go`) asserts json
fidelity for the **full `transaction.data`** (not just the kind), so gasData and
sender are covered — but only for the kinds/sub-shapes a real sample exercises.
Coverage of sample *shapes* is the weak point (the cancelled-tx tuple slipped
through because no sample had a non-empty list and the unit test used `[]`); when
touching a json shape, add a sample that actually exercises it.

## Procedure: updating a type to follow an upstream change

1. **Find the authoritative Rust definition** for the exact version/branch in
   use (mainnet vs testnet may differ). Note declaration order and every field
   type, including `#[serde_as]`/`#[serde(with=...)]` attributes.
2. **Translate to BCS layout**, applying the facts above (Option, Vec, the
   `Bytes` length-prefix gotcha, enum variant indices).
3. If a type diverges between chains, encode the divergence in `bcs` tags rather
   than Go field order. Every tag attribute is per-selector capable with a
   uniform "scoped-over-global" rule (see `bcs_enum_selector_design.md` §3.2):
   `enumNum[sui]=..`/`enumNum[iota]=..` for variant indices, and likewise
   `optional[sel]` / `-[sel]` when a struct field's optionality or presence
   differs per chain. A plain (unscoped) `enumNum`/`optional`/`-` is the default
   for selectors without an override; no tags at all == field-position semantics.
4. **Capture a real sample** of an affected transaction (json-rpc `showRawInput`
   for raw bytes; grpc `GetCheckpoint`/`GetTransaction` for structured ground
   truth). Prefer testnet; for rare kinds, find a specific checkpoint.
5. **Add a round-trip unit test** (table-driven, bytes embedded, no network):
   decode → assert fields == ground truth → re-encode → `bytes.Equal` original.
   Pattern: `transaction_expiration_test.go`.
6. Run `go test -vet=off ./chain/sui/types/...` (the package has pre-existing
   non-constant-format-string vet findings unrelated to this work, so `-vet=off`
   when iterating).
7. If the change makes a kind fully exact for a chain, remove it from
   `uncompletedKindsByVariation` (`chain/sui/extserver.go`) and verify the super node /
   syncer for that chain stops skipping it.
8. Prefer making unknown enum variants **error** rather than silently produce an
   empty value, so the next upstream addition fails loudly instead of corrupting
   a round-trip (see `TransactionExpiration.UnmarshalBCS`).

## Checklist when adding / editing a variant

- [ ] Field order matches upstream declaration order.
- [ ] Types match (watch `Option<T>`, `Vec<u8>`, fixed arrays vs `Bytes`).
- [ ] Enum variant index correct **per chain** (tags, not Go position, for
      divergent enums).
- [ ] `MarshalBCS` and `UnmarshalBCS` agree (symmetric); unknown variants error.
- [ ] Real-data round-trip test added and passing.
- [ ] `uncompletedKindsByVariation` updated if a kind became exact.
- [ ] JSON behavior preserved for fields the platform serves (`json` tags).

## Gotchas seen in practice

- `Digest`/`ChainIdentifier`: length-prefixed 32 bytes in BCS (not raw 32).
- `TransactionExpiration`: 3rd variant `ValidDuring` (index 2) exists now; the
  old decoder silently ignored unknown ids and then panicked on re-encode.
- Accumulator-write `ChangedObject`s (`OUTPUT_OBJECT_STATE_ACCUMULATOR_WRITE`)
  carry no input version/owner/type — don't enrich them (see `getGrpcSlot`).
- `ConsensusCommitPrologue` naming: Sui's index-3 `ConsensusCommitPrologue` *is*
  V1; IOTA's `ConsensusCommitPrologueV1` is index 2 with more fields. Different
  structs — keep them separate.
- `EndOfEpochTransactionKind` BCS variant order ≠ the json-rpc-types enum order:
  use the staged snapshot (BridgeStateCreate=5, BridgeCommitteeInit=6,
  StoreExecutionTimeObservations=7, WriteAccumulatorStorageCost=12). Several of
  its variants are bare-string *units* in the json reply but carry a real BCS
  payload (`StoreExecutionTimeObservations`, `WriteAccumulatorStorageCost`); those
  payloads are `json:"-"` and filled in `DeriveAuxInformationFromBCSV1`. Every
  recent Sui end-of-epoch tx contains them.
- A non-pointer enum-valued struct field is NOT detected as an enum (the
  reflection check needs the field to be a pointer, since `IsBcsEnum` is a
  pointer receiver). Model nested enum fields as pointers (e.g.
  `ExecutionTimeObservation.Key *ExecutionTimeObservationKey`).
