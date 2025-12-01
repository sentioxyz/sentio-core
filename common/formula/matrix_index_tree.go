package formula

import "fmt"

type matrixIndex struct {
	value  int
	isNode bool
	index  map[string]*matrixIndex
}

func newMatrixIndex() *matrixIndex {
	return &matrixIndex{
		isNode: false,
	}
}

func (mi *matrixIndex) add(value int, key ...string) {
	if len(key) == 0 {
		mi.isNode = true
		mi.value = value
		return
	}
	if mi.index == nil {
		mi.index = make(map[string]*matrixIndex)
	}
	if _, ok := mi.index[key[0]]; !ok {
		mi.index[key[0]] = newMatrixIndex()
	}
	mi.index[key[0]].add(value, key[1:]...)
}

func (mi *matrixIndex) get(key ...string) int {
	if len(key) == 0 {
		return mi.value
	}
	if _, ok := mi.index[key[0]]; !ok {
		return -1
	}
	return mi.index[key[0]].get(key[1:]...)
}

func labelString(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}

type labelKV struct {
	key   string
	value string
}

func (l labelKV) String() string {
	return labelString(l.key, l.value)
}

type labelKVs []labelKV

func (kvs labelKVs) Len() int {
	return len(kvs)
}

func (kvs labelKVs) Less(i, j int) bool {
	return kvs[i].key < kvs[j].key
}

func (kvs labelKVs) Swap(i, j int) {
	kvs[i], kvs[j] = kvs[j], kvs[i]
}
