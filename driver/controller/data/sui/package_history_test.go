package sui

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/chain/sui"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// fakeLedger is a transport-neutral packageHistoryLedger backed by in-memory data,
// so resolvePackageHistory's UpgradeCap version-chain walk can be tested without
// RPC. Per the interface contract objectPrevTx is self-sufficient, so the fake
// answers it from `latest`/`past` directly (an absent entry = even the recorded
// history had nothing, e.g. a chain with no history at all).
type fakeLedger struct {
	// latest[objectID] answers the latest-version digest lookup (nil version).
	latest map[string]string
	// past[objectID][version] answers versioned digest lookups.
	past map[string]map[uint64]string
	// changes[txDigest] = the object changes of that tx.
	changes map[string][]packageHistoryChange
	// records[objectID] = the object's recorded change history, ascending by
	// version. lastRecordedChange scans it backwards.
	records map[string][]sui.ObjectChangeRecord
}

// objectPrevTx goes through the real resolveObjectPrevTx so the tests exercise
// the rescue semantics the real ledgers use: the live lookup reads latest/past,
// the recorded history backs it on errObjectUnavailable.
func (f *fakeLedger) objectPrevTx(ctx context.Context, objectID string, version *uint64) (string, error) {
	return resolveObjectPrevTx(ctx, objectID, version,
		func(v *uint64) (string, error) {
			if v == nil {
				if tx, ok := f.latest[objectID]; ok {
					return tx, nil
				}
				return "", errors.Wrapf(errObjectUnavailable, "object %s (latest) not found", objectID)
			}
			if tx, ok := f.past[objectID][*v]; ok {
				return tx, nil
			}
			return "", errors.Wrapf(errObjectUnavailable, "object %s@%d not found", objectID, *v)
		},
		func(maxVersion uint64) (*sui.ObjectChangeRecord, error) {
			return f.lastRecordedChange(ctx, objectID, maxVersion)
		})
}

func (f *fakeLedger) txChanges(_ context.Context, digest string) ([]packageHistoryChange, error) {
	if cs, ok := f.changes[digest]; ok {
		return cs, nil
	}
	return nil, errors.Errorf("tx %s not found", digest)
}

func (f *fakeLedger) lastRecordedChange(
	_ context.Context, objectID string, maxVersion uint64,
) (*sui.ObjectChangeRecord, error) {
	rs := f.records[objectID]
	for i := len(rs) - 1; i >= 0; i-- {
		if maxVersion == 0 || rs[i].ObjectVersion <= maxVersion {
			r := rs[i]
			return &r, nil
		}
	}
	return nil, nil
}

func u64p(v uint64) *uint64 { return &v }

func published(pkgID string) packageHistoryChange {
	return packageHistoryChange{objectID: pkgID, isPublished: true}
}

func capCreated(capID string) packageHistoryChange {
	return packageHistoryChange{objectID: capID, isUpgradeCap: true, isCreated: true}
}

func capMutated(capID string, prevVersion uint64) packageHistoryChange {
	return packageHistoryChange{objectID: capID, isUpgradeCap: true, prevVersion: u64p(prevVersion)}
}

// Scenario: package published as p1 (tx t1, cap created v1), upgraded to p2 (tx
// t2, cap -> v2) and p3 (tx t3, cap -> v3, latest). Querying any version must
// return the full history walked back through the cap change's prevVersion.
func newUpgradeChainLedger() *fakeLedger {
	const cap = "cap"
	return &fakeLedger{
		latest: map[string]string{"p1": "t1", "p2": "t2", "p3": "t3", cap: "t3"},
		past: map[string]map[uint64]string{
			cap: {1: "t1", 2: "t2", 3: "t3"},
		},
		changes: map[string][]packageHistoryChange{
			"t1": {published("p1"), capCreated(cap)},
			"t2": {published("p2"), capMutated(cap, 1)},
			"t3": {published("p3"), capMutated(cap, 2)},
		},
		records: map[string][]sui.ObjectChangeRecord{
			cap: {
				{TxDigest: "t1", Type: "created", ObjectVersion: 1},
				{TxDigest: "t2", Type: "mutated", ObjectVersion: 2, PreviousVersion: u64p(1)},
				{TxDigest: "t3", Type: "mutated", ObjectVersion: 3, PreviousVersion: u64p(2)},
			},
		},
	}
}

