package adaptor_eventlogs

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/protojson"
	builder "sentioxyz/sentio-core/common/sqlbuilder"
	anyutil "sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/driver/timeseries/clickhouse"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/cte"
	"sentioxyz/sentio-core/service/common/timeseries/matrix"
	processormodels "sentioxyz/sentio-core/service/processor/models"

	"github.com/samber/lo"
)

type CohortAdaptor interface {
	Add(op protoscommon.JoinOperator, groups ...*protoscommon.CohortsGroup) CohortAdaptor
	FetchUserProperty() CohortAdaptor
	CountUser() CohortAdaptor
	Order(order ...string) CohortAdaptor
	Limit(limit int) CohortAdaptor
	Offset(offset int) CohortAdaptor
	Search(search string) CohortAdaptor
	Fork() CohortAdaptor

	Build() string
	Error() error
	Scan(ctx context.Context, scanFunc ScanFunc, sql string, args ...any) (matrix.Matrix, error)
}

type cohortAdaptor struct {
	Base

	cte               cte.CTEs
	fetchUserProperty bool
	countUser         bool
	limit             int
	offset            int
	search            string
	order             []string
}

func NewCohortAdaptor(ctx context.Context,
	store timeseries.Store, processor *processormodels.Processor) CohortAdaptor {
	ctx, logger := log.FromContext(ctx, "processor_id", processor.ID, "function", "CohortAdaptor")
	return &cohortAdaptor{
		Base: Base{
			ctx:       ctx,
			logger:    logger,
			store:     store,
			meta:      store.Meta().MetaByType(timeseries.MetaTypeEvent),
			processor: processor,
		},
		cte:               cte.CTEs{},
		fetchUserProperty: false,
	}
}

func (c *cohortAdaptor) Fork() CohortAdaptor {
	rhs := &cohortAdaptor{
		Base: Base{
			ctx:       c.ctx,
			logger:    c.logger,
			store:     c.store,
			meta:      c.meta,
			processor: c.processor,
			errors:    c.errors,
		},
		fetchUserProperty: c.fetchUserProperty,
		countUser:         c.countUser,
		limit:             c.limit,
		offset:            c.offset,
		search:            c.search,
		order:             c.order,
	}
	rhs.cte = append(rhs.cte, c.cte...)
	return rhs
}

func (c *cohortAdaptor) cteName(groupIdx, filterIdx int, ttype string) string {
	return fmt.Sprintf("%s_%d_%d", ttype, groupIdx, filterIdx)
}

func (c *cohortAdaptor) filterQueryHaving(logger *log.SentioLogger, aggregation *protoscommon.CohortsFilter_Aggregation) (having string) {
	if len(aggregation.GetValue()) == 0 {
		c.errors = append(c.errors, fmt.Errorf("aggregation value is nil"))
		logger.Errorf("aggregation value is nil")
		return
	}
	switch aggregation.GetOperator() {
	case protoscommon.CohortsFilter_Aggregation_EQ:
		having = "equals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) + ")"
	case protoscommon.CohortsFilter_Aggregation_NEQ:
		having = "notEquals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) + ")"
	case protoscommon.CohortsFilter_Aggregation_GT:
		having = "greater(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) + ")"
	case protoscommon.CohortsFilter_Aggregation_LT:
		having = "less(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) + ")"
	case protoscommon.CohortsFilter_Aggregation_GTE:
		having = "greaterOrEquals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) + ")"
	case protoscommon.CohortsFilter_Aggregation_LTE:
		having = "lessOrEquals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) + ")"
	case protoscommon.CohortsFilter_Aggregation_BETWEEN:
		if len(aggregation.GetValue()) < 2 {
			c.errors = append(c.errors, fmt.Errorf("aggregation need 2 values"))
			logger.Errorf("aggregation need 2 values")
			return
		}
		having = "greaterOrEquals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) +
			") AND lessOrEquals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[1]).(string) + ")"
	case protoscommon.CohortsFilter_Aggregation_NOT_BETWEEN:
		if len(aggregation.GetValue()) < 2 {
			c.errors = append(c.errors, fmt.Errorf("aggregation need 2 values"))
			logger.Errorf("aggregation need 2 values")
			return
		}
		having = "not(greaterOrEquals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[0]).(string) +
			") AND lessOrEquals(agg," + anyutil.ProtoToClickhouseValue(aggregation.GetValue()[1]).(string) + "))"
	}
	return having
}

