package data

import (
	"encoding/json"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func Test_CallStatistics(t *testing.T) {
	cs := NewCallStatistics(5, time.Millisecond*100)

	var w sync.WaitGroup
	for i := 0; i < 1000; i++ {
		time.Sleep(time.Millisecond)
		w.Add(1)
		go func(i int) {
			defer w.Done()
			startAt := time.Now()
			time.Sleep(time.Millisecond * time.Duration(rand.Int63n(10)))
			waitEndAt := time.Now()
			time.Sleep(time.Millisecond * time.Duration(rand.Int63n(10)))
			cs.Called("foo", []any{i}, nil, startAt, waitEndAt)
		}(i)
	}
	w.Wait()

	b, _ := json.MarshalIndent(cs.Snapshot(), "", "  ")
	t.Logf(string(b))
}
