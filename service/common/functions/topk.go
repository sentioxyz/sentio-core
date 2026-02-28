package functions

import (
	"container/heap"
	"sort"

	"sentioxyz/sentio-core/service/common/protos"
)

type TopK struct {
	k int
}

func NewTopKHandler(arguments []*protos.Argument) *TopK {
	if len(arguments) == 0 {
		return &TopK{k: 20}
	}
	return &TopK{k: parseArgument2Int(arguments[0], 20)}
}

type topKValueCell struct {
	matrixIdx int
	weight    float64
}

type topKValueCells []topKValueCell

func (t *topKValueCells) Len() int { return len(*t) }
func (t *topKValueCells) Less(i, j int) bool {
	return (*t)[i].weight < (*t)[j].weight
}

func (t *topKValueCells) Swap(i, j int) { (*t)[i], (*t)[j] = (*t)[j], (*t)[i] }

func (t *topKValueCells) Push(x interface{}) {
	*t = append(*t, x.(topKValueCell))
}

func (t *topKValueCells) Pop() interface{} {
	old := *t
	n := len(old)
	x := old[n-1]
	*t = old[0 : n-1]
	return x
}

func (t *TopK) Handle(matrix *protos.Matrix) (*protos.Matrix, error) {
	timeMaxHeap := map[int64]heap.Interface{}
	var timeList timeList
	for sampleIdx, sample := range matrix.Samples {
		for _, value := range sample.Values {
			h, ok := timeMaxHeap[value.Timestamp]
			if !ok {
				h = &topKValueCells{}
				timeMaxHeap[value.Timestamp] = h
				timeList = append(timeList, value.Timestamp)
			}
			cell := topKValueCell{
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
			cell := heap.Pop(h).(topKValueCell)
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

func (t *TopK) Category() string {
	return "rank"
}
