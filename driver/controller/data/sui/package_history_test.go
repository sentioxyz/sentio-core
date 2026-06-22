package sui

import (
	"testing"

	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"github.com/stretchr/testify/require"
)

// fakeGrpcLedger backs resolveGrpcPackageHistory's object/transaction fetchers
// with in-memory data so the upgrade-cap walk can be tested without any RPC.
type fakeGrpcLedger struct {
	// objects keyed by objectID; version nil ("latest") and explicit versions both
	// resolve here. We model a single linear chain, so latest == highest version.
	latestObject map[string]*rpcv2.Object
	versioned    map[string]map[uint64]*rpcv2.Object
	txs          map[string]*rpcv2.ExecutedTransaction
}

func (f *fakeGrpcLedger) getObject(objectID string, version *uint64) (*rpcv2.Object, error) {
	if version == nil {
		if o, ok := f.latestObject[objectID]; ok {
			return o, nil
		}
		return nil, errors.Errorf("object %s (latest) not found", objectID)
	}
	if o, ok := f.versioned[objectID][*version]; ok {
		return o, nil
	}
	return nil, errors.Errorf("object %s@%d not found", objectID, *version)
}

func (f *fakeGrpcLedger) getTransaction(digest string) (*rpcv2.ExecutedTransaction, error) {
	if tx, ok := f.txs[digest]; ok {
		return tx, nil
	}
	return nil, errors.Errorf("tx %s not found", digest)
}

func sp(s string) *string  { return &s }
func u64p(v uint64) *uint64 { return &v }

func obj(objectID string, version uint64, prevTx string) *rpcv2.Object {
	return &rpcv2.Object{
		ObjectId:            sp(objectID),
		Version:             u64p(version),
		PreviousTransaction: sp(prevTx),
	}
}

// fullUpgradeCapType mirrors what grpc returns: the full-length 0x0…02 address,
// not the abbreviated 0x2 — exercising move.Type address normalization in the walk.
const fullUpgradeCapType = "0x0000000000000000000000000000000000000000000000000000000000000002::package::UpgradeCap"

// publishedChange is a package publish. Real fullnodes report it as a "package"
// typed OBJECT_WRITE (not PACKAGE_WRITE), so the fake mirrors that.
func publishedChange(pkgID string) *rpcv2.ChangedObject {
	return &rpcv2.ChangedObject{
		ObjectId:    sp(pkgID),
		ObjectType:  sp(suiPackageObjectType),
		InputState:  rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST.Enum(),
		OutputState: rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_OBJECT_WRITE.Enum(),
		IdOperation: rpcv2.ChangedObject_CREATED.Enum(),
	}
}

// capCreated is the UpgradeCap created alongside the original publish (no prior version).
func capCreated(capID string) *rpcv2.ChangedObject {
	return &rpcv2.ChangedObject{
		ObjectId:    sp(capID),
		ObjectType:  sp(fullUpgradeCapType),
		InputState:  rpcv2.ChangedObject_INPUT_OBJECT_STATE_DOES_NOT_EXIST.Enum(),
		OutputState: rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_OBJECT_WRITE.Enum(),
		IdOperation: rpcv2.ChangedObject_CREATED.Enum(),
	}
}

// capMutated is the UpgradeCap mutated by an upgrade, carrying its prior version.
func capMutated(capID string, inputVersion uint64) *rpcv2.ChangedObject {
	return &rpcv2.ChangedObject{
		ObjectId:     sp(capID),
		ObjectType:   sp(fullUpgradeCapType),
		InputState:   rpcv2.ChangedObject_INPUT_OBJECT_STATE_EXISTS.Enum(),
		InputVersion: u64p(inputVersion),
		OutputState:  rpcv2.ChangedObject_OUTPUT_OBJECT_STATE_OBJECT_WRITE.Enum(),
		IdOperation:  rpcv2.ChangedObject_NONE.Enum(),
	}
}

func execTx(digest string, changes ...*rpcv2.ChangedObject) *rpcv2.ExecutedTransaction {
	return &rpcv2.ExecutedTransaction{
		Digest:  sp(digest),
		Effects: &rpcv2.TransactionEffects{ChangedObjects: changes},
	}
}

// Scenario: package published as p1 (tx t1, cap created v1), upgraded to p2 (tx
// t2, cap -> v2) and p3 (tx t3, cap -> v3, latest). Querying any version must
// return the full history walked back through the UpgradeCap's input_version.
func newUpgradeChainLedger() *fakeGrpcLedger {
	const cap = "cap"
	return &fakeGrpcLedger{
		latestObject: map[string]*rpcv2.Object{
			"p1":  obj("p1", 1, "t1"),
			"p2":  obj("p2", 1, "t2"),
			"p3":  obj("p3", 1, "t3"),
			"cap": obj(cap, 3, "t3"), // latest cap version produced by t3
		},
		versioned: map[string]map[uint64]*rpcv2.Object{
			cap: {
				1: obj(cap, 1, "t1"),
				2: obj(cap, 2, "t2"),
				3: obj(cap, 3, "t3"),
			},
		},
		txs: map[string]*rpcv2.ExecutedTransaction{
			"t1": execTx("t1", publishedChange("p1"), capCreated(cap)),
			"t2": execTx("t2", publishedChange("p2"), capMutated(cap, 1)),
			"t3": execTx("t3", publishedChange("p3"), capMutated(cap, 2)),
		},
	}
}

func TestResolveGrpcPackageHistory(t *testing.T) {
	for _, pkgID := range []string{"p1", "p2", "p3"} {
		t.Run("query_"+pkgID, func(t *testing.T) {
			l := newUpgradeChainLedger()
			history, err := resolveGrpcPackageHistory(pkgID, l.getObject, l.getTransaction)
			require.NoError(t, err)
			require.ElementsMatch(t, []string{"p1", "p2", "p3"}, history)
			require.Contains(t, history, pkgID)
		})
	}
}

// A package with no UpgradeCap (e.g. published with an immediately-burned cap, or
// a system package) resolves to just itself.
func TestResolveGrpcPackageHistoryNoUpgradeCap(t *testing.T) {
	l := &fakeGrpcLedger{
		latestObject: map[string]*rpcv2.Object{"p1": obj("p1", 1, "t1")},
		txs:          map[string]*rpcv2.ExecutedTransaction{"t1": execTx("t1", publishedChange("p1"))},
	}
	history, err := resolveGrpcPackageHistory("p1", l.getObject, l.getTransaction)
	require.NoError(t, err)
	require.Equal(t, []string{"p1"}, history)
}
