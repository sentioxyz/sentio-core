# `chain/sui/types` test data

Real transaction samples used by `transaction_kind_roundtrip_test.go` and
`transaction_data_v1_test.go` to validate BCS decode/re-encode round-trips.
Samples are split by source chain: `sui/` (Sui) and `iota/` (IOTA). Each
per-kind file is the verbatim json-rpc `sui_getTransactionBlock` /
`iota_getTransactionBlock` result (`showInput` + `showRawInput`), so
`rawTransaction` is the canonical BCS the round-trip is checked against.

Captured via `kubectl port-forward` to the internal full nodes
(`sui-mainnet-a-0`, `sui-testnet-a-0`, `iota-testnet-0`). Rare/historical kinds
were located by querying the `chain-part-b` ClickHouse `<chain>.transactions`
tables for `kind` + `checkpoint` + `digest`, then fetched from the node.

## Files

| file | chain/net | kind (json) | BCS variant | checkpoint | BCS validated |
|------|-----------|-------------|-------------|------------|---------------|
| `sui/programmable.json` | sui mainnet | ProgrammableTransaction | 0 | 2771 | yes |
| `sui/change-epoch.json` | sui mainnet | ChangeEpoch | 1 | 2771 | yes |
| `sui/consensus-commit-prologue.json` | sui mainnet | ConsensusCommitPrologue (legacy V1) | 3 | 2000000 | yes |
| `sui/consensus-commit-prologue-v2.json` | sui mainnet | ConsensusCommitPrologueV2 | 7 | 29999800 | yes |
| `sui/consensus-commit-prologue-v3.json` | sui mainnet | ConsensusCommitPrologueV3 | 8 | 45815591 | yes |
| `sui/consensus-commit-prologue-v4.json` | sui testnet | ConsensusCommitPrologueV4 | 9 | 346673896 | yes |
| `sui/randomness-state-update.json` | sui testnet | RandomnessStateUpdate | 6 | 346679383 | yes |
| `sui/authenticator-state-update.json` | sui mainnet | AuthenticatorStateUpdate | 4 | 285177209 | yes |
| `sui/end-of-epoch.json` | sui mainnet | EndOfEpochTransaction (ChangeEpoch) | 5 | 11367491 | yes |
| `sui/genesis.json` | sui mainnet | Genesis | 2 | 0 | no (payload unmodeled) |
| `sui/transactions-bundle.json` | sui mainnet | curated bundle of 60 replies (diverse Programmable + a few system txs + 1 errored tx) | — | early epochs | yes (per-tx) |
| `iota/programmable.json` | iota testnet | ProgrammableTransaction | 0 | 225943757 | yes |
| `iota/consensus-commit-prologue-v1.json` | iota testnet | ConsensusCommitPrologueV1 | 2 | 225937125 | yes |
| `iota/randomness-state-update.json` | iota testnet | RandomnessStateUpdate | 5 | 225943753 | yes |
| `iota/end-of-epoch.json` | iota testnet | EndOfEpochTransaction (ChangeEpochV4) | 4 | 225691396 | yes |
| `iota/genesis.json` | iota testnet | Genesis | 1 | 0 | no (payload unmodeled) |

Notes:

- IOTA's json-rpc reports BCS `ChangeEpoch` V2/V3/V4 all under one json kind
  `"ChangeEpochV2"`, distinguished by which optional fields appear
  (`scores` ⇒ V4, `eligible_active_validators` only ⇒ V3, neither ⇒ V2). The
  iota end-of-epoch sample is actually BCS `ChangeEpochV4`.
- `Genesis` is the only kind left in `uncompletedKindsByVariation`: its
  `GenesisTransaction` payload (every genesis object) is not modeled, so the
  samples are trimmed (object list shortened, `rawTransaction` dropped) and only
  checked for json decode + `Kind()`.
- `ProgrammableSystemTransaction` (Sui variant 10) is modeled but has no sample:
  it does not appear in current sui mainnet/testnet data.

## Refreshing / adding a sample

1. `kubectl port-forward -n nodes pod/<node> 9000:9000`
2. Locate a checkpoint with the kind (recent checkpoint, or `chain-part-b`
   `SELECT checkpoint, digest FROM \`<chain>.transactions\` WHERE kind='...'`).
3. Fetch the tx with `{"showInput": true, "showRawInput": true}` and save the
   `result` object as `testdata/<chain>/<kebab-case-kind>.json` (json filenames
   must be kebab-case per the repo lint).
4. Add a row to the table in `transaction_kind_roundtrip_test.go`.