func (c *cohortAdaptor) filterQueryAggregation(logger *log.SentioLogger, meta timeseries.Meta,
	aggregation *protoscommon.CohortsFilter_Aggregation) (agg string) {
	switch aggregation.GetKey().(type) {
	case *protoscommon.CohortsFilter_Aggregation_Total_:
		agg = "count() AS agg"
	case *protoscommon.CohortsFilter_Aggregation_AggregateProperties_:
		field := aggregation.GetAggregateProperties().GetPropertyName()
		fieldType, ok := meta.GetFieldType(field)
		if !ok {
			c.errors = append(c.errors, fmt.Errorf("field not found: %s", field))
			logger.Errorf("field not found: %s", field)
		}
		switch aggregation.GetAggregateProperties().GetType() {
		case protoscommon.CohortsFilter_Aggregation_AggregateProperties_SUM:
			agg = fmt.Sprintf("sum(%s) AS agg", clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType))
		case protoscommon.CohortsFilter_Aggregation_AggregateProperties_AVG:
			agg = fmt.Sprintf("avg(%s) AS agg", clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType))
		case protoscommon.CohortsFilter_Aggregation_AggregateProperties_MAX:
			agg = fmt.Sprintf("max(%s) AS agg", clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType))
		case protoscommon.CohortsFilter_Aggregation_AggregateProperties_MIN:
			agg = fmt.Sprintf("min(%s) AS agg", clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType))
		case protoscommon.CohortsFilter_Aggregation_AggregateProperties_DISTINCT_COUNT:
			agg = fmt.Sprintf("uniqExact(%s) AS agg", clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType))
		case protoscommon.CohortsFilter_Aggregation_AggregateProperties_LAST:
			agg = "argMax(" + clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType) + "," +
				"tuple(" +
				timeseries.SystemFieldPrefix + "block_number," +
				timeseries.SystemFieldPrefix + "transaction_index," +
				timeseries.SystemFieldPrefix + "log_index)" +
				") AS agg"
		case protoscommon.CohortsFilter_Aggregation_AggregateProperties_FIRST:
			agg = "argMin(" + clickhouse.DbNullableTypeCasting(timeseries.EscapeEventlogFieldName(field), fieldType) + "," +
				"tuple(" +
				timeseries.SystemFieldPrefix + "block_number," +
				timeseries.SystemFieldPrefix + "transaction_index," +
				timeseries.SystemFieldPrefix + "log_index)" +
				") AS agg"
		default:
			c.errors = append(c.errors, fmt.Errorf("unknown aggregation type: %v", aggregation.GetAggregateProperties().GetType()))
			logger.Errorf("unknown aggregation type: %v", aggregation.GetAggregateProperties().GetType())
		}
	}
	return
}

func (c *cohortAdaptor) filterQuery(groupIdx, filterIdx int, filter *protoscommon.CohortsFilter) error {
	filterJSON, _ := protojson.Marshal(filter)
	logger := c.logger.With("group_idx", groupIdx, "filter_idx", filterIdx, "filter", string(filterJSON))
	meta, ok := c.meta[filter.GetName()]
	if !ok {
		c.errors = append(c.errors, fmt.Errorf("meta not found: %s", filter.GetName()))
		return fmt.Errorf("meta not found: %s", filter.GetName())
	}
	if filter.GetAggregation() == nil {
		c.errors = append(c.errors, fmt.Errorf("aggregation not found"))
	}

	var (
		timeRange              *timerange.TimeRange
		timeCond, selectorCond = "1", "1"
		aggregateField, having = c.filterQueryAggregation(logger, meta, filter.GetAggregation()), c.filterQueryHaving(logger, filter.GetAggregation())
		err                    error
	)

	if filter.GetTimeRange() != nil {
		timeRange, err = timerange.NewTimeRangeFromLite(c.ctx, filter.GetTimeRange())
		if err != nil {
			c.errors = append(c.errors, err)
			logger.Errorf("new time range failed: %v", err)
		}
		timeCond = fmt.Sprintf("%s BETWEEN toDateTime64('%s', 6, 'UTC') AND toDateTime64('%s', 6, 'UTC')",
			timeseries.SystemTimestamp,
			timeRange.Start.UTC().Format("2006-01-02 15:04:05"),
			timeRange.End.UTC().Format("2006-01-02 15:04:05"),
		)
	}
	if filter.GetSelectorExpr() != nil {
		selectorExpression := NewSelectorExpression2(c.ctx, filter.GetSelectorExpr(), meta)
		selectorCond = selectorExpression.String()
	}

	if len(c.errors) > 0 {
		return fmt.Errorf("cte query failed, groupIdx: %d, filterIdx: %d, err: %v", groupIdx, filterIdx, c.errors[len(c.errors)-1])
	}

	const (
		selectTpl = "SELECT groupArray({user_field}) as a FROM (SELECT {user_field}, {agg_field} FROM `{table}` WHERE {cond} GROUP BY {user_field} HAVING {having} ORDER BY agg DESC)"
		totalTpl  = "SELECT groupArray({user_field}) as a FROM (SELECT DISTINCT {user_field} FROM `{table}` WHERE {cond})"
		diffTpl   = "SELECT arrayFilter(x -> NOT has((SELECT a FROM {select_tpl}), x), (SELECT a FROM {total_tpl})) as a"
	)
	args := map[string]any{
		"user_field": timeseries.SystemUserID,
		"agg_field":  aggregateField,
		"table":      c.store.MetaTable(meta),
		"cond":       strings.Join([]string{timeCond, selectorCond}, " AND "),
		"having":     having,
	}
	if filter.GetSymbol() {
		c.cte = append(c.cte, cte.CTE{
			Alias: c.cteName(groupIdx, filterIdx, "filter"),
			Query: builder.FormatSQLTemplate(selectTpl, args),
		})
	} else {
		args["select_tpl"] = c.cteName(groupIdx, filterIdx, "filter_select")
		args["total_tpl"] = c.cteName(groupIdx, filterIdx, "filter_total")
		c.cte = append(c.cte, cte.CTE{
			Alias: c.cteName(groupIdx, filterIdx, "filter_select"),
			Query: builder.FormatSQLTemplate(selectTpl, args),
		})
		c.cte = append(c.cte, cte.CTE{
			Alias: c.cteName(groupIdx, filterIdx, "filter_total"),
			Query: builder.FormatSQLTemplate(totalTpl, args),
		})
		c.cte = append(c.cte, cte.CTE{
			Alias: c.cteName(groupIdx, filterIdx, "filter"),
			Query: builder.FormatSQLTemplate(diffTpl, args),
		})
	}
	return nil
}

