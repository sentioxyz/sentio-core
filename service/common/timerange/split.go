package timerange

import (
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"time"
)

const TimeRangeBatchLimit = 90

func SplitBatch(start time.Time, end time.Time, step time.Duration, tz *time.Location) []prometheus.Range {
	needDetectDayLightSaving := step >= 24*time.Hour
	var ret []prometheus.Range
	batchStart := start
	batchEnd := batchStart
	for batchStart.Before(end) {
		newEnd := batchEnd.Add(step)
		if newEnd.After(end) {
			batchEnd = end
			ret = append(ret, prometheus.Range{
				Start: batchStart,
				End:   batchEnd,
				Step:  step,
			})
			break
		}
		if needDetectDayLightSaving {
			isStartDst := batchStart.In(tz).IsDST()
			isEndDst := newEnd.In(tz).IsDST()
			if isStartDst != isEndDst {
				//crossing daylight saving time
				ret = append(ret, prometheus.Range{
					Start: batchStart,
					End:   batchEnd,
					Step:  step,
				})
				batchStart = AlignStartTime(newEnd, step, tz)
				batchEnd = batchStart
			}
		}
		sub := newEnd.Sub(batchStart)
		if sub >= TimeRangeBatchLimit*24*time.Hour && (sub/step > TimeRangeBatchLimit) {
			ret = append(ret, prometheus.Range{
				Start: batchStart,
				End:   batchEnd,
				Step:  step,
			})
			batchStart = newEnd
			batchEnd = batchStart
		}
		batchEnd = newEnd
	}

	return ret
}
