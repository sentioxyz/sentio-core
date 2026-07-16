package prebuilt_function

import (
	"testing"
	"time"

	"sentioxyz/sentio-core/service/common/timerange"

	"github.com/stretchr/testify/require"
)

func TestClickhouseAlignedTimeUsesTimezone(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	require.NoError(t, err)

	f := &BaseFunction{}
	tr := &timerange.TimeRange{
		Start:    time.Date(2026, 6, 16, 14, 55, 44, 0, time.UTC),
		End:      time.Date(2026, 7, 17, 14, 55, 44, 0, time.UTC),
		Step:     24 * time.Hour,
		Timezone: tokyo,
	}

	start := f.StartAlignedTime(tr)
	end := f.EndAlignedTime(tr)

	require.Equal(t,
		"toDateTime64(date_trunc('day', toDateTime64('2026-06-16 14:55:44', 6, 'UTC'), 'Asia/Tokyo'), 6, 'UTC')",
		start)
	require.Equal(t,
		"toDateTime64(date_trunc('day', toDateTime64('2026-07-17 14:55:44', 6, 'UTC'), 'Asia/Tokyo'), 6, 'UTC')",
		end)
}

func TestClickhouseAlignedTimeDefaultsToUTC(t *testing.T) {
	f := &BaseFunction{}
	tr := &timerange.TimeRange{
		Start:    time.Date(2026, 6, 16, 14, 55, 44, 0, time.UTC),
		End:      time.Date(2026, 7, 17, 14, 55, 44, 0, time.UTC),
		Step:     24 * time.Hour,
		Timezone: time.UTC,
	}

	require.Equal(t,
		"toDateTime64(date_trunc('day', toDateTime64('2026-06-16 14:55:44', 6, 'UTC'), 'UTC'), 6, 'UTC')",
		f.StartAlignedTime(tr))
}
