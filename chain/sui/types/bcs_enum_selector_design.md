# Design: per-chain BCS enum selector for Sui / IOTA transaction types

Status: implemented (this document is the design record; the live reference is
the code in `chain/sui/types` + `chain/sui/types/serde` and `CLAUDE.md`).
Scope: `chain/sui/types` and `chain/sui/types/serde`

> Implemented: the per-selector `enumNum`/`optional`/`-` tags + decoder/encoder
> selector described below are in place and in use. All transaction kinds are
> modeled and validated by real-data round-trips except `Genesis` (still in
> `uncompletedKindsByVariation`). Sections in future tense below are kept as the
> original design rationale.

## 1. Background

`chain/sui/types` decodes raw Sui/IOTA transaction bytes (BCS) and must be able
to **re-encode them byte-for-byte**. `TxSanityCheck` (in `chain/sui/rpc_types.go`)
encodes the decoded `SenderSignedData` and asserts it equals the original raw
bytes:

```go
encodedBCS, _ = types.EncodeSenderSignedData(...)
if !bytes.Equal(encodedBCS, tx.RawTransaction.Data()) { /* fail */ }
```

So any field that is mis-typed, mis-ordered, missing, or whose optionality is
wrong makes the round-trip fail. That failure is what `uncompletedKindsByVariation`
(`chain/sui/extserver.go`) works around: transaction kinds whose Go structs are
not yet byte-exact are **skipped** (no `DeriveAuxInformationFromBCSV1` /
`TxSanityCheck`) instead of failing slot loading.

BCS is a **non-self-describing** format. An enum (Rust `enum`) is encoded as a
ULEB128 **variant index** followed by the variant payload. The decoder must know,
per index, which variant/payload to expect — there is no tag name on the wire.

### 1.1 The core problem: Sui and IOTA enums diverge

`TransactionKind` is a BCS enum. Today the Go `TransactionKind` struct is decoded
by `serde` using **field position = variant index** (see `decodeEnum` /
`encodeEnum`). That only works for one chain, because Sui and IOTA assign
different indices (and different payloads) to the same conceptual variant:

| index | Sui `TransactionKind`            | IOTA `TransactionKind`                 |
|------:|----------------------------------|----------------------------------------|
| 0     | ProgrammableTransaction          | Programmable                           |
| 1     | ChangeEpoch                      | Genesis                                |
| 2     | Genesis                          | ConsensusCommitPrologueV1 (6 fields)   |
| 3     | ConsensusCommitPrologue (V1)     | AuthenticatorStateUpdateV1Deprecated   |
| 4     | AuthenticatorStateUpdate         | EndOfEpoch                             |
| 5     | EndOfEpoch                       | RandomnessStateUpdate                  |
| 6     | RandomnessStateUpdate            | —                                      |
| 7     | ConsensusCommitPrologueV2        | —                                      |
| 8     | ConsensusCommitPrologueV3        | —                                      |
| 9     | ConsensusCommitPrologueV4        | —                                      |
| 10    | ProgrammableSystemTransaction    | —                                      |

Sources: Sui `crates/sui-types/src/transaction.rs`; IOTA
`crates/iota-sdk-types/src/transaction/mod.rs`.

