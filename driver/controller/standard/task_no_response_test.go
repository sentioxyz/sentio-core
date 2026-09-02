package standard

import (
	"testing"

	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/processor/protos"
)

func TestValidateNoResponse(t *testing.T) {
	require.NoError(t, validateNoResponse(&protos.DBRequest{
		Op: &protos.DBRequest_Get{Get: &protos.DBRequest_DBGet{Entity: "E", Id: "1"}},
	}), "a plain read is fine")
	require.NoError(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_Upsert{Upsert: &protos.DBRequest_DBUpsert{}},
	}), "fire-and-forget upsert")
	require.NoError(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_Delete{Delete: &protos.DBRequest_DBDelete{}},
	}), "fire-and-forget delete")
	require.Error(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_Get{Get: &protos.DBRequest_DBGet{Entity: "E", Id: "1"}},
	}), "a read without a waiter would hang the processor")
	require.Error(t, validateNoResponse(&protos.DBRequest{
		NoResponse: true,
		Op:         &protos.DBRequest_List{List: &protos.DBRequest_DBList{Entity: "E"}},
	}), "a list without a waiter would hang the processor")
}
