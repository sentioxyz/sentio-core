package timerange

import (
	"testing"
	"time"
)

func TestResolveTimeStr(t *testing.T) {
	tests := []struct {
		input      string
		wantErr    bool
		wantResult time.Time
	}{
		{"now", false, time.Now().UTC()},
		{"now+1d", false, time.Now().Add(24 * time.Hour).UTC()},
		{"+1d", false, time.Now().Add(24 * time.Hour).UTC()},
		{"-1h", false, time.Now().Add(-1 * time.Hour).UTC()},
		{"-120d", false, time.Now().Add(-120 * 24 * time.Hour).UTC()},
		{"now-1M", false, time.Now().AddDate(0, -1, 0).UTC()},
		{"invalid", true, time.Time{}},
		{"1234567890", false, time.Unix(1234567890, 0).UTC()},
		{"1234567890-1h", false, time.Unix(1234567890, 0).Add(-1 * time.Hour).UTC()},
		{"1234567890123", false, time.UnixMilli(1234567890123).UTC()},
		{"1234567890123+1m", false, time.UnixMilli(1234567890123).Add(1 * time.Minute).UTC()},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ResolveTimeStr(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error but got none")
			} else if !tt.wantErr && err != nil {
				t.Fatalf("got an unexpected error: %v", err)
			}

			// Compare the times with a tolerance, since the time.Now() calls may differ slightly
			tolerance := 100 * time.Millisecond
			diff := result.Sub(tt.wantResult)
			if diff < 0 {
				diff = -diff
			}
			if !tt.wantErr && diff > tolerance {
				t.Fatalf("got %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestAlignTime(t *testing.T) {
	now := time.Now()

	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	startOfWeek := startOfDay.AddDate(0, 0, -int(now.Weekday()))

	tests := []struct {
		input      string
		asBegin    bool
		wantErr    bool
		wantResult time.Time
	}{
		// this month,
		{"now/M", true, false, startOfMonth},
		{"now/M", false, false, now},
		// this week
		{"now/w", true, false, startOfWeek},
		{"now/w", false, false, now},
		// last month
		{"now-1M/M", true, false, startOfMonth.AddDate(0, -1, 0)},
		{"now-1M/M", false, false, startOfMonth.Add(-1 * time.Nanosecond)},
		// last week
		{"now-1w/w", true, false, startOfWeek.AddDate(0, 0, -7)},
		{"now-1w/w", false, false, startOfWeek.Add(-1 * time.Nanosecond)},
		// yesterday
		{"now-1d/d", true, false, startOfDay.AddDate(0, 0, -1)},
		{"now-1d/d", false, false, startOfDay.Add(-1 * time.Nanosecond)},
		// last 3 months
		{"now-3M/M", true, false, startOfMonth.AddDate(0, -3, 0)},
		{"now-3M/M", false, false, startOfMonth.AddDate(0, -2, 0).Add(-1 * time.Nanosecond)},
		// this year
		{"now/y", true, false, time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)},
		{"now/y", false, false, now},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ResolveTimeStrWithAlign(tt.input, tt.asBegin, time.UTC)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error but got none")
			} else if !tt.wantErr && err != nil {
				t.Fatalf("got an unexpected error: %v", err)
			}

			// Compare the times with a tolerance, since the time.Now() calls may differ slightly
			tolerance := 100 * time.Millisecond
			diff := result.Sub(tt.wantResult)
			if diff < 0 {
				diff = -diff
			}
			if !tt.wantErr && diff > tolerance {
				t.Fatalf("got %v, want %v", result, tt.wantResult)
			}
		})
	}
}