func (c *cohortAdaptor) groupQuery(groupIdx int, group *protoscommon.CohortsGroup) error {
	var (
		op        = group.GetJoinOperator()
		unionName = c.cteName(groupIdx, math.MaxInt32, "group")
		aggName   = c.cteName(groupIdx, math.MaxInt32, "agg")
		elements  []string
	)
	for filterIdx, filter := range group.GetFilters() {
		if err := c.filterQuery(groupIdx, filterIdx, filter); err != nil {
			return c.Error()
		}
		elements = append(elements, "SELECT a FROM `"+c.cteName(groupIdx, filterIdx, "filter")+"`")
	}
	c.cte = append(c.cte, cte.CTE{
		Alias: unionName,
		Query: strings.Join(elements, " UNION ALL "),
	})
	switch op {
	case protoscommon.JoinOperator_AND:
		c.cte = append(c.cte, cte.CTE{
			Alias: aggName,
			Query: "SELECT groupArrayIntersect(a) as a FROM `" + unionName + "`",
		})
	case protoscommon.JoinOperator_OR:
		c.cte = append(c.cte, cte.CTE{
			Alias: aggName,
			Query: "SELECT arrayUnion(groupArrayArray(a)) as a FROM `" + unionName + "`",
		})
	default:
		c.logger.Warnf("unknown join operator: %v", op)
		c.errors = append(c.errors, fmt.Errorf("unknown join operator: %v", op))
		return fmt.Errorf("unknown join operator: %v", op)
	}
	return nil
}

func (c *cohortAdaptor) Add(op protoscommon.JoinOperator, groups ...*protoscommon.CohortsGroup) CohortAdaptor {
	var elements []string
	for groupIdx, group := range groups {
		if err := c.groupQuery(groupIdx, group); err != nil {
			return c
		}
		elements = append(elements,
			"SELECT a FROM `"+c.cteName(groupIdx, math.MaxInt32, "agg")+"`")
	}
	c.cte = append(c.cte, cte.CTE{
		Alias: c.cteName(math.MaxInt32, math.MaxInt32, "group"),
		Query: strings.Join(elements, " UNION ALL "),
	})
	switch op {
	case protoscommon.JoinOperator_AND:
		c.cte = append(c.cte, cte.CTE{
			Alias: c.cteName(math.MaxInt32, math.MaxInt32, "final"),
			Query: "SELECT groupArrayIntersect(a) as a FROM `" + c.cteName(math.MaxInt32, math.MaxInt32, "group") + "`",
		})
	case protoscommon.JoinOperator_OR:
		c.cte = append(c.cte, cte.CTE{
			Alias: c.cteName(math.MaxInt32, math.MaxInt32, "final"),
			Query: "SELECT arrayUnion(groupArrayArray(a)) as a FROM `" + c.cteName(math.MaxInt32, math.MaxInt32, "group") + "`",
		})
	default:
		c.logger.Warnf("unknown join operator: %v", op)
		c.errors = append(c.errors, fmt.Errorf("unknown join operator: %v", op))
		return c
	}
	return c
}

