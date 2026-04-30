package chain

import (
	"fmt"
)

type Slot interface {
	GetNumber() uint64
	GetHash() string
	GetParentHash() string
	Linked() bool
}

func SlotSummary(b Slot) string {
	return fmt.Sprintf("[%d:%s->%s]", b.GetNumber(), b.GetParentHash(), b.GetHash())
}

func CheckLinkMismatch(left, right Slot) bool {
	if left == nil || right == nil {
		return false
	}
	return left.GetNumber()+1 == right.GetNumber() && left.GetHash() != right.GetParentHash()
}

func CheckLinksMismatch[SLOT Slot](slots []SLOT) error {
	for i := 1; i < len(slots); i++ {
		if slots[i-1].GetNumber()+1 != slots[i].GetNumber() || slots[i-1].GetHash() != slots[i].GetParentHash() {
			return fmt.Errorf("link mismatch between %s and %s", SlotSummary(slots[i-1]), SlotSummary(slots[i]))
		}
	}
	return nil
}
