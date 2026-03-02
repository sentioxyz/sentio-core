package adaptor_eventlogs

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/cte"
)

func mustTimeRange(start, end time.Time, step time.Duration, tz *time.Location) *timerange.TimeRange {
	return &timerange.TimeRange{Start: start.UTC(), End: end.UTC(), Step: step, Timezone: tz}
}

func TestClickhouseHistogramTime_MappedSteps(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	tr := mustTimeRange(time.Unix(0, 0), time.Unix(1000, 0), time.Hour, time.UTC)
	agg := &aggregator{ctx: ctx, logger: logger, timeRange: tr}

	col := agg.ClickhouseHistogramTime(timeseries.SystemTimestamp)
	expected := "dateTrunc('hour', timestamp, 'UTC')"
	if col != expected {
		t.Fatalf("expected %s, got %s", expected, col)
	}
}

func TestClickhouseHistogramTime_ArbitraryStep(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	step := 45 * time.Second
	tz, _ := time.LoadLocation("Europe/Berlin")
	tr := mustTimeRange(time.Unix(0, 0), time.Unix(1000, 0), step, tz)
	agg := &aggregator{ctx: ctx, logger: logger, timeRange: tr}

	col := agg.ClickhouseHistogramTime(timeseries.SystemTimestamp)
	expected := "toDateTime64(formatDateTime(toStartOfInterval(timestamp, toIntervalSecond(45)), '%F %T', 'UTC'), 6, 'Europe/Berlin')"
	if col != expected {
		t.Fatalf("expected %s, got %s", expected, col)
	}
}

func TestNewAggregator_TotalAndUnique(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	tr := mustTimeRange(time.Unix(0, 0), time.Unix(1000, 0), time.Minute, time.UTC)

	// Total
	opTotal := &protoscommon.SegmentationQuery_Aggregation{Value: &protoscommon.SegmentationQuery_Aggregation_Total_{Total: &protoscommon.SegmentationQuery_Aggregation_Total{}}}
	a1, err := NewAggregator(ctx, logger, opTotal, tr, nil, QueryOption{})
	if err != nil {
		t.Fatalf("NewAggregator total err: %v", err)
	}
	if a1.AggField() != "count()" {
		t.Fatalf("total agg field expected count(), got %s", a1.AggField())
	}
	if a1.TimeField() == timeseries.SystemTimestamp {
		t.Fatalf("total should use histogram time field, got raw timestamp")
	}

	// Unique
	opUnique := &protoscommon.SegmentationQuery_Aggregation{Value: &protoscommon.SegmentationQuery_Aggregation_Unique_{Unique: &protoscommon.SegmentationQuery_Aggregation_Unique{}}}
	a2, err := NewAggregator(ctx, logger, opUnique, tr, nil, QueryOption{})
	if err != nil {
		t.Fatalf("NewAggregator unique err: %v", err)
	}
	if a2.AggField() == "" || a2.TimeField() == timeseries.SystemTimestamp {
		t.Fatalf("unique should set agg field and histogram time")
	}
}

func TestNewAggregator_SimpleSum(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	tr := mustTimeRange(time.Unix(0, 0), time.Unix(1000, 0), time.Minute, time.UTC)
	op := &protoscommon.SegmentationQuery_Aggregation{
		Value: &protoscommon.SegmentationQuery_Aggregation_AggregateProperties_{
			AggregateProperties: &protoscommon.SegmentationQuery_Aggregation_AggregateProperties{
				Type:         protoscommon.SegmentationQuery_Aggregation_AggregateProperties_SUM,
				PropertyName: "value",
			},
		},
	}
	a, err := NewAggregator(ctx, logger, op, tr, Breakdown{"label"}, QueryOption{})
	if err != nil {
		t.Fatalf("NewAggregator sum err: %v", err)
	}
	if a.AggField() != "sum(`value`)" {
		t.Fatalf("expected sum(`value`), got %s", a.AggField())
	}
	if a.TimeField() == timeseries.SystemTimestamp {
		t.Fatalf("sum should use histogram time field")
	}
}

func TestCumulativeAggregation_BuildsCTEAndUnion(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	tr := mustTimeRange(time.Unix(1000, 0), time.Unix(2000, 0), time.Minute, time.UTC)
	op := &protoscommon.SegmentationQuery_Aggregation{
		Value: &protoscommon.SegmentationQuery_Aggregation_AggregateProperties_{
			AggregateProperties: &protoscommon.SegmentationQuery_Aggregation_AggregateProperties{
				Type:         protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM,
				PropertyName: "value",
			},
		},
	}
	bkd := Breakdown{"chain"}
	aIntf, err := NewAggregator(ctx, logger, op, tr, bkd, QueryOption{})
	if err != nil {
		t.Fatalf("NewAggregator cumulative err: %v", err)
	}
	a := aIntf.(*aggregator)

	if len(a.CTE()) == 0 {
		t.Fatalf("expected CTE for cumulative earlierAggregationQuery")
	}
	u := a.Union()
	if len(u) != 1 || u[0] != "SELECT * FROM "+preAggTable {
		t.Fatalf("unexpected union: %v", u)
	}
	if a.Breakdown().String(false) != "" {
		t.Fatalf("cumulative should reset breakdown after agg field build")
	}
	if a.TimeField() == "" || a.AggField() == "" {
		t.Fatalf("timeField and aggField should be set")
	}
}

