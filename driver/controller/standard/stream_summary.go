package standard

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	"sentioxyz/sentio-core/processor/protos"
)

// A summary is written at Error level on every ERR200, so its length must not
// scale with the payload it describes. Three independent caps enforce that: how
// many entries are spelled out, how long any single value may be, and a final
// ceiling on the whole string as a backstop for shapes not anticipated here.
const (
	maxSummarySamples = 8
	maxSummaryValue   = 96 // fits a 66-char transaction hash whole, with room for the marker
	maxSummaryTotal   = 2048

	// Budget truncate reserves for its "...+N" marker so that a clipped value
	// respects the cap it was given instead of overshooting it by the marker.
	truncateMarkerBudget = 16
)

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
			// Processor errors routinely run to hundreds of characters (a wrapped
			// eth_call revert carries the whole request), so this one gets clipped too.
			fmt.Fprintf(&b, " error=%q", truncate(errMsg, maxSummaryValue))
		}
	case nil:
		b.WriteString(" <no value>")
	default:
		fmt.Fprintf(&b, " <unknown value %T>", v)
	}
	return truncate(b.String(), maxSummaryTotal)
}

func summarizeDBOp(req *protos.DBRequest) string {
	switch op := req.GetOp().(type) {
	case *protos.DBRequest_Get:
		return fmt.Sprintf("get entity=%s id=%s",
			truncate(op.Get.GetEntity(), maxSummaryValue), truncate(op.Get.GetId(), maxSummaryValue))
	case *protos.DBRequest_List:
		return fmt.Sprintf("list entity=%s filters=%d pageSize=%d cursor=%t",
			truncate(op.List.GetEntity(), maxSummaryValue), len(op.List.GetFilters()),
			op.List.GetPageSize(), op.List.GetCursor() != "")
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
// then name so the output is stable across runs. Only the top maxSummarySamples
// names are named — the number of distinct entity or metric names is as unbounded
// as the row count, so it needs the same treatment.
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
	elided := 0
	if len(names) > maxSummarySamples {
		elided = len(names) - maxSummarySamples
		names = names[:maxSummarySamples]
	}
	parts := make([]string, 0, len(names)+1)
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s:%d", truncate(name, maxSummaryValue), counts[name]))
	}
	if elided > 0 {
		parts = append(parts, fmt.Sprintf("...+%d more", elided))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// sample renders at most maxSummarySamples values, each clipped, noting how many
// were elided.
func sample(values []string) string {
	elided := 0
	if len(values) > maxSummarySamples {
		elided = len(values) - maxSummarySamples
		values = values[:maxSummarySamples]
	}
	clipped := make([]string, 0, len(values))
	for _, v := range values {
		clipped = append(clipped, truncate(v, maxSummaryValue))
	}
	if elided > 0 {
		return fmt.Sprintf("[%s,...+%d]", strings.Join(clipped, ","), elided)
	}
	return "[" + strings.Join(clipped, ",") + "]"
}

// truncate clips s to max runes — runes, not bytes, so a clipped value never ends
// in a broken UTF-8 sequence — and reports how many were dropped. The marker is
// paid for out of the budget, so the result never exceeds max.
func truncate(s string, max int) string {
	if len(s) <= max { // byte length is an upper bound on rune count
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	cut := max - truncateMarkerBudget
	if cut < 0 {
		cut = 0
	}
	return fmt.Sprintf("%s...+%d", string(runes[:cut]), len(runes)-cut)
}
