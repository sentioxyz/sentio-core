import type { ArgumentLike } from '../types/metrics'

export enum ArgumentType {
  String,
  Integer,
  Double,
  Bool,
  Duration
}

export interface ArgumentDef {
  name: string
  type: ArgumentType
}

export interface FunctionDef {
  name: string
  displayName?: string
  description: string
  arguments: ArgumentDef[]
  defaultArguments?: ArgumentLike[]
  deprecated?: boolean
}

const abs: FunctionDef = {
  name: 'abs',
  description: 'Returns the absolute value.',
  arguments: []
}

const ceil: FunctionDef = {
  name: 'ceil',
  description:
    'Returns the smallest integer greater than or equal to a number.',
  arguments: []
}

const floor: FunctionDef = {
  name: 'floor',
  description: 'Returns the largest integer less than or equal to a number.',
  arguments: []
}
const round: FunctionDef = {
  name: 'round',
  description: 'Returns the value of a number rounded to the nearest integer.',
  arguments: []
}
const log2: FunctionDef = {
  name: 'log2',
  description: 'Returns the base 2 logarithm.',
  arguments: []
}
const log10: FunctionDef = {
  name: 'log10',
  description: 'Returns the base 10 logarithm.',
  arguments: []
}
const ln: FunctionDef = {
  name: 'ln',
  description: 'Returns the natural logarithm.',
  arguments: []
}
const aggregations = ['avg', 'count', 'last', 'max', 'min', 'sum', 'delta']

const aggregationDescriptions: { [key: string]: string } = {
  avg: 'Calculates the sum of all values in the specified interval.',
  count: 'Calculates the number of values in the specified interval.',
  last: 'Calculates the last value in the specified interval.',
  max: 'Calculates the maximum of all values in the specified interval.',
  min: 'Calculates the minimum of all values in the specified interval.',
  sum: 'Calculates the sum of all values in the specified interval.',
  delta:
    'Calculates the difference between the first and last value in the specified interval.'
}

const aggregateOverTimeFunctions: FunctionDef[] = aggregations.map(
  (method): FunctionDef => ({
    name: `${method}_over_time`,
    description: aggregationDescriptions[method],
    arguments: [
      {
        name: 'interval',
        type: ArgumentType.Duration
      }
    ],
    defaultArguments: [{ durationValue: { value: 1, unit: 'm' } }]
  })
)

const rollupDescriptions: { [key: string]: string } = {
  avg: 'Roll up the metric by its average value over the specified time period.',
  count:
    'Roll up the metric by its count value over the specified time period.',
  last: 'Roll up the metric by its last value over the specified time period.',
  max: 'Roll up the metric by its maximum value over the specified time period.',
  min: 'Roll up the metric by its minimum value over the specified time period.',
  sum: 'Roll up the metric by its sum value over the specified time period.',
  delta: 'Roll up the metric by its delta value over the specified time period.'
}

const rollupFunctions: FunctionDef[] = aggregations.map(
  (method): FunctionDef => ({
    name: `rollup_${method}`,
    description: rollupDescriptions[method],
    arguments: [
      {
        name: 'interval',
        type: ArgumentType.Duration
      }
    ],
    defaultArguments: [{ durationValue: { value: 1, unit: 'm' } }]
  })
)

const rate: FunctionDef = {
  name: 'rate',
  description:
    'Calculates the per-second average rate of increase of the time series.',
  arguments: [
    {
      name: 'interval',
      type: ArgumentType.Duration
    }
  ],
  defaultArguments: [{ durationValue: { value: 1, unit: 'm' } }]
}

const irate: FunctionDef = {
  name: 'irate',
  description:
    'Calculates the per-second instant rate of increase of the time series.',
  arguments: [
    {
      name: 'interval',
      type: ArgumentType.Duration
    }
  ],
  defaultArguments: [{ durationValue: { value: 1, unit: 'm' } }]
}

const delta: FunctionDef = {
  name: 'delta',
  description:
    'Calculates the difference between the first and last value of each time series.',
  arguments: [
    {
      name: 'interval',
      type: ArgumentType.Duration
    }
  ],
  defaultArguments: [{ durationValue: { value: 1, unit: 'm' } }],
  deprecated: true
}

const moving_delta: FunctionDef = {
  name: 'moving_delta',
  description:
    'Calculates the difference between the first and last value of each time series. (continuously)',
  arguments: [
    {
      name: 'interval',
      type: ArgumentType.Duration
    }
  ],
  defaultArguments: [{ durationValue: { value: 1, unit: 'm' } }],
  deprecated: true
}

