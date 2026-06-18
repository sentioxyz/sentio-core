package fetcher

import (
	"time"

	"sentioxyz/sentio-core/common/timehist"
)

type processStat struct {
	startAt time.Time

	fetchUsed            timehist.Histogram
	fetchTotalUsed       time.Duration
	fetchFailedCount     int
	fetchQueryBlockRange uint64
	fetchGotDataSize     int

	getUsed       timehist.Histogram
	getTotalUsed  time.Duration
	getEmptyCount int
}

func (s *processStat) GetStartAt() time.Time {
	return s.startAt
}

func (s *processStat) Merge(a *processStat) {
	s.fetchUsed = s.fetchUsed.Add(a.fetchUsed)
	s.fetchTotalUsed += a.fetchTotalUsed
	s.fetchFailedCount += a.fetchFailedCount
	s.fetchQueryBlockRange += a.fetchQueryBlockRange
	s.fetchGotDataSize += a.fetchGotDataSize

	s.getEmptyCount += a.getEmptyCount
	s.getUsed = s.getUsed.Add(a.getUsed)
	s.getTotalUsed += a.getTotalUsed
}

func (s *processStat) Snapshot(endAt time.Time) any {
	sn := map[string]any{
		"startAt":  s.startAt.String(),
		"endAt":    endAt.String(),
		"duration": endAt.Sub(s.startAt).String(),
	}
	if fetchCount := s.fetchUsed.Sum(); fetchCount > 0 {
		sn["fetch"] = map[string]any{
			"count":                fetchCount,
			"failedCount":          s.fetchFailedCount,
			"totalUsed":            s.fetchTotalUsed.String(),
			"used":                 s.fetchUsed.String(),
			"avgUsed":              (s.fetchTotalUsed / time.Duration(fetchCount)).String(),
			"totalGotDataSize":     s.fetchGotDataSize,
			"avgGotDataSize":       s.fetchGotDataSize / fetchCount,
			"totalQueryBlockRange": s.fetchQueryBlockRange,
			"avgQueryBlockRange":   s.fetchQueryBlockRange / uint64(fetchCount),
			"pressure":             float64(s.fetchTotalUsed) / float64(endAt.Sub(s.startAt)),
		}
	}
	if getCount := s.getUsed.Sum(); getCount > 0 {
		sn["get"] = map[string]any{
			"count":      getCount,
			"emptyCount": s.getEmptyCount,
			"totalUsed":  s.getTotalUsed,
			"used":       s.getUsed.String(),
			"avgUsed":    (s.getTotalUsed / time.Duration(getCount)).String(),
		}
	}
	return sn
}

func (s *processStat) fetchComplete(used time.Duration, succeed bool, queryRange uint64, dataSize int) {
	s.fetchUsed = timehist.Histogram{}.Incr(used)
	s.fetchTotalUsed = used
	if !succeed {
		s.fetchFailedCount = 1
	}
	s.fetchQueryBlockRange = queryRange
	s.fetchGotDataSize = dataSize
}

func (s *processStat) getComplete(used time.Duration, has bool) {
	s.getUsed = timehist.Histogram{}.Incr(used)
	s.getTotalUsed = used
	if !has {
		s.getEmptyCount = 1
	}
}
