package standard

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestSummarizeStreamResponseClipsOversizedError(t *testing.T) {
	errMsg := strings.Repeat("z", 5000)
	s := summarizeStreamResponse(&protos.ProcessStreamResponseV3{
		Value: &protos.ProcessStreamResponseV3_Result{Result: &protos.ProcessResult{
			States: &protos.StateResult{Error: &errMsg},
		}},
	})
	assert.Contains(t, s, "...+4920")
	assert.NotContains(t, s, strings.Repeat("z", maxSummaryValue+1))
}

// The number of *distinct* entity names is as unbounded as the row count, so it
// must be capped too — a batch touching hundreds of entities cannot be allowed to
// name every one of them.
func TestSummarizeStreamResponseCapsDistinctNames(t *testing.T) {
	var entities, ids []string
	for i := 0; i < 300; i++ {
		entities = append(entities, fmt.Sprintf("Entity%03d", i))
		ids = append(ids, fmt.Sprintf("id%03d", i))
	}
	s := summarizeStreamResponse(&protos.ProcessStreamResponseV3{
		Value: &protos.ProcessStreamResponseV3_DbRequest{DbRequest: &protos.DBRequest{
			Op: &protos.DBRequest_Upsert{Upsert: &protos.DBRequest_DBUpsert{Entity: entities, Id: ids}},
		}},
	})
	assert.Contains(t, s, "n=300")
	assert.Contains(t, s, "...+292 more")
	assert.Equal(t, maxSummarySamples, strings.Count(s, "Entity"))
}

// Whatever the payload, the log line stays bounded. This is the backstop for
// shapes the per-field caps above did not anticipate.
func TestSummarizeStreamResponseIsBoundedOverall(t *testing.T) {
	long := strings.Repeat("x", 4000)
	var data []*protos.TimeseriesResult
	for i := 0; i < 500; i++ {
		data = append(data, &protos.TimeseriesResult{Metadata: &protos.RecordMetaData{
			Name:            fmt.Sprintf("%s-%d", long, i),
			TransactionHash: long,
		}})
	}
	for _, resp := range []*protos.ProcessStreamResponseV3{
		{Value: &protos.ProcessStreamResponseV3_TsRequest{TsRequest: &protos.TSRequest{Data: data}}},
		{Value: &protos.ProcessStreamResponseV3_DbRequest{DbRequest: &protos.DBRequest{
			Op: &protos.DBRequest_Get{Get: &protos.DBRequest_DBGet{Entity: long, Id: long}},
		}}},
	} {
		s := summarizeStreamResponse(resp)
		assert.LessOrEqual(t, utf8.RuneCountInString(s), maxSummaryTotal)
	}
}

func TestTruncateRespectsBudgetAndRuneBoundaries(t *testing.T) {
	assert.Equal(t, "short", truncate("short", 10))
	assert.Equal(t, "exactlyten", truncate("exactlyten", 10))

	// Multi-byte input: the cut must land on a rune boundary, and the result must
	// still fit the budget it was given.
	s := truncate(strings.Repeat("中", 100), 20)
	assert.True(t, utf8.ValidString(s))
	assert.LessOrEqual(t, utf8.RuneCountInString(s), 20)
	assert.Equal(t, strings.Repeat("中", 4)+"...+96", s)
}
