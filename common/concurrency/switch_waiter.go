package concurrency

import "context"

type SwitchWaiter struct {
	*StatusWaiter[bool]
}

func NewSwitchWaiter(initOn bool) *SwitchWaiter {
	return &SwitchWaiter{
		StatusWaiter: NewStatusWaiter(initOn),
	}
}

func (sw *SwitchWaiter) Change(on bool) {
	sw.StatusWaiter.NewStatus(on)
}

func (sw *SwitchWaiter) Wait(ctx context.Context, on bool) error {
	_, err := sw.StatusWaiter.Wait(ctx, func(cur bool) bool {
		return cur == on
	})
	return err
}