func TestCountUnique_LifetimeAndWindowNotImplemented(t *testing.T) {
	ctx, logger := log.FromContext(context.Background())
	tr := mustTimeRange(time.Unix(0, 0), time.Unix(10_000, 0), time.Hour, time.UTC)

	// lifetime (0 duration)
	opLifetime := &protoscommon.SegmentationQuery_Aggregation{
		Value: &protoscommon.SegmentationQuery_Aggregation_CountUnique_{
			CountUnique: &protoscommon.SegmentationQuery_Aggregation_CountUnique{Duration: &protoscommon.Duration{Value: 0, Unit: "day"}},
		},
	}
	a1Intf, err := NewAggregator(ctx, logger, opLifetime, tr, Breakdown{"label"}, QueryOption{})
	if err != nil {
		t.Fatalf("NewAggregator lifetime unique err: %v", err)
	}
	a1 := a1Intf.(*aggregator)
	if a1.Table() != "unique_new_user_per_day" {
		t.Fatalf("expected table unique_new_user_per_day, got %s", a1.Table())
	}
	if len(a1.CTE()) < 2 {
		t.Fatalf("lifetime unique should build two CTEs, got %d", len(a1.CTE()))
	}
}

func TestIsCumulativeAggregationOp_Simple(t *testing.T) {
	tests := []struct {
		name     string
		op       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_AggregationType
		expected bool
	}{
		{
			name:     "CUMULATIVE_SUM is cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM,
			expected: true,
		},
		{
			name:     "CUMULATIVE_DISTINCT_COUNT is cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_DISTINCT_COUNT,
			expected: true,
		},
		{
			name:     "CUMULATIVE_FIRST is cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_FIRST,
			expected: true,
		},
		{
			name:     "CUMULATIVE_LAST is cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_LAST,
			expected: true,
		},
		{
			name:     "CUMULATIVE_COUNT is cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_COUNT,
			expected: true,
		},
		{
			name:     "SUM is not cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_SUM,
			expected: false,
		},
		{
			name:     "AVG is not cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_AVG,
			expected: false,
		},
		{
			name:     "DISTINCT_COUNT is not cumulative",
			op:       protoscommon.SegmentationQuery_Aggregation_AggregateProperties_DISTINCT_COUNT,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCumulativeAggregationOp(tt.op)
			if result != tt.expected {
				t.Errorf("IsCumulativeAggregationOp(%v) = %v, expected %v", tt.op, result, tt.expected)
			}
		})
	}
}

func TestAggregator_Interface_Methods(t *testing.T) {
	// Create a simple aggregator struct to test interface methods
	agg := &aggregator{
		aggField:  "count(*)",
		table:     "test_table",
		timeField: "timestamp_field",
		breakdown: Breakdown{"field1", "field2"},
		distinct:  true,
		cte:       []cte.CTE{{Alias: "test", Query: "SELECT 1"}},
	}

	// Test AggField method
	if agg.AggField() != "count(*)" {
		t.Errorf("AggField() = %s, expected count(*)", agg.AggField())
	}

	// Test Table method
	if agg.Table() != "test_table" {
		t.Errorf("Table() = %s, expected test_table", agg.Table())
	}

	// Test TimeField method
	if agg.TimeField() != "timestamp_field" {
		t.Errorf("TimeField() = %s, expected timestamp_field", agg.TimeField())
	}

	// Test Breakdown method
	breakdown := agg.Breakdown()
	if len(breakdown) != 2 || breakdown[0] != "field1" || breakdown[1] != "field2" {
		t.Errorf("Breakdown() = %v, expected [field1 field2]", breakdown)
	}

	// Test Distinct method
	if !agg.Distinct() {
		t.Error("Distinct() should return true")
	}

	// Test CTE method
	ctes := agg.CTE()
	if len(ctes) != 1 || ctes[0].Alias != "test" || ctes[0].Query != "SELECT 1" {
		t.Errorf("CTE() = %v, expected single CTE with alias 'test'", ctes)
	}
}

func TestAggregator_Union_Methods(t *testing.T) {
	t.Run("non-cumulative aggregator returns no union", func(t *testing.T) {
		agg := &aggregator{cumulative: false}
		union := agg.Union()
		if union != nil {
			t.Errorf("Union() should return nil for non-cumulative aggregator, got %v", union)
		}
	})

	t.Run("cumulative aggregator returns union", func(t *testing.T) {
		agg := &aggregator{cumulative: true}
		union := agg.Union()
		expected := []string{"SELECT * FROM " + preAggTable}
		if len(union) != 1 || union[0] != expected[0] {
			t.Errorf("Union() = %v, expected %v", union, expected)
		}
	})
}

func TestAggregator_Join_Methods(t *testing.T) {
	t.Run("non-cumulative aggregator returns empty join", func(t *testing.T) {
		agg := &aggregator{cumulative: false}
		joinType, joinTable, onParameter := agg.Join()
		if joinType != "" || joinTable != "" || onParameter != "" {
			t.Errorf("Join() should return empty strings for non-cumulative aggregator, got (%s, %s, %s)", joinType, joinTable, onParameter)
		}
	})

	t.Run("cumulative aggregator with breakdown returns join", func(t *testing.T) {
		agg := &aggregator{
			cumulative: true,
			label:      Breakdown{"field1", "field2"},
			table:      "main_table",
		}
		joinType, joinTable, onParameter := agg.Join()

		if joinType != "LEFT" {
			t.Errorf("Join type = %s, expected LEFT", joinType)
		}
		if joinTable != preAggTable {
			t.Errorf("Join table = %s, expected %s", joinTable, preAggTable)
		}
		expectedOn := "(" + preAggTable + ".field1=main_table.field1) AND (" + preAggTable + ".field2=main_table.field2)"
		if onParameter != expectedOn {
			t.Errorf("Join on = %s, expected %s", onParameter, expectedOn)
		}
	})
}
