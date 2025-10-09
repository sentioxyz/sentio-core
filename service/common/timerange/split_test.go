package timerange

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSplitForDaylightSaving(t *testing.T) {
	tz, err := time.LoadLocation("Europe/Paris")
	require.NoError(t, err)
	start := AlignStartTime(time.Date(2024, 3, 27, 0, 0, 0, 0, time.UTC), 24*time.Hour, tz)
	end := AlignEndTime(time.Date(2024, 4, 4, 0, 0, 0, 0, time.UTC), 24*time.Hour, tz)

	batches := SplitBatch(start, end, 24*time.Hour, tz)
	// daylight saving time starts on 31st March 2024
	for _, batch := range batches {
		println(batch.Start.In(tz).String(), batch.End.In(tz).String())
	}
	require.Equal(t, 2, len(batches))
}

func TestSplitBatchForLongRange(t *testing.T) {
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 12, 30, 0, 0, 0, 0, time.UTC)

	batches := SplitBatch(start, end, 24*time.Hour, time.UTC)
	for _, batch := range batches {
		println(batch.Start.String(), batch.End.String())
	}
	require.Equal(t, 4, len(batches))
}

func TestSplitBatchShouldNoBatchShorRange(t *testing.T) {
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 2, 30, 0, 0, 0, 0, time.UTC)

	batches := SplitBatch(start, end, 24*time.Hour, time.UTC)

	require.Equal(t, 1, len(batches))
}

func TestSplitBatchShouldSplitForDaylightSavingTwice(t *testing.T) {
	tz, err := time.LoadLocation("Europe/Paris")
	require.NoError(t, err)
	start := AlignStartTime(time.Date(2024, 3, 27, 0, 0, 0, 0, time.UTC), 24*time.Hour, tz)
	end := AlignEndTime(time.Date(2024, 11, 4, 0, 0, 0, 0, time.UTC), 24*time.Hour, tz)

	batches := SplitBatch(start, end, 24*time.Hour, tz)
	// daylight saving time starts on 31st March 2024
	for _, batch := range batches {
		println(batch.Start.In(tz).String(), batch.End.In(tz).String())
	}
	require.Equal(t, 5, len(batches))
}
