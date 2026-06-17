import type { DurationLike } from '@sentio/ui-core'

/*
 * Metrics-query shapes — minimal structural interfaces for the timeseries query
 * form. All fields optional so a consumer can pass its own (richer) generated
 * query objects directly; they assign structurally. Mirrors the proto `Query`
 * / `Aggregate` / `Function` / `Argument` / `MetricInfo` messages, but only the
 * fields the timeseries components actually read or write.
 */

/** Aggregate operation. Mirror of the proto `Aggregate.AggregateOps` enum. */
export type AggregateOpsLike = 'AVG' | 'SUM' | 'MIN' | 'MAX' | 'COUNT'

/** A single function argument — exactly one value field is set. */
export interface ArgumentLike {
  stringValue?: string
  intValue?: number
  doubleValue?: number | 'NaN' | 'Infinity' | '-Infinity'
  boolValue?: boolean
  durationValue?: DurationLike
}

/** A query function (e.g. `rate`, `rollup_avg`) with its arguments. */
export interface FunctionLike {
  name?: string
  arguments?: ArgumentLike[]
}

/** Aggregation config on a query. */
export interface AggregateLike {
  op?: AggregateOpsLike
  grouping?: string[]
}

/** A metrics query. `query` is the metric name; the rest are refinements. */
export interface QueryLike {
  query?: string
  aggregate?: AggregateLike
  labelSelector?: { [key: string]: string }
  functions?: FunctionLike[]
}

/** Metric metadata used to populate label/aggregate option lists. */
export interface MetricInfoLike {
  contractName?: string[]
  contractAddress?: string[]
  chainId?: string[]
  labels?: { [key: string]: { values?: string[] } }
}
