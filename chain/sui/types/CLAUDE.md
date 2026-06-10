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
`uncompletedKinds` (`chain/sui/extserver.go`) so it is **skipped** rather than
failing slot loading. The long-term goal is to complete the types and empty that
list. See `bcs_enum_selector_design.md` for the dual-chain plan.

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

Fetch source without auth via the GitHub raw URL or `gh api repos/<org>/<repo>/contents/<path> --jq .content | base64 -d`.

## Two wire paths: BCS *and* JSON

A transaction reaches `TransactionResponseV1` two ways, and **both** can diverge
per chain:

- **BCS** — `rawTransaction` is BCS-decoded by `serde` and must re-encode
  byte-for-byte (`TxSanityCheck`). Strict; this is what `uncompletedKinds` guards.
- **JSON** — the json-rpc reply is `json.Unmarshal`ed into the same types
  (`getSlot`), dispatching on a `kind` discriminator (see
  `TransactionKind.UnmarshalJSON`). `encoding/json` is lenient (unknown keys
  ignored, absent keys zero, no round-trip check), so it usually tolerates a
  union struct — but the **same `kind` name can mean different payloads on Sui
  vs IOTA**, which a single name→field dispatch cannot express.

Neither `UnmarshalBCS(r)` nor `UnmarshalJSON(data)` receives a chain, so chain
routing must be injected at a chain-aware entry point (the decoder selector for
BCS; a wrapper like `UnmarshalTransaction(raw, selector)` for JSON). When you
complete a kind, verify **both** paths against real sui/iota replies — see
`bcs_enum_selector_design.md` §9 for the JSON-path rules.

## Sui and IOTA are NOT the same enum layout

`TransactionKind` (and possibly other enums) have **different variant orders and
payloads** on Sui vs IOTA — see the table in `bcs_enum_selector_design.md`. Do
not assume a single position-based struct can serve both. The planned mechanism
is per-field `bcs:"enumNum[sui]=..,enumNum[iota]=.."` tags + a selector on the
decoder/encoder. Until that lands, only variants that coincide (programmable =
index 0 on both) are safe to decode chain-agnostically; the rest stay in
`uncompletedKinds`.

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
   `uncompletedKinds` (`chain/sui/extserver.go`) and verify the super node /
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
- [ ] `uncompletedKinds` updated if a kind became exact.
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
