package standard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/processor/protos"
)

func TestSummarizeStreamResponseDBUpsert(t *testing.T) {
	resp := &protos.ProcessStreamResponseV3{
		ProcessId: 42,
		Value: &protos.ProcessStreamResponseV3_DbRequest{DbRequest: &protos.DBRequest{
			OpId: 7,
			Op: &protos.DBRequest_Upsert{Upsert: &protos.DBRequest_DBUpsert{
				Entity: []string{"Pool", "Pool", "User"},
				Id:     []string{"a", "b", "c"},
			}},
		}},
	}
	s := summarizeStreamResponse(resp)
	assert.Contains(t, s, "dbRequest op#7 upsert n=3")
	assert.Contains(t, s, "entities={Pool:2,User:1}")
	assert.Contains(t, s, "ids=[a,b,c]")
}

func TestSummarizeStreamResponseSamplesLongIDList(t *testing.T) {
	ids := make([]string, 0, maxSummarySamples+3)
	entities := make([]string, 0, maxSummarySamples+3)
	for i := 0; i < maxSummarySamples+3; i++ {
		ids = append(ids, "id")
		entities = append(entities, "Pool")
	}
	s := summarizeStreamResponse(&protos.ProcessStreamResponseV3{
		Value: &protos.ProcessStreamResponseV3_DbRequest{DbRequest: &protos.DBRequest{
			Op: &protos.DBRequest_Delete{Delete: &protos.DBRequest_DBDelete{Entity: entities, Id: ids}},
		}},
	})
	assert.Contains(t, s, "delete n=11")
	assert.Contains(t, s, ",...+3]")
	assert.Equal(t, maxSummarySamples, strings.Count(s, "id,"))
}

func TestSummarizeStreamResponseTimeseriesCarriesBlock(t *testing.T) {
	s := summarizeStreamResponse(&protos.ProcessStreamResponseV3{
		Value: &protos.ProcessStreamResponseV3_TsRequest{TsRequest: &protos.TSRequest{
			Data: []*protos.TimeseriesResult{{
				Type: protos.TimeseriesResult_GAUGE,
				Metadata: &protos.RecordMetaData{
					Name:            "vault_idle_fund",
					BlockNumber:     67699999,
					TransactionHash: "0x8d1979",
				},
			}},
		}},
	})
	assert.Contains(t, s, "tsRequest n=1 types={GAUGE:1}")
	assert.Contains(t, s, "names={vault_idle_fund:1}")
	assert.Contains(t, s, "blocks=[67699999/0x8d1979]")
}

func TestSummarizeStreamResponseResultAndEdgeCases(t *testing.T) {
	errMsg := "boom"
	s := summarizeStreamResponse(&protos.ProcessStreamResponseV3{
		Value: &protos.ProcessStreamResponseV3_Result{Result: &protos.ProcessResult{
			Exports: []*protos.ExportResult{{}},
			States:  &protos.StateResult{Error: &errMsg},
		}},
	})
	assert.Contains(t, s, "result gauges=0 counters=0 events=0 exports=1")
	assert.Contains(t, s, `error="boom"`)

	assert.Equal(t, "<nil>", summarizeStreamResponse(nil))
	assert.Contains(t, summarizeStreamResponse(&protos.ProcessStreamResponseV3{}), "<no value>")
}
