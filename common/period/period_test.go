package period

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func buildTestcases() map[string]Period {
	return map[string]Period{
		"0s":     {},
		"1s":     {seconds: 1},
		"1m":     {seconds: 60},
		"5m":     {seconds: minute * 5},
		"30m":    {seconds: minute * 30},
		"1h":     {seconds: hour},
		"3601s":  {seconds: hour + 1},
		"2h":     {seconds: hour * 2},
		"12h":    {seconds: hour * 12},
		"1d":     {seconds: day},
		"86401s": {seconds: day + 1},
		"2d":     {seconds: day * 2},
		"1w":     {seconds: week},
		"30d":    {seconds: day * 30},
		"1month": {natualMonth: 1},
		"2month": {natualMonth: 2},
	}
}

func Test_period_cmp(t *testing.T) {
	periods := []Period{
		{},
		{seconds: 1},
		{seconds: 60},
		{seconds: minute * 5},
		{seconds: minute * 30},
		{seconds: hour},
		{seconds: hour + 1},
		{seconds: hour * 2},
		{seconds: hour * 12},
		{seconds: day},
		{seconds: day + 1},
		{seconds: day * 2},
		{seconds: week},
		{seconds: day * 29},
		{natualMonth: 1},
		{natualMonth: 2},
	}

	for i := 0; i+1 < len(periods); i++ {
		assert.Truef(t, periods[i].LT(periods[i+1]), "%s < %s", periods[i], periods[i+1])
		assert.Falsef(t, periods[i+1].LT(periods[i]), "%s > %s", periods[i+1], periods[i])
	}
}

func Test_period_string(t *testing.T) {
	for s, p := range buildTestcases() {
		assert.Equal(t, s, p.String())
	}

	x := struct {
		PA Period
		PB Period
	}{
		PA: Zero,
		PB: Month,
	}
	text, err := json.Marshal(&x)
	assert.NoError(t, err)
	assert.Equal(t, `{"PA":"0s","PB":"1month"}`, string(text))
}

func Test_parse(t *testing.T) {
	for s, p := range buildTestcases() {
		pp, err := Parse(s)
		assert.NoError(t, err)
		assert.Equal(t, p, pp)
	}
}

func Test_TimeBucket(t *testing.T) {
	parseTime := func(s string) time.Time {
		tt, _ := time.Parse(time.RFC3339, s)
		return tt
	}
	testcases := []struct {
		period Period
		src    time.Time
		dst    time.Time
	}{{
		period: Year,
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-01-01T00:00:00Z"),
	}, {
		period: Month.Multi(11),
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-10-01T00:00:00Z"),
	}, {
		period: Month.Multi(3),
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-10-01T00:00:00Z"),
	}, {
		period: Month,
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-12-01T00:00:00Z"),
	}, {
		period: Day,
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-12-26T00:00:00Z"),
	}, {
		period: Hour,
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-12-26T11:00:00Z"),
	}, {
		period: Minute,
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-12-26T11:22:00Z"),
	}, {
		period: Second.Multi(10),
		src:    parseTime("2024-12-26T11:22:33Z"),
		dst:    parseTime("2024-12-26T11:22:30Z"),
	}, {
		period: Year,
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-01-01T00:00:00Z"),
	}, {
		period: Month.Multi(11),
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-02-01T00:00:00Z"),
	}, {
		period: Month.Multi(3),
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-10-01T00:00:00Z"),
	}, {
		period: Month,
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-12-01T00:00:00Z"),
	}, {
		period: Day,
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-12-26T00:00:00Z"),
	}, {
		period: Hour,
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-12-26T11:00:00Z"),
	}, {
		period: Minute,
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-12-26T11:22:00Z"),
	}, {
		period: Second.Multi(10),
		src:    parseTime("1999-12-26T11:22:33Z"),
		dst:    parseTime("1999-12-26T11:22:30Z"),
	}}
	for i, testcase := range testcases {
		assert.Equal(t, testcase.dst, testcase.period.TimeBucket(testcase.src), fmt.Sprintf("testcase #%d: %v", i, testcase))
	}
}
