package jsonrpc

import (
	"encoding/json"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type statWindow struct {
	startAt time.Time
	used    map[string]map[string]timehist.Histogram // map[<method>][<source>]
}

func newStatWindow(method string, src RequestSource, used time.Duration) *statWindow {
	return &statWindow{
		startAt: time.Now(),
		used: map[string]map[string]timehist.Histogram{
			method: {
				src.Summary(): timehist.Histogram{}.Incr(used),
			},
		},
	}
}

func (w *statWindow) GetStartAt() time.Time {
	return w.startAt
}

func (w *statWindow) Merge(a *statWindow) {
	for k, stat := range a.used {
		for pid, h := range stat {
			wh, _ := utils.GetFromK2Map(w.used, k, pid)
			utils.PutIntoK2Map(w.used, k, pid, wh.Add(h))
		}
	}
}

func (w *statWindow) Snapshot(endAt time.Time) any {
	return map[string]any{
		"startAt":  w.startAt,
		"endAt":    endAt,
		"duration": endAt.Sub(w.startAt),
		"methodUsed": utils.MapMapNoError(w.used, func(stat map[string]timehist.Histogram) string {
			return utils.ReduceMapValues(stat, timehist.Histogram.Add).String()
		}),
		"methodCount": utils.MapMapNoError(w.used, func(stat map[string]timehist.Histogram) int {
			return utils.ReduceMapValues(stat, timehist.Histogram.Add).Sum()
		}),
		"sourceCount": utils.MapMapNoError(
			utils.MergeMapSumByFunc(utils.GetMapValuesOrderByKey(w.used), timehist.Histogram.Add),
			timehist.Histogram.Sum,
		),
		"methodSourceCount": utils.MapMapNoError(w.used, func(stat map[string]timehist.Histogram) map[string]int {
			return utils.MapMapNoError(stat, timehist.Histogram.Sum)
		}),
	}
}

const (
	slowQueryUsed              = time.Second * 5
	bigQueryResponseSize       = 10 << 20 // 10MB
	bigQueryResponseEncodeUsed = time.Second
)

type requestSample struct {
	RequestID    uint64
	RequestSubID uint64
	Source       RequestSource
	RequestTime  time.Time
	RequestBody  json.RawMessage
	Used         time.Duration
}

type slowRequest struct {
	requestSample
}

type failedRequest struct {
	requestSample

	Error string
}

type bigRequest struct {
	requestSample

	ResponseSize       int
	ResponseEncodeUsed time.Duration
}

func (r requestSample) Snapshot() map[string]any {
	return map[string]any{
		"requestId":    r.RequestID,
		"requestSubId": r.RequestSubID,
		"source":       r.Source,
		"requestTime":  r.RequestTime.String(),
		"requestBody":  utils.StringSummaryV1(string(r.RequestBody), 5*1024),
		"used":         r.Used.String(),
	}
}

func (r slowRequest) Snapshot() any {
	return r.requestSample.Snapshot()
}

func (r failedRequest) Snapshot() any {
	sn := r.requestSample.Snapshot()
	sn["error"] = r.Error
	return sn
}

func (r bigRequest) Snapshot() any {
	sn := r.requestSample.Snapshot()
	sn["responseSize"] = r.ResponseSize
	sn["responseEncodeUsed"] = r.ResponseEncodeUsed.String()
	return sn
}
