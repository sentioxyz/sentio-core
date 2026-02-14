package jsonrpc

import (
	"sentioxyz/sentio-core/common/log"
	"sync"
	"testing"
	"time"
)

func Test_bufPool(t *testing.T) {
	bp := NewBufPool(1024, 3, 5, time.Millisecond*100)
	for round := 0; round < 2; round++ {
		log.Infof("round #%d", round)
		var w sync.WaitGroup
		for i := 0; i < 10; i++ {
			w.Add(1)
			go func(i int) {
				defer w.Done()
				b := bp.Get()
				log.Infof("#%d got %d/%d", i, len(b), cap(b))
				time.Sleep(time.Second)
				b = append(b, byte(i))
				bp.Put(b)
			}(i)
		}
		w.Wait()
	}
	log.Infof("final: %v\n", bp.Snapshot())
	//assert.Equal(t, int64(10), bp.havingNum.Load())
}