const topk: FunctionDef = {
  name: 'topk',
  description: 'Returns the top k elements by sample value.',
  arguments: [
    {
      name: 'k',
      type: ArgumentType.Integer
    }
  ],
  defaultArguments: [{ intValue: 1 }]
}

const bottomk: FunctionDef = {
  name: 'bottomk',
  description: 'Returns the bottom k elements by sample value.',
  arguments: [
    {
      name: 'k',
      type: ArgumentType.Integer
    }
  ],
  defaultArguments: [{ intValue: 1 }]
}

const timestamp: FunctionDef = {
  name: 'timestamp',
  description:
    'Returns the timestamp of each of the samples of the given vector as the number of seconds since January 1, 1970 UTC.',
  arguments: []
}
const day_of_week: FunctionDef = {
  name: 'day_of_week',
  description:
    'Returns the day of the week for each of the given times. (needs timestamp)',
  arguments: []
}
const day_of_month: FunctionDef = {
  name: 'day_of_month',
  description:
    'Returns the day of the month for each of the given times. (needs timestamp)',
  arguments: []
}

const day_of_year: FunctionDef = {
  name: 'day_of_year',
  description:
    'Returns the day of the year for each of the given times. (needs timestamp)',
  arguments: []
}

const month: FunctionDef = {
  name: 'month',
  description:
    'Returns the month of the given time. Returned values are from 1 to 12, where 1 means January etc. (needs timestamp)',
  arguments: []
}

const year: FunctionDef = {
  name: 'year',
  description: 'Returns the year of the given time. (needs timestamp)',
  arguments: []
}

const hour: FunctionDef = {
  name: 'hour',
  description:
    'Returns the hour of the given time. Returned values are from 0 to 23. (needs timestamp)',
  arguments: []
}

const minute: FunctionDef = {
  name: 'minute',
  description:
    'Returns the minute of the given time. Returned values are from 0 to 59. (needs timestamp)',
  arguments: []
}

const before: FunctionDef = {
  name: 'before',
  displayName: 'shift earlier',
  description: 'Shifts the vector back in time by the specified duration.',
  arguments: [
    {
      name: 'duration',
      type: ArgumentType.Duration
    }
  ],
  defaultArguments: [{ durationValue: { value: 1, unit: 'h' } }]
}

const after: FunctionDef = {
  name: 'after',
  displayName: 'shift later',
  description: 'Shifts the vector forward in time by the specified duration.',
  arguments: [
    {
      name: 'duration',
      type: ArgumentType.Duration
    }
  ],
  defaultArguments: [{ durationValue: { value: 1, unit: 'h' } }]
}

export const FunctionsCategories: { [category: string]: FunctionDef[] } = {
  Math: [abs, ceil, floor, round, log2, log10, ln],
  Rollup: rollupFunctions,
  'Aggregate Over Time': aggregateOverTimeFunctions,
  Rate: [rate, irate, delta, moving_delta],
  Rank: [topk, bottomk],
  Time: [
    timestamp,
    day_of_year,
    day_of_month,
    day_of_week,
    year,
    month,
    hour,
    minute
  ],
  TimeShift: [before, after]
}

export const FunctionMap: { [name: string]: FunctionDef } = Object.values(
  FunctionsCategories
).reduce(
  (acc, funcs) => {
    funcs.forEach((f) => {
      acc[f.name] = f
    })
    return acc
  },
  {} as { [name: string]: FunctionDef }
)

export function isAggrOrRollupFunction(name: string) {
  const f = FunctionMap[name]
  return f && (f.name.startsWith('rollup_') || f.name.endsWith('_over_time'))
}

const eventsDelta: FunctionDef = {
  name: 'delta',
  description:
    'Calculates the difference between the first and last value of each time series.',
  arguments: [
    {
      name: 'interval',
      type: ArgumentType.Duration
    }
  ],
  defaultArguments: [{ durationValue: { value: 1, unit: 'm' } }]
}

export const EventsFunctionCategories: { [category: string]: FunctionDef[] } = {
  Rank: [topk, bottomk],
  Delta: [eventsDelta]
}

export const EventsFunctionMap: { [name: string]: FunctionDef } = Object.values(
  EventsFunctionCategories
).reduce(
  (acc, funcs) => {
    funcs.forEach((f) => {
      acc[f.name] = f
    })
    return acc
  },
  {} as { [name: string]: FunctionDef }
)