func (c *cohortAdaptor) Build() string {
	if err := c.Error(); err != nil {
		c.logger.Errorf("error: %s", err)
		return err.Error()
	}

	var sql string
	const (
		tpl               = "{with} SELECT {field} FROM {final}"
		dataTable         = "__data__"
		userPropertyTable = "__user_property__"
		userTable         = "__user__"
		userPropertyTpl   = "SELECT " +
			"{user_field} as " + string(matrix.CohortUser) + "," +
			"max({timestamp_field}) as " + string(matrix.CohortUpdatedAt) + "," +
			"argMax({chain_field}, {timestamp_field}) as " + string(matrix.CohortChain) + "," +
			"count() as " + string(matrix.CohortAgg) + " FROM {user_property_table} {where} GROUP BY {user_field}"
		wrappedTpl      = "{with} SELECT * FROM {user_table} {where} {order} {limit} {offset}"
		wrappedCountTpl = "{with} SELECT uniqExact(user) as " + string(matrix.CohortAgg) + " FROM {user_table} {where}"
	)
	switch {
	case c.fetchUserProperty, c.countUser:
		var (
			elements         []string
			userPropertyCond string
		)
		switch len(c.cte) {
		case 0:
			// total users
			for _, m := range c.meta {
				elements = append(elements,
					"SELECT "+timeseries.SystemUserID+","+
						timeseries.SystemTimestamp+","+
						timeseries.SystemFieldPrefix+"chain FROM "+
						c.store.MetaTable(m))
			}
		default:
			c.cte = append(c.cte, cte.CTE{
				Alias: dataTable,
				Query: builder.FormatSQLTemplate(tpl, map[string]any{
					"with":  "",
					"field": "a",
					"final": c.cteName(math.MaxInt32, math.MaxInt32, "final"),
				})})
			for _, m := range c.meta {
				elements = append(elements,
					"SELECT "+timeseries.SystemUserID+","+
						timeseries.SystemTimestamp+","+
						timeseries.SystemFieldPrefix+"chain FROM "+
						c.store.MetaTable(m)+" WHERE has((select a from "+dataTable+"),"+timeseries.SystemUserID+")")
			}
			userPropertyCond = "WHERE has((select a from " + dataTable + "), user)"
		}
		c.cte = append(c.cte, cte.CTE{
			Alias: userPropertyTable,
			Query: strings.Join(elements, " UNION ALL "),
		})
		c.cte = append(c.cte, cte.CTE{
			Alias: userTable,
			Query: builder.FormatSQLTemplate(userPropertyTpl, map[string]any{
				"user_field":          timeseries.SystemUserID,
				"timestamp_field":     timeseries.SystemTimestamp,
				"chain_field":         timeseries.SystemFieldPrefix + "chain",
				"user_property_table": userPropertyTable,
				"where":               userPropertyCond,
			}),
		})
		var tpl = lo.If(c.fetchUserProperty, wrappedTpl).ElseIf(c.countUser, wrappedCountTpl).Else("")
		sql = builder.FormatSQLTemplate(tpl, map[string]any{
			"with":       c.cte.String(),
			"user_table": userTable,
			"where":      lo.If(c.search != "", " WHERE lower(user) like '%"+strings.ToLower(c.search)+"%' ").Else(""),
			"order":      lo.If(len(c.order) > 0, " ORDER BY "+strings.Join(c.order, ", ")).Else(""),
			"limit":      lo.If(c.limit > 0, " LIMIT "+strconv.Itoa(c.limit)).Else(""),
			"offset":     lo.If(c.offset > 0, " OFFSET "+strconv.Itoa(c.offset)).Else(""),
		})
	default:
		sql = builder.FormatSQLTemplate(tpl, map[string]any{
			"with":  c.cte.String(),
			"field": "arrayJoin(a) as a",
			"final": c.cteName(math.MaxInt32, math.MaxInt32, "final"),
		})
	}
	c.logger.Infof("sql: %s", sql)
	return sql
}

func (c *cohortAdaptor) FetchUserProperty() CohortAdaptor {
	c.logger = c.logger.With("fetch_user_property", "true")
	c.fetchUserProperty = true
	return c
}

func (c *cohortAdaptor) CountUser() CohortAdaptor {
	c.logger = c.logger.With("count_user", "true")
	c.countUser = true
	return c
}

func (c *cohortAdaptor) Order(order ...string) CohortAdaptor {
	c.logger = c.logger.With("order", order)
	c.order = append(c.order, order...)
	return c
}

func (c *cohortAdaptor) Limit(limit int) CohortAdaptor {
	c.logger = c.logger.With("limit", limit)
	c.limit = limit
	return c
}

func (c *cohortAdaptor) Offset(offset int) CohortAdaptor {
	c.logger = c.logger.With("offset", offset)
	c.offset = offset
	return c
}

func (c *cohortAdaptor) Search(search string) CohortAdaptor {
	c.logger = c.logger.With("search", search)
	c.search = search
	return c
}
