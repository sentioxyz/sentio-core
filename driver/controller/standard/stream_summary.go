package standard

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	"sentioxyz/sentio-core/processor/protos"
)

// maxSummarySamples caps how many ids / names a summary spells out. A single
// batched upsert can carry thousands of rows, and these summaries are written to
// the log on every ERR200, so the sample stays short and the rest is counted.
const maxSummarySamples = 8

// summarizeStreamResponse renders a compact description of a message the processor
// sent back on the binding stream. It is a diagnostic aid for the "unexpected
// ProcessID" path, where the message belongs to some *other* process and the useful
// question is "what was it and where did it come from" — so it reports the message
// shape (op kind, entity names, counts) plus block/transaction metadata when the
// payload carries any, never the full payload.
func summarizeStreamResponse(resp *protos.ProcessStreamResponseV3) string {
	if resp == nil {
		return "<nil>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "size=%dB", proto.Size(resp))
	switch v := resp.GetValue().(type) {
	case *protos.ProcessStreamResponseV3_Partitions:
		fmt.Fprintf(&b, " partitions n=%d", len(v.Partitions.GetPartitions()))
	case *protos.ProcessStreamResponseV3_DbRequest:
		fmt.Fprintf(&b, " dbRequest op#%d %s", v.DbRequest.GetOpId(), summarizeDBOp(v.DbRequest))
	case *protos.ProcessStreamResponseV3_TsRequest:
		fmt.Fprintf(&b, " tsRequest %s", summarizeTimeseries(v.TsRequest.GetData()))
	case *protos.ProcessStreamResponseV3_TplRequest:
		fmt.Fprintf(&b, " tplRequest n=%d remove=%t",
			len(v.TplRequest.GetTemplates()), v.TplRequest.GetRemove())
	case *protos.ProcessStreamResponseV3_Result:
		r := v.Result
		fmt.Fprintf(&b, " result gauges=%d counters=%d events=%d exports=%d ts=[%s]",
			len(r.GetGauges()), len(r.GetCounters()), len(r.GetEvents()), len(r.GetExports()),
			summarizeTimeseries(r.GetTimeseriesResult()))
		if errMsg := r.GetStates().GetError(); errMsg != "" {
			fmt.Fprintf(&b, " error=%q", errMsg)
		}
	case nil:
		b.WriteString(" <no value>")
	default:
		fmt.Fprintf(&b, " <unknown value %T>", v)
	}
	return b.String()
}

func summarizeDBOp(req *protos.DBRequest) string {
	switch op := req.GetOp().(type) {
	case *protos.DBRequest_Get:
		return fmt.Sprintf("get entity=%s id=%s", op.Get.GetEntity(), op.Get.GetId())
	case *protos.DBRequest_List:
		return fmt.Sprintf("list entity=%s filters=%d pageSize=%d cursor=%t",
			op.List.GetEntity(), len(op.List.GetFilters()), op.List.GetPageSize(), op.List.GetCursor() != "")
	case *protos.DBRequest_Upsert:
		return "upsert " + summarizeEntityRows(op.Upsert.GetEntity(), op.Upsert.GetId())
	case *protos.DBRequest_Update:
		return "update " + summarizeEntityRows(op.Update.GetEntity(), op.Update.GetId())
	case *protos.DBRequest_Delete:
		return "delete " + summarizeEntityRows(op.Delete.GetEntity(), op.Delete.GetId())
	case nil:
		return "<no op>"
	default:
		return fmt.Sprintf("<unknown op %T>", op)
	}
}

// summarizeEntityRows describes the parallel entity/id arrays the batched db ops
// carry: how many rows, which entities (with per-entity counts), and a sample of ids.
func summarizeEntityRows(entities, ids []string) string {
	return fmt.Sprintf("n=%d entities=%s ids=%s", len(ids), countByName(entities), sample(ids))
}

func summarizeTimeseries(data []*protos.TimeseriesResult) string {
	types := make([]string, 0, len(data))
	names := make([]string, 0, len(data))
	blocks := make([]string, 0, len(data))
	for _, d := range data {
		types = append(types, d.GetType().String())
		names = append(names, d.GetMetadata().GetName())
		blocks = append(blocks, fmt.Sprintf("%d/%s", d.GetMetadata().GetBlockNumber(),
			d.GetMetadata().GetTransactionHash()))
	}
	return fmt.Sprintf("n=%d types=%s names=%s blocks=%s",
		len(data), countByName(types), countByName(names), sample(blocks))
}

// countByName collapses a list into "name:count" pairs, sorted by descending count
// then name so the output is stable across runs.
func countByName(values []string) string {
	counts := make(map[string]int, len(values))
	for _, v := range values {
		counts[v]++
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s:%d", name, counts[name]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// sample renders at most maxSummarySamples values, noting how many were elided.
func sample(values []string) string {
	if len(values) <= maxSummarySamples {
		return "[" + strings.Join(values, ",") + "]"
	}
	return fmt.Sprintf("[%s,...+%d]",
		strings.Join(values[:maxSummarySamples], ","), len(values)-maxSummarySamples)
}
