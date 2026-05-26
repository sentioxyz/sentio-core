package persistent

import (
	"sort"

	"sentioxyz/sentio-core/driver/entity/schema"
)

type changeHistory []*EntityBox
type changeSet map[string]map[string]changeHistory // key is [entity][id]

func (cs changeSet) Count(blockNumberLE uint64) (total int) {
	for _, set := range cs {
		for _, history := range set {
			total += history.Count(blockNumberLE)
		}
	}
	return total
}

func (cs changeSet) Split(blockNumber uint64) changeSet {
	ret := make(changeSet)
	for entity, set := range cs {
		newSet := make(map[string]changeHistory)
		for id, history := range set {
			after := history.Split(blockNumber)
			if len(history) == 0 {
				delete(set, id)
			} else {
				set[id] = history
			}
			if len(after) > 0 {
				newSet[id] = after
			}
		}
		if len(set) == 0 {
			delete(cs, entity)
		}
		if len(newSet) > 0 {
			ret[entity] = newSet
		}
	}
	return ret
}

func (cs changeSet) Snapshot() any {
	st := make(map[string]any)
	for entity, changes := range cs {
		var changeCount int
		for _, history := range changes {
			changeCount += len(history)
		}
		st[entity] = map[string]any{
			"idCount":     len(changes),
			"changeCount": changeCount,
		}
	}
	return st
}

func (ch *changeHistory) Count(blockNumberLE uint64) int {
	if ch == nil {
		return 0
	}
	n := len(*ch)
	if n == 0 {
		return 0
	}
	// let (*ch)[n].GenBlockNumber == +INF
	// so  (*ch)[n].GenBlockNumber > blockNumberLE
	// sort.Search return c so
	//     (*ch)[c-1].GenBlockNumber <= blockNumberLE &&
	//     (*ch)[c].GenBlockNumber > blockNumberLE
	// so count is c
	return sort.Search(n, func(i int) bool {
		return (*ch)[i].GenBlockNumber > blockNumberLE
	})
}

func (ch *changeHistory) Latest(blockNumber uint64) *EntityBox {
	if p := ch.Count(blockNumber); p > 0 {
		return (*ch)[p-1]
	}
	return nil
}

func (ch *changeHistory) Split(blockNumber uint64) changeHistory {
	i := ch.Count(blockNumber)
	if i == len(*ch) {
		return nil
	}
	ret := make(changeHistory, len(*ch)-i)
	copy(ret, (*ch)[i:])
	*ch = (*ch)[:i]
	return ret
}

func (ch *changeHistory) Push(entityType *schema.Entity, nw *EntityBox) (merged bool, mergedBox *EntityBox) {
	i := ch.Count(nw.GenBlockNumber)
	if i > 0 && (*ch)[i-1].GenBlockNumber == nw.GenBlockNumber {
		// just override (*ch)[i-1]
		(*ch)[i-1].Merge(entityType, nw)
		return true, (*ch)[i-1]
	}
	// rebuild the history by [ch[:i] + nw + ch[i:]]
	if i == len(*ch) {
		*ch = append(*ch, nw)
		return false, nw
	}
	*ch = append(*ch, nil)
	for j := len(*ch) - 1; j > i; j-- {
		(*ch)[j] = (*ch)[j-1]
	}
	(*ch)[i] = nw
	return false, nw
}
