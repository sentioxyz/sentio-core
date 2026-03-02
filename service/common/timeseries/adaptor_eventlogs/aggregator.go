package adaptor_eventlogs

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/log"
	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/cte"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"
	"sentioxyz/sentio-core/service/common/timeseries/util"

	"github.com/jinzhu/copier"
	"github.com/samber/lo"
)

const (
	dau      = time.Hour * 24
	wau      = time.Hour * 24 * 7
	mau      = time.Hour * 24 * 30
	lifetime = time.Duration(0)
)

type Aggregator interface {
	CTE() []cte.CTE
	Union() []string
	Join() (joinType, joinTable, onParameter string)
	Distinct() bool
	AggField() string
	Breakdown() Breakdown
	Table() string
	TimeField() string
	TimePostcondition() bool
	Label() Breakdown
	Cumulative() bool
}

func IsCumulativeAggregationOp(op protoscommon.SegmentationQuery_Aggregation_AggregateProperties_AggregationType) bool {
	switch op {
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM,
		protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_DISTINCT_COUNT,
		protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_FIRST,
		protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_LAST,
		protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_COUNT:
		return true
	}
	return false
}

type aggregator struct {
	ctx    context.Context
	logger *log.SentioLogger
	op     *protoscommon.SegmentationQuery_Aggregation
	option QueryOption

	cumulative        bool
	distinct          bool
	timePostcondition bool
	timeRange         *timerange.TimeRange
	breakdown         Breakdown
	label             Breakdown
	cte               []cte.CTE
	table             string
	aggField          string
	timeField         string
}

func NewAggregator(ctx context.Context, logger *log.SentioLogger,
	aggregation *protoscommon.SegmentationQuery_Aggregation,
	timeRange *timerange.TimeRange, breakdown Breakdown, option QueryOption) (Aggregator, error) {
	agg := &aggregator{
		ctx:       ctx,
		logger:    logger,
		op:        aggregation,
		option:    option,
		timeRange: timeRange,
		table:     mainTable,
		timeField: timeseries.SystemTimestamp,
	}
	_ = copier.CopyWithOption(&agg.breakdown, &breakdown, copier.Option{
		IgnoreEmpty: true,
		DeepCopy:    true,
	})
	_ = copier.CopyWithOption(&agg.label, &breakdown, copier.Option{
		IgnoreEmpty: true,
		DeepCopy:    true,
	})

	switch agg.op.Value.(type) {
	case *protoscommon.SegmentationQuery_Aggregation_Total_:
		agg.aggField = "count()"
		agg.timeField = agg.ClickhouseHistogramTime(timeseries.SystemTimestamp)
		agg.breakdown = append(agg.breakdown, timeseries.SystemTimestamp)
	case *protoscommon.SegmentationQuery_Aggregation_Unique_:
		agg.aggField = "uniqExact(cityHash64(" +
			timeseries.SystemFieldPrefix + "chain," +
			timeseries.SystemFieldPrefix + "block_number," +
			timeseries.SystemFieldPrefix + "block_hash," +
			timeseries.SystemFieldPrefix + "transaction_hash," +
			timeseries.SystemFieldPrefix + "transaction_index," +
			timeseries.SystemFieldPrefix + "log_index))"
		agg.timeField = agg.ClickhouseHistogramTime(timeseries.SystemTimestamp)
		agg.breakdown = append(agg.breakdown, timeseries.SystemTimestamp)
	case *protoscommon.SegmentationQuery_Aggregation_CountUnique_:
		if err := agg.uniqueUser(); err != nil {
			return nil, err
		}
	case *protoscommon.SegmentationQuery_Aggregation_AggregateProperties_:
		if err := agg.opFunc(); err != nil {
			return nil, err
		}
	}
	return agg, nil
}

