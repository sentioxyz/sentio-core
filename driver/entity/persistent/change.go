package persistent

import (
	"sort"

	"sentioxyz/sentio-core/driver/entity/schema"
)

type changeHistory []*UncommittedEntityBox
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

func (ch *changeHistory) Latest(blockNumber uint64) *UncommittedEntityBox {
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

// Push stores nw, a write at block nw.GenBlockNumber. When the history already has an entry for
// that block, nw is merged into it and mergedBox is the merged entry, otherwise nw itself is
// inserted in block order. validate, when given, sees the box that is about to be stored and can
// reject it: the history is only changed after it returns nil, so a rejected write leaves no trace.
func (ch *changeHistory) Push(
	entityType *schema.Entity,
	nw *UncommittedEntityBox,
	validate func(box *UncommittedEntityBox) error,
) (merged bool, mergedBox *UncommittedEntityBox, err error) {
	i := ch.Count(nw.GenBlockNumber)
	if i > 0 && (*ch)[i-1].GenBlockNumber == nw.GenBlockNumber {
		// just override (*ch)[i-1]
		if mergedBox, err = (*ch)[i-1].Merged(entityType, nw); err != nil {
			return true, nil, err
		}
		if validate != nil {
			if err = validate(mergedBox); err != nil {
				return true, nil, err
			}
		}
		*(*ch)[i-1] = *mergedBox
		return true, (*ch)[i-1], nil
	}
	if validate != nil {
		if err = validate(nw); err != nil {
			return false, nil, err
		}
	}
	// rebuild the history by [ch[:i] + nw + ch[i:]]
	if i == len(*ch) {
		*ch = append(*ch, nw)
		return false, nw, nil
	}
	*ch = append(*ch, nil)
	for j := len(*ch) - 1; j > i; j-- {
		(*ch)[j] = (*ch)[j-1]
	}
	(*ch)[i] = nw
	return false, nw, nil
}
