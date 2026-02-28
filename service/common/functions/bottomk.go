package functions

import (
	"container/heap"
	"sort"

	"sentioxyz/sentio-core/service/common/protos"
)

type BottomK struct {
	k int
}

func NewBottomKHandler(arguments []*protos.Argument) *BottomK {
	if len(arguments) == 0 {
		return &BottomK{k: 20}
	}
	return &BottomK{k: parseArgument2Int(arguments[0], 20)}
}

type BottomKValueCell struct {
	matrixIdx int
	weight    float64
}

type BottomKValueCells []BottomKValueCell

func (t *BottomKValueCells) Len() int { return len(*t) }
func (t *BottomKValueCells) Less(i, j int) bool {
	return (*t)[i].weight > (*t)[j].weight
}

func (t *BottomKValueCells) Swap(i, j int) { (*t)[i], (*t)[j] = (*t)[j], (*t)[i] }

func (t *BottomKValueCells) Push(x interface{}) {
	*t = append(*t, x.(BottomKValueCell))
}

func (t *BottomKValueCells) Pop() interface{} {
	old := *t
	n := len(old)
	x := old[n-1]
	*t = old[0 : n-1]
	return x
}

func (t *BottomK) Handle(matrix *protos.Matrix) (*protos.Matrix, error) {
	timeMaxHeap := map[int64]heap.Interface{}
	var timeList timeList
	for sampleIdx, sample := range matrix.Samples {
		for _, value := range sample.Values {
			h, ok := timeMaxHeap[value.Timestamp]
			if !ok {
				h = &BottomKValueCells{}
				timeMaxHeap[value.Timestamp] = h
				timeList = append(timeList, value.Timestamp)
			}
			cell := BottomKValueCell{
				matrixIdx: sampleIdx,
				weight:    value.Value,
			}
			heap.Push(h, cell)
			if h.Len() > t.k {
				_ = heap.Pop(h)
			}
		}
	}
	sort.Sort(timeList)

	result := &protos.Matrix{
		TotalSamples: matrix.TotalSamples,
	}
	resultSamples := map[int]*protos.Matrix_Sample{}
	for _, ts := range timeList {
		h := timeMaxHeap[ts]
		for h.Len() > 0 {
			cell := heap.Pop(h).(BottomKValueCell)
			sample, ok := resultSamples[cell.matrixIdx]
			if !ok {
				sample = &protos.Matrix_Sample{
					Metric: matrix.Samples[cell.matrixIdx].Metric,
					Values: []*protos.Matrix_Value{},
				}
				resultSamples[cell.matrixIdx] = sample
				result.Samples = append(result.Samples, sample)
			}
			sample.Values = append(sample.Values, &protos.Matrix_Value{
				Timestamp: ts,
				Value:     cell.weight,
			})
		}
	}
	return result, nil
}

func (t *BottomK) Category() string {
	return "rank"
}