func (a *aggregator) lifetimeUniqueUser() error {
	var (
		breakdown = a.breakdown.String(true)
		partition = lo.If(breakdown != "", "partition by "+a.breakdown.String(false)).Else("")
	)
	a.cte = append(a.cte, cte.CTE{
		Alias: "user_first_time",
		Query: "SELECT " +
			"min(" + a.ClickhouseHistogramTime(timeseries.SystemTimestamp) + ") as " + timeseries.SystemTimestamp + "," +
			timeseries.SystemUserID + breakdown + " FROM " + mainTable + " GROUP BY " +
			timeseries.SystemUserID + breakdown,
	})
	a.cte = append(a.cte, cte.CTE{
		Alias: "unique_new_user_per_day",
		Query: "SELECT DISTINCT count() as count," + timeseries.SystemTimestamp + breakdown +
			" FROM user_first_time GROUP BY " + timeseries.SystemTimestamp + breakdown,
	})
	a.table = "unique_new_user_per_day"
	a.aggField = "sum(count) OVER (" + partition + " order by " + timeseries.SystemTimestamp + " asc rows between unbounded preceding and current row)"
	a.timeField = timeseries.SystemTimestamp
	a.breakdown = Breakdown{}
	a.timePostcondition = true
	return nil
}

func (a *aggregator) uniqueUserWithWindowFunction(duration time.Duration) error {
	var (
		breakdown         = a.breakdown.String(true)
		partition         = lo.If(breakdown != "", "partition by "+a.breakdown.String(false)).Else("")
		rollupParams, err = NewRollupParams(duration, "d")
	)
	if err != nil {
		return err
	}

	var (
		rollupAggregate = "uniqCombined(" + timeseries.SystemUserID + ") OVER (" + partition +
			" ORDER BY toDate(" + timeseries.SystemTimestamp + ",'" + a.timeRange.Timezone.String() + "') DESC RANGE BETWEEN " +
			strconv.FormatInt(int64(rollupParams.ToDate()), 10) + " PRECEDING AND CURRENT ROW)"
	)

	a.cte = append(a.cte, cte.CTE{
		Alias: "rollup_table",
		Query: "SELECT DISTINCT " + timeseries.SystemTimestamp + ", " + rollupAggregate + " AS rollup_aggr " + breakdown + " FROM " + mainTable,
	})
	a.cte = append(a.cte, cte.CTE{
		Alias: "rollup_after_filter",
		Query: "SELECT DISTINCT " + a.histogram(time.Hour*24, timeseries.SystemTimestamp, a.timeRange.Timezone.String()) + " AS _" + timeseries.SystemTimestamp + "," +
			"rollup_aggr" + breakdown + " FROM rollup_table WHERE _" + timeseries.SystemTimestamp + " = " + a.ClickhouseHistogramTime(timeseries.SystemTimestamp),
	})
	a.table = "rollup_after_filter"
	a.aggField = "rollup_aggr"
	a.timeField = "_" + timeseries.SystemTimestamp
	a.breakdown = Breakdown{}
	a.timePostcondition = true
	return nil
}

func (a *aggregator) uniqueUser() error {
	uniqueUserOp := a.op.GetCountUnique()

	switch d := timerange.ParseTimeDuration(uniqueUserOp.GetDuration()); d {
	case lifetime:
		return a.lifetimeUniqueUser()
	default:
		switch {
		case a.timeRange.Step >= d:
			var supported = map[time.Duration]struct{}{
				dau: {},
				wau: {},
				mau: {},
			}
			if _, ok := supported[d]; !ok {
				return fmt.Errorf("unsupported duration: %v", d)
			}
			a.aggField = "uniqExact(" + timeseries.SystemUserID + ")"
			a.timeField = a.ClickhouseHistogramTime(timeseries.SystemTimestamp)
			a.breakdown = append(a.breakdown, timeseries.SystemTimestamp)
		default:
			return a.uniqueUserWithWindowFunction(d)
		}
	}
	return nil
}

func (a *aggregator) histogram(d time.Duration, f, tz string) string {
	return util.HistogramFunction(d, f, tz)
}

func (a *aggregator) ClickhouseHistogramTime(timeField string) string {
	return a.histogram(a.timeRange.Step, timeField, a.timeRange.Timezone.String())
}

func (a *aggregator) CTE() []cte.CTE {
	return a.cte
}

func (a *aggregator) Union() []string {
	if a.cumulative {
		return []string{
			"SELECT * FROM " + preAggTable,
		}
	}
	return nil
}

