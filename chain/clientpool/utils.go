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
	// Append latest as the new back, keeping the queue strictly ordered: number strictly
	// increasing and timestamp non-decreasing. First pop any trailing entries that would break
	// that ordering against latest (number not below latest's, or timestamp after latest's) —
	// e.g. on a reorg/backoff — then push latest. latest always ends up in the queue.
	for {
		bc, has := q.Back()
		if !has || (bc.Number < latest.Number && !bc.Timestamp.After(latest.Timestamp)) {
			break
		}
		q.PopBack()
	}
	q.PushBack(latest)
	// Trim entries from the front whose timestamp is more than dur behind latest. Because
	// latest was just pushed (and latest.Timestamp.Sub(latest.Timestamp) == 0 <= dur), the
	// queue is never emptied here — so Front() always returns a real block.
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
