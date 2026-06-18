package standard

import (
	"time"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/processor/protos"
)

func NewIntervalConfig(c *protos.OnIntervalConfig) (data.IntervalConfig, error) {
	if c.GetSlot() > 0 || c.GetSlotInterval() != nil {
		interval := data.BlockInterval{
			Backfill: max(uint64(c.GetSlotInterval().GetBackfillInterval()), uint64(c.GetSlot()), minBackfillSlotInterval),
			Watching: max(uint64(c.GetSlotInterval().GetRecentInterval()), uint64(c.GetSlot()), 1),
		}
		return data.IntervalConfig{BlockInterval: &interval}, nil
	} else if c.GetMinutes() > 0 || c.GetMinutesInterval() != nil {
		interval := data.TimeInterval{
			Backfill: max(
				time.Duration(c.GetMinutesInterval().GetBackfillInterval())*time.Minute,
				time.Duration(c.GetMinutes())*time.Minute,
				minBackfillTimeInterval),
			Watching: max(
				time.Duration(c.GetMinutesInterval().GetRecentInterval())*time.Minute,
				time.Duration(c.GetMinutes())*time.Minute,
				time.Minute),
		}
		return data.IntervalConfig{TimeInterval: &interval}, nil
	} else {
		return data.IntervalConfig{}, errors.Errorf("invalid interval config %#v", c)
	}
}
