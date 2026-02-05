package period

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_SecondTicker(t *testing.T) {
	newTime := func(str string) time.Time {
		ti, _ := time.Parse(time.RFC3339, str)
		return ti
	}

	tk := NewTicker(Hour, newTime("2023-02-28T15:16:17Z"))
	assert.Equal(t, newTime("2023-02-28T16:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T17:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T18:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T19:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T20:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T21:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T22:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T23:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-03-01T00:00:00Z"), tk.Next())

	tk = NewTicker(Hour, newTime("2023-02-28T15:00:00Z"))
	assert.Equal(t, newTime("2023-02-28T16:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T17:00:00Z"), tk.Next())

	tk = NewTicker(Hour, newTime("2023-02-28T15:59:59Z"))
	assert.Equal(t, newTime("2023-02-28T16:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-02-28T17:00:00Z"), tk.Next())

	tk = NewTicker(Day, newTime("2023-02-27T15:16:17Z"))
	assert.Equal(t, newTime("2023-02-28T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-03-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-03-02T00:00:00Z"), tk.Next())

	tk = NewTicker(Day, newTime("2023-02-27T00:00:00Z"))
	assert.Equal(t, newTime("2023-02-28T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-03-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-03-02T00:00:00Z"), tk.Next())

	tk = NewTicker(Month, newTime("2023-02-27T15:16:17Z"))
	assert.Equal(t, newTime("2023-03-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-04-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-05-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-06-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-07-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-08-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-09-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-10-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-11-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-12-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2024-01-01T00:00:00Z"), tk.Next())

	tk = NewTicker(Month, newTime("2023-02-01T00:00:00Z"))
	assert.Equal(t, newTime("2023-03-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-04-01T00:00:00Z"), tk.Next())

	tk = NewTicker(Month.Multi(2), newTime("2023-02-27T15:16:17Z"))
	assert.Equal(t, newTime("2023-03-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-05-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-07-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-09-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2023-11-01T00:00:00Z"), tk.Next())
	assert.Equal(t, newTime("2024-01-01T00:00:00Z"), tk.Next())
}