func TestResolvePackageHistory(t *testing.T) {
	for _, pkgID := range []string{"p1", "p2", "p3"} {
		t.Run("query_"+pkgID, func(t *testing.T) {
			history, err := resolvePackageHistory(context.Background(), pkgID, newUpgradeChainLedger())
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"p1", "p2", "p3"}, history)
			require.Contains(t, history, pkgID)
		})
	}
}

// A package with no UpgradeCap (e.g. published with an immediately-burned cap, or
// a system package) resolves to just itself.
func TestResolvePackageHistoryNoUpgradeCap(t *testing.T) {
	l := &fakeLedger{
		latest:  map[string]string{"p1": "t1"},
		changes: map[string][]packageHistoryChange{"t1": {published("p1")}},
	}
	history, err := resolvePackageHistory(context.Background(), "p1", l)
	require.NoError(t, err)
	require.Equal(t, []string{"p1"}, history)
}

// A burned UpgradeCap: the latest live lookup fails, and the ledger's objectPrevTx
// rescues the digest from the recorded change history, so the walk still resolves
// the full history.
func TestResolvePackageHistoryBurnedCap(t *testing.T) {
	l := newUpgradeChainLedger()
	delete(l.latest, "cap") // deleted: the live latest lookup fails
	for _, pkgID := range []string{"p1", "p2", "p3"} {
		history, err := resolvePackageHistory(context.Background(), pkgID, l)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"p1", "p2", "p3"}, history)
	}
}

// When the cap is gone and nothing is recorded either, the unavailability error
// surfaces.
func TestResolvePackageHistoryCapGone(t *testing.T) {
	l := newUpgradeChainLedger()
	delete(l.latest, "cap")
	delete(l.records, "cap")
	_, err := resolvePackageHistory(context.Background(), "p3", l)
	require.Error(t, err)
	require.ErrorIs(t, err, errObjectUnavailable)
	require.Contains(t, err.Error(), "get upgrade cap object")
}

// Scenario modeled on real chain data: the cap spent part of its life wrapped in
// another object. The unwrap tx (tw) produced cap v2 but does not list the cap in
// its json-rpc object changes, so the walk cannot read a prevVersion there and
// falls back to the recorded change history (which has no rows for wrap-period
// versions either — the newest recorded change below v2 is the creation).
func TestResolvePackageHistoryWrappedCap(t *testing.T) {
	const cap = "cap"
	l := &fakeLedger{
		latest: map[string]string{"p1": "t1", "p3": "t3", cap: "t3"},
		past: map[string]map[uint64]string{
			cap: {1: "t1", 2: "tw", 3: "t3"},
		},
		changes: map[string][]packageHistoryChange{
			"t1": {published("p1"), capCreated(cap)},
			"tw": {}, // the unwrap: cap absent from json-rpc object changes
			"t3": {published("p3"), capMutated(cap, 2)},
		},
		records: map[string][]sui.ObjectChangeRecord{
			cap: {
				{TxDigest: "t1", Type: "created", ObjectVersion: 1},
				{TxDigest: "t3", Type: "mutated", ObjectVersion: 3, PreviousVersion: u64p(2)},
			},
		},
	}
	history, err := resolvePackageHistory(context.Background(), "p3", l)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"p1", "p3"}, history)
}

// The grpc tx changes DO carry the cap for an unwrap, but without a previous
// version (input state does-not-exist, not a creation): same fallback.
func TestResolvePackageHistoryUnwrappedCapGrpc(t *testing.T) {
	const cap = "cap"
	l := &fakeLedger{
		latest: map[string]string{"p1": "t1", "p3": "t3", cap: "t3"},
		past: map[string]map[uint64]string{
			cap: {1: "t1", 2: "tw", 3: "t3"},
		},
		changes: map[string][]packageHistoryChange{
			"t1": {published("p1"), capCreated(cap)},
			"tw": {{objectID: cap, isUpgradeCap: true}}, // unwrapped: no prevVersion, not created
			"t3": {published("p3"), capMutated(cap, 2)},
		},
		records: map[string][]sui.ObjectChangeRecord{
			cap: {
				{TxDigest: "t1", Type: "created", ObjectVersion: 1},
				{TxDigest: "tw", Type: "unwrapped", ObjectVersion: 2}, // grpc history records the unwrap itself
				{TxDigest: "t3", Type: "mutated", ObjectVersion: 3, PreviousVersion: u64p(2)},
			},
		},
	}
	history, err := resolvePackageHistory(context.Background(), "p3", l)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"p1", "p3"}, history)
}
