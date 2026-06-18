package data

import (
	"context"

	"sentioxyz/sentio-core/common/utils"
)

func BinarySearchContractStart(
	ctx context.Context,
	start, end uint64,
	checker func(context.Context, uint64) (bool, error),
) (uint64, bool, error) {
	if has, err := checker(ctx, end); err != nil {
		return 0, false, err
	} else if !has {
		return 0, false, nil
	}
	low, high := start, end
	for p := uint64(1); p > 0; p <<= 1 {
		if p <= start {
			low = p
		}
		if p >= end {
			high = p
			break
		}
	}
	internalChecker := func(ctx context.Context, bn uint64) (bool, error) {
		if bn < start {
			return false, nil
		}
		if bn >= end {
			return true, nil
		}
		return checker(ctx, bn)
	}
	for low < high {
		mid := (low + high) / 2
		if has, err := internalChecker(ctx, mid); err != nil {
			return 0, false, err
		} else if has {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low, true, nil
}

func GetFirst(firstConfig int64, latest uint64) uint64 {
	return uint64(max(0, utils.Select(firstConfig < 0, int64(latest)+firstConfig, firstConfig)))
}