Only index `0` (programmable) matches between the two chains — which is why the
common case decodes for both and **every system transaction is in
`uncompletedKindsByVariation`**. The current single struct is implicitly modeled after Sui;
the `ConsensusCommitPrologueV1` field at Go index 10 (commented "iota-mainnet has
this") is actually wrong: IOTA's CCPV1 is index 2, and Sui's index 10 is
`ProgrammableSystemTransaction`.

### 1.2 Guiding principle: Sui and IOTA are *variations* of one chain type

Treat Sui and IOTA as two **variations** of the `sui` chain type — analogous to
the several EVM variations the platform already supports. The working rule:

> Variations may **differ** (a variant/field present in one, absent in the other)
> but must not **conflict** — where *conflict* = the **same position** holding a
> **different-typed value**. For BCS, "position" is the numeric variant index /
> field offset; for JSON it is the key name.

Consequences:
- **BCS**: the index *numbering* itself differs between variations (Sui idx 2 =
  `Genesis`, IOTA idx 2 = `ConsensusCommitPrologueV1`). The per-selector
  `enumNum` tags (§3) let each variation declare its own indices, so this is a
  *diff* resolved by the selector — not an unresolvable conflict.
- **JSON**: keyed by **names**, which do not collide (`"Genesis"`,
  `"ConsensusCommitPrologue"`, `"ConsensusCommitPrologueV1"` are distinct; a kind
  both variations have, e.g. `RandomnessStateUpdate`, uses the same field names
  with the same types). So a single **union struct** decodes both variations'
  JSON — no per-chain JSON routing is needed (see §9).

Adopt "no conflict" as the **default assumption**; do not build machinery for a
conflict until one actually appears. When refactoring `chain/sui/types`, watch
for a real violation (same JSON key / same BCS index with incompatible types)
and only then add chain-specific handling for that one spot.

## 2. Goals / non-goals

Goals:
- Decode/encode `TransactionKind` (and any other divergent enum) correctly for
  **both Sui and IOTA** with a single Go type, byte-exact, so the
  `uncompletedKindsByVariation` skip list can shrink toward empty.
- Keep the divergence **declarative** (in struct tags), not scattered across
  hand-written branches.
- Stay backward compatible: enums without the new tags keep position semantics.

Non-goals:
- Changing how non-enum BCS or self-describing JSON is handled.
- Modeling fields the platform does not need beyond what round-trip requires
  (round-trip requires *all* fields, but JSON-facing types may still hide them
  with `json:"-"`).

## 3. Design

### 3.1 Per-field variant-number tags

Annotate each variant field of a divergent enum with its per-chain variant
index. `serde` already reads the `bcs` struct tag (today only `-` / `optional`);
extend it:

```go
type TransactionKind struct {
    ProgrammableTransaction       *ProgrammableTransaction       `bcs:"enumNum[sui]=0,enumNum[iota]=0"`
    ChangeEpoch                   *ChangeEpoch                   `bcs:"enumNum[sui]=1"`
    Genesis                       *Genesis                       `bcs:"enumNum[sui]=2,enumNum[iota]=1"`
    ConsensusCommitPrologue       *ConsensusCommitPrologue       `bcs:"enumNum[sui]=3"`        // sui V1
    IotaConsensusCommitPrologueV1 *IotaConsensusCommitPrologueV1 `bcs:"enumNum[iota]=2"`       // iota V1 (more fields)
    AuthenticatorStateUpdate      *AuthenticatorStateUpdate      `bcs:"enumNum[sui]=4,enumNum[iota]=3"`
    EndOfEpochTransaction         *EndOfEpochTransaction         `bcs:"enumNum[sui]=5,enumNum[iota]=4"`
    RandomnessStateUpdate         *RandomnessStateUpdate         `bcs:"enumNum[sui]=6,enumNum[iota]=5"`
    ConsensusCommitPrologueV2     *ConsensusCommitPrologueV2     `bcs:"enumNum[sui]=7"`
    ConsensusCommitPrologueV3     *ConsensusCommitPrologueV3     `bcs:"enumNum[sui]=8"`
    ConsensusCommitPrologueV4     *ConsensusCommitPrologueV4     `bcs:"enumNum[sui]=9"`
    ProgrammableSystemTransaction *ProgrammableTransaction       `bcs:"enumNum[sui]=10"`
}
```

The struct is the **union** of both chains' variants; each field declares which
selector(s) it appears under and at what index. A field with no entry for a
selector simply does not exist on that chain (e.g. `ChangeEpoch` is sui-only).

This naturally handles the "same index, different payload" case: sui index 2 is
`Genesis`, iota index 2 is `IotaConsensusCommitPrologueV1` — two distinct Go
fields/types, each with its own per-selector index.

### 3.2 Tag grammar & `fieldTag` (systematic, per-selector for every attribute)

The divergence between chains is not limited to enum indices. The same struct
field can legitimately differ per chain in other ways too, e.g. it is
`Option<T>` on one chain but `T` on the other (different optionality), or it only
exists in one chain's struct layout (present vs ignored). So **every** tag
attribute — not just `enumNum` — must be expressible per selector, with a single
uniform resolution rule. (Making only `enumNum` selector-aware while `ignore` /
`optional` stay global would be inconsistent and would break the first time an
upstream change diverges on optionality.)

#### Grammar

`bcs:"<seg>,<seg>,..."`. Each segment is an attribute, optionally scoped to a
selector with `[selector]`, optionally with a value:

```
<attr>            global (applies to every selector unless overridden)
<attr>[<sel>]     scoped to one selector
<attr>=<v>        global, with value
<attr>[<sel>]=<v> scoped, with value
```

Attributes:

| segment                | meaning                                             |
|------------------------|-----------------------------------------------------|
| `-`, `-[sui]`          | ignore the field in BCS (global / per selector)     |
| `optional`, `optional[iota]` | field is `Option<T>` (global / per selector)  |
| `enumNum=3`, `enumNum[sui]=2` | enum variant index (global / per selector)   |

`<selector>` is an opaque string. The `sui` package uses `"sui"` / `"iota"`;
`serde` hard-codes no chain semantics — it only matches the selector string
carried by the decoder/encoder.

#### Resolution rule (uniform for all attributes)

For a decode/encode running under selector `S`, an attribute's effective value is:

1. the `[S]`-scoped entry if present, else
2. the unscoped (global) entry if present, else
3. unset (→ `ignore=false`, `optional=false`, `enumNum` = none).

The default selector `""` (position-based decoder/encoder) only ever sees
unscoped entries, so today's structs behave unchanged.

#### Representation

Replace the bitmask `int64` returned by `parseTagValue` with a `fieldTag` whose
every attribute is a `selectorValue[T]` (unscoped default + per-selector
overrides). One generic type drives parsing and lookup for all attributes, and
new attributes are added by appending a field — no special-casing.

```go
// selectorValue holds an optional unscoped default plus per-selector overrides
// for a single tag attribute.
type selectorValue[T any] struct {
    global     *T           // value of the unscoped form, nil if not given
    bySelector map[string]T // selector -> value
}

// resolve returns the effective value for selector and whether it is set,
// applying scoped-over-global precedence.
func (sv selectorValue[T]) resolve(selector string) (v T, ok bool) {
    if x, found := sv.bySelector[selector]; found {
        return x, true
    }
    if sv.global != nil {
        return *sv.global, true
    }
    return v, false
}

type fieldTag struct {
    ignore   selectorValue[bool] // set true via "-" / "-[sel]"
    optional selectorValue[bool] // set true via "optional" / "optional[sel]"
    enumNum  selectorValue[int]  // "enumNum=.." / "enumNum[sel]=.."
}

func (t fieldTag) isIgnored(sel string) bool  { v, ok := t.ignore.resolve(sel);   return ok && v }
func (t fieldTag) isOptional(sel string) bool { v, ok := t.optional.resolve(sel); return ok && v }
func (t fieldTag) variantNum(sel string) (int, bool) { return t.enumNum.resolve(sel) }
```

`parseTag(tag string) (fieldTag, error)` splits on `,`, and for each segment
parses `attr`, optional `[sel]`, optional `=value`, filling the right
`selectorValue`. Unknown attributes must still **error** (typos fail loudly).
The three current call sites (`encodeEnum`, the struct-field encode/decode loops)
switch from the bitmask to `fieldTag` + the `isIgnored/isOptional/variantNum`
helpers, threading the decoder/encoder's selector.

> Note: `enumNum` is only consulted for enum (`IsBcsEnum`) types; `ignore` /
> `optional` apply to plain struct fields. They share the same parser and the
> same resolution rule, which is the point — one systematic mechanism, not three
> ad-hoc ones.

#### `enumNum` is enum-scoped — partial-tagging rules

`optional` / `ignore` are **per-field independent**. `enumNum` is different: it
defines the variant→field mapping for the **whole enum**, so the fields relate to
each other and "some tagged, some not" needs an explicit, unambiguous meaning.
The rules are evaluated **per enum type `T` and per selector `S`** (cache the
result per `(T, S)`):

Let `Resolved(T, S)` = the set of fields whose `enumNum` resolves under `S`
(scoped `[S]` or global), and `HasAnyEnumNum(T)` = `T` has at least one field
with any `enumNum` entry at all.

1. **Pure legacy type** — `HasAnyEnumNum(T)` is false → **position mode**:
   variant index = exported-field position (today's behavior; also what the
   default selector `""` gets). Nothing changes for `CallArg`, `Command`, etc.

2. **Tagged type, selector resolves variants** — `HasAnyEnumNum(T)` and
   `Resolved(T, S)` is non-empty → **tag mode for `S`**:
   - the variants under `S` are **exactly** `Resolved(T, S)`;
   - a field **not** in `Resolved(T, S)` is **absent on `S`** — it is *not* a
     decodable/encodable variant there. There is **no** position fallback mixed
     in. (This is how "variant exists on Sui but not IOTA" is expressed, e.g.
     `ChangeEpoch` has `enumNum[sui]=1` and no iota entry.)
   - indices in `Resolved(T, S)` must be **unique**; a duplicate is a definition
     bug → error at build-of-map time.
   - decode: wire index not in the map → error `variant %d not defined for
     selector %q on %s`. encode: the non-nil field is absent on `S` → error
     `variant %s not valid for selector %q`.

3. **Tagged type, selector resolves nothing** — `HasAnyEnumNum(T)` but
   `Resolved(T, S)` is empty → **error**, do *not* silently fall back to
   position mode: `type %s uses enumNum tags but selector %q resolves no
   variant`. This catches decoding a per-chain enum under the default `""` or an
   unknown selector (e.g. `TransactionKind` must always be decoded with a
   concrete `"sui"`/`"iota"` selector).

Consequences / guidance:
- Mixing tagged and untagged fields *within one selector* is **not** "untagged =
  position"; it is "untagged = absent on that selector". This is intentional and
  is the mechanism for chain-specific variants.
- Forgetting an `enumNum[S]` on a field that should exist on `S` does not corrupt
  silently — it surfaces as a decode/encode error and is caught by the
  per-variant round-trip tests.
- Prefer expressing "this variant doesn't exist on chain `S`" by **omitting** its
  `enumNum[S]` (rather than `-[S]`); reserve `ignore` for non-variant struct
  fields.
- A global `enumNum=n` (unscoped) makes that field a variant under **every**
  selector including `""`; only use it for enums whose layout is genuinely
  chain-independent. For divergent enums use scoped `enumNum[sui]/[iota]` only,
  which keeps `""` in (unused) position mode and forces callers to pass a real
  selector (rule 3).

### 3.3 Selector on Decoder / Encoder

Add an opaque selector to both, defaulting to `""` (position-based, fully
backward compatible):

```go
type Decoder struct { r io.Reader; selector string }
type Encoder struct { w io.Writer; selector string }

func NewDecoderForSelector(r io.Reader, selector string) *Decoder { ... }
func NewEncoderForSelector(w io.Writer, selector string) *Encoder { ... }
```

The selector lives on the decoder/encoder instance, so it **propagates
automatically** through the recursive `decode`/`encode` calls (nested enums see
the same selector).

### 3.4 decodeEnum / encodeEnum changes

`decodeEnum`:
1. read `enumID` (ULEB128).
2. if any field of the struct has `enumNum` for the current selector, build
   `map[variantIndex]fieldIndex` for that selector (cache per `(type, selector)`),
   look up `enumID`, decode that field.
3. else fall back to current behavior (field position).
4. selector set but `enumID` not in the map → error
   `"variant %d not defined for selector %q on type %s"`.

`encodeEnum`:
1. find the single non-nil field.
2. if it has `enumNum` for the current selector → write that number.
3. else if the struct has **no** enum tags at all → write field position (current).
4. else (tagged type, but this field has no number for the selector) → error
   `"variant %s not valid for selector %q"`.

### 3.5 Plumbing the selector from the call site

The chain is known where slots are loaded (the sui `Client` carries
`SpecialMethodPrefix == "iota"`; the dimension knows its network). Thread it down:

- `DecodeSenderSignedData(b, selector)` / `EncodeSenderSignedData(data, selector)`
  (keep thin position-based wrappers for callers that don't care).
- `DeriveAuxInformationFromBCSV1(data, rawTransaction, selector)`.
- `TxSanityCheck(tx, selector)` — must use the **same** selector for the
  re-encode, otherwise the byte-exact compare cannot pass.
- `getSlot` passes the dimension's chain.

## 4. Payload modeling (still required per variant)

The tag mechanism only chooses **which** variant; each variant's payload struct
must still match the upstream Rust definition byte-for-byte. See
[`CLAUDE.md`](./CLAUDE.md) for the field-mapping rules and gotchas. The recently
added `TransactionExpiration::ValidDuring` (variant 2) in `transaction.go` is the
reference example, including the `ChainIdentifier`/`Digest` length-prefix gotcha
(`serde_as(as = "Readable<Base58, Bytes>")` ⇒ BCS `serialize_bytes` ⇒ ULEB128
length prefix).

Known payloads to complete (from upstream, must be verified by round-trip):
- Sui `ConsensusCommitPrologueV2/V3/V4`: add
  `consensus_determined_version_assignments: ConsensusDeterminedVersionAssignments`
  (+ confirm `sub_dag_index` presence per version).
- `ConsensusDeterminedVersionAssignments` enum (+ `CancelledTransaction`).
- IOTA `ConsensusCommitPrologueV1`: `epoch, round, sub_dag_index: Option<u64>,
  commit_timestamp_ms, consensus_commit_digest, consensus_determined_version_assignments`.
- `RandomnessStateUpdate`: `epoch, randomness_round, random_bytes, randomness_obj_initial_shared_version`.
- `Genesis`: `objects: Vec<GenesisObject>`.
- `ProgrammableSystemTransaction` (sui index 10): same payload as
  `ProgrammableTransaction`.

## 5. Emptying `uncompletedKindsByVariation`

For each kind, once its payload is byte-exact AND its per-chain `enumNum` tags
are set AND verified by a real-data round-trip, remove it from
`uncompletedKindsByVariation`. Suggested order by sampling ease:
1. ConsensusCommitPrologue family + `ProgrammableSystemTransaction` + enum-order
   fix (present in every checkpoint — abundant samples, both chains).
2. `RandomnessStateUpdate` (frequent).
3. `Genesis` (checkpoint 0 only), `EndOfEpochTransaction`, epoch-boundary kinds
   (sample specific checkpoints).

## 6. Verification methodology

For each variant, validate against **real chain bytes** (do not trust prose):
1. fetch a real tx of that kind:
   - json-rpc `sui_getTransactionBlock` with `showRawInput` → base64 raw tx bytes;
   - grpc `LedgerService.GetCheckpoint` (ReadMask `*`) → structured field values
     (ground truth) for cross-checking.
   (Fetch from any Sui/IOTA full node that retains the checkpoint.)
2. add a table-driven unit test that decodes the raw bytes, asserts the decoded
   fields equal the grpc ground truth, then re-encodes and asserts
   `bytes.Equal` with the original (this is exactly what `TxSanityCheck`
   requires). See `transaction_expiration_test.go` for the pattern.
3. tests must not depend on a live node — embed the captured bytes as constants.

## 7. Backward compatibility & risks

- Enums without `enumNum` tags are unchanged (position-based): `CallArg`,
  `Command`, `ObjectArg`, and `TransactionExpiration` (custom `MarshalBCS`).
- Default selector `""` reproduces today's behavior exactly.
- Risk: a wrong `enumNum` or payload silently corrupts round-trip → mitigated by
  per-variant real-data round-trip tests and by erroring on unknown variants.
- Risk: serde gains a generic "selector" concept — acceptable; it stays
  domain-agnostic (opaque string), the `sui` package owns the `"sui"`/`"iota"`
  values.

## 8. Rollout

- PR 1: serde selector + tag grammar + `decodeEnum`/`encodeEnum` (with unit
  tests proving position-mode unchanged); no behavior change to callers yet.
- PR 2: tag `TransactionKind`, add `ProgrammableSystemTransaction`, fix the
  enum, complete the Sui CCP family payloads, thread the selector from `getSlot`
  for Sui, drop the relevant entries from `uncompletedKindsByVariation`. Verify with Sui
  samples.
- PR 3: IOTA variant payloads (`IotaConsensusCommitPrologueV1`,
  `ConsensusDeterminedVersionAssignments`, …), iota selector plumbing, verify
  with IOTA samples, drop iota-applicable entries.
- Later PRs: `Genesis`, `EndOfEpochTransaction`, remaining kinds.

## 9. The JSON path diverges too

Transactions reach `types.TransactionResponseV1` via `json.Unmarshal(rawTx, &tx)`
from the node's json-rpc reply (`chain/sui/extserver.go`, `getSlot`), **not** only
via BCS. `TransactionKind.UnmarshalJSON` (and `EndOfEpochTransactionSingle`)
dispatch on a discriminator — the `kind` string / object key — to a Go field:

```go
switch j.Kind {
case "ConsensusCommitPrologueV1":
    return json.Unmarshal(data, &s.ConsensusCommitPrologueV1)
...
}
```

Different variations may use **different discriminator names for the analogous
tx** (e.g. Sui `"ConsensusCommitPrologue"` vs IOTA `"ConsensusCommitPrologueV1"`)
and one variation may have kinds/fields the other lacks. Under §1.2 these are
*diffs*, not conflicts.

### Default: a single union struct (no JSON selector)

By the §1.2 principle, the variations do **not** conflict on JSON keys — distinct
kinds use distinct names, and a kind both have uses the same field names with the
same types. JSON is also self-describing and `encoding/json` is lenient (unknown
keys ignored, absent keys zero, no byte-exact check). So:

- Keep **one `TransactionKind` (and friends)** whose `UnmarshalJSON` dispatch is
  the **union** of both variations' discriminator names — each name maps to its
  own Go field (e.g. both `case "ConsensusCommitPrologue"` and
  `case "ConsensusCommitPrologueV1"`). A reply only ever carries the names of its
  own variation; the others stay nil.
- For a kind shared by both variations, one struct with the (identical-typed)
  field set decodes both.
- **No chain selector is needed on the JSON path.** `UnmarshalJSON(data)` having
  no chain parameter is fine, because the wire is self-describing and
  conflict-free.

This is unlike BCS, where the numeric index *positions* genuinely differ and the
selector (§3) is required.

### Fallback (only if the no-conflict assumption breaks)

If a real conflict ever appears — the **same** discriminator name needing a
different Go type per variation, or the **same** json key carrying a
different-typed value — then (and only then) inject a selector at a chain-aware
entry point, e.g. `UnmarshalTransaction(raw, selector)` called from `getSlot`,
that peeks the discriminator and routes to the variation-specific type; likewise
`MarshalJSON` would need the selector to emit the expected name. Record any such
conflict in `CLAUDE.md`. Do not build this until it is actually needed.

### Scope note

These divergent kinds are in `uncompletedKindsByVariation` (BCS-skipped today) but their
**JSON values are still decoded and served to drivers**. Completing a kind thus
has two independent correctness aspects — the BCS round-trip (§3–§4 selector) and
JSON field routing (this section) — and both must be verified against real
sui/iota json-rpc replies.