func (a *aggregator) Join() (joinType, joinTable, onParameter string) {
	if a.cumulative {
		var labels []string
		for _, b := range a.label {
			labels = append(labels, "("+preAggTable+"."+b+"="+a.table+"."+b+")")
		}
		if len(labels) == 0 {
			labels = append(labels, "1=1")
		}
		return "LEFT", preAggTable, strings.Join(labels, " AND ")
	}
	return
}

func (a *aggregator) Distinct() bool {
	return a.distinct
}

func (a *aggregator) AggField() string {
	return a.aggField
}

func (a *aggregator) Breakdown() Breakdown {
	return a.breakdown
}

func (a *aggregator) Label() Breakdown {
	return a.label
}

func (a *aggregator) Table() string {
	return a.table
}

func (a *aggregator) TimeField() string {
	return a.timeField
}

func (a *aggregator) TimePostcondition() bool {
	return a.timePostcondition
}

func (a *aggregator) opFunc() error {
	op := a.op.GetAggregateProperties()
	switch {
	case IsCumulativeAggregationOp(op.GetType()):
		return a.cumulativeOpFunc()
	default:
		return a.normalOpFunc()
	}
}

func (a *aggregator) normalOpFunc() error {
	a.timeField = a.ClickhouseHistogramTime(timeseries.SystemTimestamp)
	a.breakdown = append(a.breakdown, timeseries.SystemTimestamp)
	switch a.op.GetAggregateProperties().GetType() {
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_SUM:
		a.aggField = "sum(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_AVG:
		a.aggField = "avg(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_MEDIAN:
		a.aggField = "median(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_MIN:
		a.aggField = "min(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_MAX:
		a.aggField = "max(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_DISTINCT_COUNT:
		a.aggField = "uniqExact(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_FIRST:
		a.aggField = "argMin(`" + a.op.GetAggregateProperties().GetPropertyName() + "`," +
			"tuple(" +
			timeseries.SystemFieldPrefix + "block_number," +
			timeseries.SystemFieldPrefix + "transaction_index," +
			timeseries.SystemFieldPrefix + "log_index)" +
			")"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_LAST:
		a.aggField = "argMax(`" + a.op.GetAggregateProperties().GetPropertyName() + "`," +
			"tuple(" +
			timeseries.SystemFieldPrefix + "block_number," +
			timeseries.SystemFieldPrefix + "transaction_index," +
			timeseries.SystemFieldPrefix + "log_index)" +
			")"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_25TH:
		a.aggField = "quantile(0.25)(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_75TH:
		a.aggField = "quantile(0.75)(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_90TH:
		a.aggField = "quantile(0.90)(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_95TH:
		a.aggField = "quantile(0.95)(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_99TH:
		a.aggField = "quantile(0.99)(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	default:
		return fmt.Errorf("unknown aggregation type: %v", a.op.GetAggregateProperties().GetType())
	}
	return nil
}

