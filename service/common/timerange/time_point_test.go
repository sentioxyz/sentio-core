package timerange

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_TimePoint(t *testing.T) {
	t2023 := time.Unix(1672574400, 0)
	a := NewTimePoint(t2023, time.Hour, nil)
	b := a.Next()
	require.EqualValues(t, a.Day(), b.Day())
	require.EqualValues(t, 13, b.Hour())
	require.EqualValues(t, 1672578000, b.Unix())

	t2022 := time.Unix(1664553600, 0)
	tz, _ := time.LoadLocation("Asia/Shanghai")
	a = NewTimePoint(t2022, time.Hour*24, tz)
	b = a.Next()
	require.EqualValues(t, 2, b.Day())
	require.EqualValues(t, 0, b.Hour())
	require.EqualValues(t, 1664640000, b.Unix())

	a = NewTimePoint(t2022, time.Hour*24*30, tz)
	b = a.Pre()
	require.EqualValues(t, 1, b.Day())
	require.EqualValues(t, 0, b.Hour())
	require.EqualValues(t, 9, b.Month())
	require.EqualValues(t, 1661961600, b.Unix())
}
