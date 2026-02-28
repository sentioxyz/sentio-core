package functions

import (
	"sort"

	"sentioxyz/sentio-core/service/common/protos"
)

type TruncateType int
type TruncateOrder int

const (
	TruncateAsc  TruncateOrder = 0
	TruncateDesc TruncateOrder = 1

	TruncateByLastValue TruncateType = 0
)

type Truncate struct {
	limit         int
	offset        int
	truncateType  TruncateType
	truncateOrder TruncateOrder
}

func NewTruncateHandler(arguments []*protos.Argument) *Truncate {
	var (
		limit, offset = 20, 0
		truncateType  = TruncateByLastValue
		truncateOrder = TruncateDesc
	)
	if len(arguments) > 0 {
		if n := parseArgument2Int(arguments[0], limit); n > 0 {
			limit = n
		}
	}
	if len(arguments) > 1 {
		if n := parseArgument2Int(arguments[1], offset); n > 0 {
			offset = n
		}
	}
	if len(arguments) > 2 {
		truncateType = TruncateType(parseArgument2Int(arguments[2], int(TruncateByLastValue)))
	}
	if len(arguments) > 3 {
		truncateOrder = TruncateOrder(parseArgument2Int(arguments[3], int(TruncateDesc)))
	}
	return &Truncate{
		limit:         limit,
		offset:        offset,
		truncateType:  truncateType,
		truncateOrder: truncateOrder,
	}
}

type truncateCell struct {
	sample *protos.Matrix_Sample
	weight float64
}

type truncateCells []*truncateCell

func (s truncateCells) Less(i, j int) bool {
	return s[i].weight < s[j].weight
}

func (s truncateCells) Len() int {
	return len(s)
}

func (s truncateCells) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func lastValue(sample *protos.Matrix_Sample) float64 {
	var maxTimestamp int64 = 0
	var weight float64 = 0
	if sample == nil {
		return 0
	}
	for _, value := range sample.Values {
		if value.Timestamp > maxTimestamp {
			maxTimestamp = value.Timestamp
			weight = value.Value
		}
	}
	return weight
}

func (t *Truncate) Handle(matrix *protos.Matrix) (*protos.Matrix, error) {
	if t.limit <= 0 {
		matrix.Samples = make([]*protos.Matrix_Sample, 0)
		return matrix, nil
	}
	if t.offset < 0 {
		t.offset = 0
	}
	var samples truncateCells
	for _, sample := range matrix.Samples {
		if t.truncateType == TruncateByLastValue {
			samples = append(samples, &truncateCell{
				sample: sample,
				weight: lastValue(sample),
			})
		}
	}
	if t.truncateOrder == TruncateAsc {
		sort.Sort(samples)
	} else {
		sort.Sort(sort.Reverse(samples))
	}
	matrix.Samples = make([]*protos.Matrix_Sample, 0)
	if t.offset >= len(samples) {
		return matrix, nil
	} else if t.offset+t.limit >= len(samples) {
		for idx := t.offset; idx < len(samples); idx++ {
			matrix.Samples = append(matrix.Samples, samples[idx].sample)
		}
	} else {
		for idx := t.offset; idx < t.offset+t.limit; idx++ {
			matrix.Samples = append(matrix.Samples, samples[idx].sample)
		}
	}
	return matrix, nil
}

func (t *Truncate) Category() string {
	return "rank"
}