func (a *aggregator) earlierAggregationQuery() (string, error) {
	var (
		tpl = "SELECT {start_time} as {start_time_alias}, " +
			"{agg_field} as {agg_field_alias} " +
			"{breakdown} FROM {table} {group_breakdown}"
		countTpl = "SELECT count() FROM ({tpl})"
		aggField string
		preLabel Breakdown
	)
	_ = copier.CopyWithOption(&preLabel, &a.label, copier.Option{
		IgnoreEmpty: true,
		DeepCopy:    true,
	})
	preLabel = append(preLabel, timeseries.SystemTimestamp)
	switch a.op.GetAggregateProperties().GetType() {
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM:
		aggField = "sum(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_DISTINCT_COUNT:
		aggField = "uniqExact(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_FIRST:
		aggField = "argMin(`" + a.op.GetAggregateProperties().GetPropertyName() + "`," +
			"tuple(" +
			timeseries.SystemFieldPrefix + "block_number," +
			timeseries.SystemFieldPrefix + "transaction_index," +
			timeseries.SystemFieldPrefix + "log_index)" +
			")"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_LAST:
		aggField = "argMax(`" + a.op.GetAggregateProperties().GetPropertyName() + "`," +
			"tuple(" +
			timeseries.SystemFieldPrefix + "block_number," +
			timeseries.SystemFieldPrefix + "transaction_index," +
			timeseries.SystemFieldPrefix + "log_index)" +
			")"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_COUNT:
		aggField = "count(`" + a.op.GetAggregateProperties().GetPropertyName() + "`)"
	}
	sql := builder.FormatSQLTemplate(tpl, map[string]any{
		"start_time":       a.ClickhouseHistogramTime(fmt.Sprintf("toDateTime64('%s', 6, '%s')", a.timeRange.Start.UTC().Format("2006-01-02 15:04:05"), a.timeRange.Timezone.String())),
		"timezone":         a.timeRange.Timezone.String(),
		"start_time_alias": matrix.TimeFieldName,
		"agg_field":        aggField,
		"agg_field_alias":  matrix.AggFieldName,
		"breakdown":        a.breakdown.String(true),
		"table":            beforeTimeRangeTable,
		"group_breakdown":  lo.If(preLabel.String(false) != "", "GROUP BY "+preLabel.String(false)).Else(""),
	})

	if a.option.CumulativePreCheck && a.option.Conn != nil {
		countSQL := builder.FormatSQLTemplate(countTpl, map[string]any{
			"tpl": sql,
		})
		var label uint64
		if err := a.option.Conn.QueryRow(a.ctx, countSQL).Scan(&label); err != nil {
			return "", fmt.Errorf("aggergate pre check failed: %w", err)
		}
		if a.option.CumulativeLabelLimit != 0 && label > uint64(a.option.CumulativeLabelLimit) {
			return "", fmt.Errorf("aggergate pre check failed: label count %d exceeds limit %d", label, a.option.CumulativeLabelLimit)
		}
	}
	return sql, nil
}

func (a *aggregator) cumulativeOpFunc() error {
	a.distinct = true
	a.cumulative = true
	preQuery, err := a.earlierAggregationQuery()
	if err != nil {
		return err
	}
	a.cte = append(a.cte, cte.CTE{
		Alias: preAggTable,
		Query: preQuery,
	})
	a.timeField = a.ClickhouseHistogramTime(timeseries.SystemTimestamp)
	var partition = lo.If(a.breakdown.String(false) != "", "partition by "+a.breakdown.String(false)).Else("")
	switch a.op.GetAggregateProperties().GetType() {
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM:
		a.aggField = preAggTable + "." + matrix.AggFieldName + "+sum(`" + a.op.GetAggregateProperties().GetPropertyName() + "`) OVER (" + partition + " order by " + timeseries.SystemTimestamp + " asc)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_DISTINCT_COUNT:
		a.aggField = preAggTable + "." + matrix.AggFieldName + "+uniqExact(`" + a.op.GetAggregateProperties().GetPropertyName() + "`) OVER (" + partition + " order by " + timeseries.SystemTimestamp + " asc)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_FIRST:
		a.aggField = preAggTable + "." + matrix.AggFieldName + "+argMin(`" + a.op.GetAggregateProperties().GetPropertyName() + "`," +
			"tuple(" +
			timeseries.SystemFieldPrefix + "block_number," +
			timeseries.SystemFieldPrefix + "transaction_index," +
			timeseries.SystemFieldPrefix + "log_index)" +
			") OVER (" + partition + " order by " + timeseries.SystemTimestamp + " asc)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_LAST:
		a.aggField = preAggTable + "." + matrix.AggFieldName + "+argMax(`" + a.op.GetAggregateProperties().GetPropertyName() + "`," +
			"tuple(" +
			timeseries.SystemFieldPrefix + "block_number," +
			timeseries.SystemFieldPrefix + "transaction_index," +
			timeseries.SystemFieldPrefix + "log_index)" +
			") OVER (" + partition + " order by " + timeseries.SystemTimestamp + " asc)"
	case protoscommon.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_COUNT:
		a.aggField = preAggTable + "." + matrix.AggFieldName + "+count(`" + a.op.GetAggregateProperties().GetPropertyName() + "`) OVER (" + partition + " order by " + timeseries.SystemTimestamp + " asc)"
	default:
		return fmt.Errorf("unknown cumulative aggregation type: %v", a.op.GetAggregateProperties().GetType())
	}
	a.breakdown = Breakdown{}
	return nil
}

func (a *aggregator) Cumulative() bool {
	return a.cumulative
}
