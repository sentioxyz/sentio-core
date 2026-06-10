package types

import (
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/goccy/go-json"
	"github.com/kinbiko/jsonassert"
)

func TestObjectChangeTypeClassification(t *testing.T) {
	deleted := []ObjectChangeType{ObjectChangeTypeDeleted, ObjectChangeTypeWrapped, ObjectChangeTypeUnwrappedThenDeleted}
	created := []ObjectChangeType{ObjectChangeTypeCreated, ObjectChangeTypePublished, ObjectChangeTypeUnwrapped}
	for _, c := range deleted {
		assert.Truef(t, c.IsDeleted(), "%s should be deleted", c)
		assert.Falsef(t, c.IsCreated(), "%s should not be created", c)
	}
	for _, c := range created {
		assert.Truef(t, c.IsCreated(), "%s should be created", c)
		assert.Falsef(t, c.IsDeleted(), "%s should not be deleted", c)
	}
	assert.False(t, ObjectChangeType(ObjectChangeTypeMutated).IsDeleted())
	assert.False(t, ObjectChangeType(ObjectChangeTypeMutated).IsCreated())
}

func TestObjectChangeGetObjectID(t *testing.T) {
	pkg := StrToObjectIDMust("0x2")
	obj := StrToObjectIDMust("0x5")
	// packageId takes precedence over objectId
	assert.Equal(t, pkg.String(), ObjectChange{PackageID: &pkg, ObjectID: &obj}.GetObjectID())
	assert.Equal(t, obj.String(), ObjectChange{ObjectID: &obj}.GetObjectID())
	assert.Equal(t, "", ObjectChange{}.GetObjectID())
}

func TestTransactionResponseV1JSON(t *testing.T) {
	b, _ := os.ReadFile(transactionBundleFile)
	var rawTxs []json.RawMessage
	err := json.Unmarshal(b, &rawTxs)
	if err != nil {
		t.Fatal(err)
	}

	ja := jsonassert.New(t)
	for _, rawTx := range rawTxs {
		var tx TransactionResponseV1
		err := json.Unmarshal(rawTx, &tx)
		if err != nil {
			t.Fatal(errors.Wrap(err, string(rawTx)))
		}

		b, err := json.Marshal(tx)
		if err != nil {
			t.Fatal(err)
		}
		ja.Assertf(string(b), string(rawTx))
	}
}
