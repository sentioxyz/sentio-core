package sui

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// fakeLedger is a transport-neutral packageHistoryLedger backed by in-memory data,
// so resolvePackageHistory's UpgradeCap version-chain walk can be tested without RPC.
type fakeLedger struct {
	// latestPrevTx[objectID] = tx that produced the object's latest version.
	latestPrevTx map[string]string
	// pastPrevTx[objectID][version] = tx that produced that specific version.
	pastPrevTx map[string]map[uint64]string
	// changes[txDigest] = the object changes of that tx.
	changes map[string][]packageHistoryChange
}

func (f *fakeLedger) objectPrevTx(_ context.Context, objectID string, version *uint64) (string, error) {
	if version == nil {
		if tx, ok := f.latestPrevTx[objectID]; ok {
			return tx, nil
		}
		return "", errors.Errorf("object %s (latest) not found", objectID)
	}
	if tx, ok := f.pastPrevTx[objectID][*version]; ok {
		return tx, nil
	}
	return "", errors.Errorf("object %s@%d not found", objectID, *version)
}

func (f *fakeLedger) txChanges(_ context.Context, digest string) ([]packageHistoryChange, error) {
	if cs, ok := f.changes[digest]; ok {
		return cs, nil
	}
	return nil, errors.Errorf("tx %s not found", digest)
}

func u64p(v uint64) *uint64 { return &v }

func published(pkgID string) packageHistoryChange {
	return packageHistoryChange{objectID: pkgID, isPublished: true}
}

func capChange(capID string, prevVersion *uint64) packageHistoryChange {
	return packageHistoryChange{objectID: capID, isUpgradeCap: true, prevVersion: prevVersion}
}

// Scenario: package published as p1 (tx t1, cap created v1), upgraded to p2 (tx
// t2, cap -> v2) and p3 (tx t3, cap -> v3, latest). Querying any version must
// return the full history walked back through the UpgradeCap's prevVersion.
func newUpgradeChainLedger() *fakeLedger {
	const cap = "cap"
	return &fakeLedger{
		latestPrevTx: map[string]string{
			"p1": "t1", "p2": "t2", "p3": "t3",
			cap: "t3", // cap's latest version was produced by t3
		},
		pastPrevTx: map[string]map[uint64]string{
			cap: {1: "t1", 2: "t2", 3: "t3"},
		},
		changes: map[string][]packageHistoryChange{
			"t1": {published("p1"), capChange(cap, nil)},     // cap created here
			"t2": {published("p2"), capChange(cap, u64p(1))}, // prev version 1 -> t1
			"t3": {published("p3"), capChange(cap, u64p(2))}, // prev version 2 -> t2
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
		latestPrevTx: map[string]string{"p1": "t1"},
		changes:      map[string][]packageHistoryChange{"t1": {published("p1")}},
	}
	history, err := resolvePackageHistory(context.Background(), "p1", l)
	require.NoError(t, err)
	require.Equal(t, []string{"p1"}, history)
}
