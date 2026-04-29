package clientpool

import (
	"crypto/sha1"
	"encoding/hex"
	"sentioxyz/sentio-core/common/queue"
	"time"
)

func pushLatestQueue(q queue.Queue[Block], latest Block, dur time.Duration) (queue.Queue[Block], time.Duration) {
	if q == nil {
		q = queue.NewQueue[Block]()
	}
	if bc, has := q.Back(); !has || (bc.Number < latest.Number && bc.Timestamp.Before(latest.Timestamp)) {
		q.PushBack(latest)
	}
	// here q will never be empty
	var fr Block
	for {
		fr, _ = q.Front()
		if latest.Timestamp.Sub(fr.Timestamp) <= dur {
			break
		}
		q.PopFront()
	}
	if fr.Number < latest.Number && fr.Timestamp.Before(latest.Timestamp) {
		return q, latest.Timestamp.Sub(fr.Timestamp) / time.Duration(latest.Number-fr.Number)
	}
	return q, 0
}

func BuildPublicName(name string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(name))
	return hex.EncodeToString(h.Sum(nil))
}

const MethodNotSupportedTagPrefix = "MethodNotSupported/"

func MethodNotSupportedTag(method string) string {
	return MethodNotSupportedTagPrefix + method
}
