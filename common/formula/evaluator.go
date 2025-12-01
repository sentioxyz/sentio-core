package formula

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"sort"
	"sync"

	"sentioxyz/sentio-core/common/log"

	"sentioxyz/sentio-core/service/common/protos"

	"github.com/pkg/errors"
)

const (
	PositiveOrder = iota
	ReverseOrder
)

var mathOpSet = map[AggregateOp]bool{
	ABS: true,
}

type Value interface {
	CheckSum() string
	Debug() string
	equal(rhs Value) bool
}

// ScalarValue is a constant value
type ScalarValue struct {
	Value float64
}

func (s *ScalarValue) equal(rhs Value) bool {
	switch rhs := rhs.(type) {
	case *ScalarValue:
		return floatEqual(s.Value, rhs.Value)
	default:
		return false
	}
}

func (s *ScalarValue) CheckSum() string {
	return fmt.Sprintf("%f", s.Value)
}

func (s *ScalarValue) Debug() string {
	return fmt.Sprintf("%f", s.Value)
}

func (s *ScalarValue) GetValue() float64 {
	return s.Value
}

// VectorValue is a vector of values
type VectorValue struct {
	sample *protos.Matrix_Sample
}

func NewVectorValueFromSample(sample *protos.Matrix_Sample) *VectorValue {
	return &VectorValue{
		sample: sample,
	}
}

func newVectorValue() *VectorValue {
	return &VectorValue{
		sample: &protos.Matrix_Sample{
			Metric: &protos.Matrix_Metric{
				Labels: map[string]string{},
			},
			Values: []*protos.Matrix_Value{},
		},
	}
}

type kv struct {
	timestamp int64
	value     float64
}

type kvs []kv

func (kvs kvs) Len() int {
	return len(kvs)
}

func (kvs kvs) Less(i, j int) bool {
	return kvs[i].timestamp < kvs[j].timestamp
}

func (kvs kvs) Swap(i, j int) {
	kvs[i], kvs[j] = kvs[j], kvs[i]
}

func copyFromVectorValue(vectors ...*VectorValue) *VectorValue {
	res := newVectorValue()
	for _, rhs := range vectors {
		for idx := range rhs.sample.Values {
			res.sample.Values = append(res.sample.Values, &protos.Matrix_Value{
				Timestamp: rhs.sample.Values[idx].Timestamp,
			})
		}
	}
	if len(vectors) > 1 {
		var kvs kvs
		for _, value := range res.sample.Values {
			kvs = append(kvs, kv{
				timestamp: value.Timestamp,
				value:     value.Value,
			})
		}
		sort.Sort(kvs)
		res.sample.Values = []*protos.Matrix_Value{}
		for idx := range kvs {
			if idx == 0 {
				res.sample.Values = append(res.sample.Values, &protos.Matrix_Value{
					Timestamp: kvs[idx].timestamp,
					Value:     kvs[idx].value,
				})
			} else {
				if kvs[idx].timestamp != kvs[idx-1].timestamp {
					res.sample.Values = append(res.sample.Values, &protos.Matrix_Value{
						Timestamp: kvs[idx].timestamp,
						Value:     kvs[idx].value,
					})
				}
			}
		}
	}
	return res
}

func (v *VectorValue) equal(rhs Value) bool {
	switch rhs := rhs.(type) {
	case *VectorValue:
		if len(v.sample.Values) != len(rhs.sample.Values) {
			return false
		}
		for i, value := range v.sample.Values {
			if !floatEqual(value.Value, rhs.sample.Values[i].Value) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func (v *VectorValue) Debug() string {
	s, _ := json.Marshal(v.sample)
	return string(s)
}

func (v *VectorValue) GetValues() []*protos.Matrix_Sample {
	return []*protos.Matrix_Sample{v.sample}
}

func valuesCheckSum(values []*protos.Matrix_Value) string {
	lens := len(values)
	if lens == 0 {
		return "nil"
	}
	first := values[0]
	last := values[lens-1]
	mid := values[lens/2]
	return fmt.Sprintf("%d-%d-%f-%d-%f-%d-%f", lens,
		first.Timestamp, first.Value,
		mid.Timestamp, mid.Value,
		last.Timestamp, last.Value)
}

func metricsCheckSum(metrics *protos.Matrix_Metric) string {
	if metrics == nil {
		return "<nil>-<nil>"
	}
	return fmt.Sprintf("%s-%s", metrics.Name, metrics.DisplayName)
}

func (v *VectorValue) CheckSum() string {
	return metricsCheckSum(v.sample.Metric) + valuesCheckSum(v.sample.Values)
}

type MatrixValue struct {
	samples []*protos.Matrix_Sample
	labels  []string
	index   *matrixIndex
	mutex   sync.Mutex
}

func NewMatrixValueFromSamples(samples []*protos.Matrix_Sample) *MatrixValue {
	return &MatrixValue{
		samples: samples,
	}
}

func newMatrixValue() *MatrixValue {
	return &MatrixValue{
		samples: []*protos.Matrix_Sample{},
	}
}

func copyFromMatrixValue(rhs *MatrixValue) *MatrixValue {
	res := newMatrixValue()
	for idx := range rhs.samples {
		res.samples = append(res.samples, &protos.Matrix_Sample{
			Metric: &protos.Matrix_Metric{
				Labels: rhs.samples[idx].Metric.Labels,
			},
			Values: []*protos.Matrix_Value{},
		})
		for valueIdx := range rhs.samples[idx].Values {
			res.samples[idx].Values = append(res.samples[idx].Values, &protos.Matrix_Value{
				Timestamp: rhs.samples[idx].Values[valueIdx].Timestamp,
			})
		}
	}
	return res
}

func (m *MatrixValue) equal(rhs Value) bool {
	switch rhs := rhs.(type) {
	case *MatrixValue:
		if len(m.samples) != len(rhs.samples) {
			return false
		}
		for i, sample := range m.samples {
			if len(sample.Values) != len(rhs.samples[i].Values) {
				return false
			}
			for j, value := range sample.Values {
				if !floatEqual(value.Value, rhs.samples[i].Values[j].Value) {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

func (m *MatrixValue) Debug() string {
	s, _ := json.Marshal(m.samples)
	return string(s)
}

func (m *MatrixValue) CheckSum() string {
	var checkSum string
	for _, sample := range m.samples {
		checkSum += metricsCheckSum(sample.Metric)
		checkSum += valuesCheckSum(sample.Values)
	}
	return checkSum
}

func (m *MatrixValue) Labels() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if len(m.labels) > 0 {
		return m.labels
	}

	labels := map[string]bool{}
	for _, sample := range m.samples {
		for k := range sample.Metric.Labels {
			if _, ok := labels[k]; !ok {
				m.labels = append(m.labels, k)
				labels[k] = true
			}
		}
	}
	return m.labels
}

func (m *MatrixValue) buildIndex() *matrixIndex {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.index != nil {
		return m.index
	}

	if len(m.labels) == 0 {
		labels := map[string]bool{}
		for _, sample := range m.samples {
			for k := range sample.Metric.Labels {
				if _, ok := labels[k]; !ok {
					m.labels = append(m.labels, k)
					labels[k] = true
				}
			}
		}
	}
	index := newMatrixIndex()
	for sampleIdx, sample := range m.samples {
		labels := sample.GetMetric().GetLabels()
		var kvs labelKVs
		if len(labels) == len(m.labels) {
			for k, v := range labels {
				kvs = append(kvs, labelKV{key: k, value: v})
			}
		} else {
			for _, label := range m.labels {
				if v, ok := labels[label]; ok {
					kvs = append(kvs, labelKV{key: label, value: v})
				} else {
					kvs = append(kvs, labelKV{key: label, value: ""})
				}
			}
		}
		sort.Sort(kvs)
		labelStrings := make([]string, len(kvs))
		for idx, kv := range kvs {
			labelStrings[idx] = kv.String()
		}
		index.add(sampleIdx, labelStrings...)
	}
	m.index = index
	return index
}

type matrixPrepareContext struct {
	smaller      *MatrixValue
	smallerIndex *matrixIndex
	larger       *MatrixValue
	largerIndex  *matrixIndex
	labelPool    []string
	sequence     int
	result       *MatrixValue
}

func (m *MatrixValue) prepareCalculate(
	rhs *MatrixValue,
) (prepareContext *matrixPrepareContext, supportCalculate bool) {
	if len(m.samples) == 0 || len(rhs.samples) == 0 {
		return nil, false
	}
	prepareContext = &matrixPrepareContext{}

	selfLabels := m.Labels()
	rhsLabels := rhs.Labels()
	if len(selfLabels) < len(rhsLabels) {
		prepareContext.smaller = m
		prepareContext.larger = rhs
		prepareContext.sequence = ReverseOrder
	} else {
		prepareContext.smaller = rhs
		prepareContext.larger = m
		prepareContext.sequence = PositiveOrder
	}
	prepareContext.smallerIndex = prepareContext.smaller.buildIndex()
	prepareContext.largerIndex = prepareContext.larger.buildIndex()

	// check if smaller is subset of larger, just check the first sample in matrix
	for _, key := range prepareContext.smaller.Labels() {
		prepareContext.labelPool = append(prepareContext.labelPool, key)
		if !prepareContext.larger.checkLabelExists(key) {
			return nil, false
		}
	}

	// merge two matrix labels union set
	prepareContext.result = copyFromMatrixValue(prepareContext.larger)
	for _, sample := range prepareContext.smaller.samples {
		var foundEqual = false
		for _, resultSample := range prepareContext.result.samples {
			var equal = true
			for k, v := range sample.Metric.Labels {
				if resultSample.Metric.Labels[k] != v {
					equal = false
					break
				}
			}
			if equal {
				foundEqual = true
				break
			}
		}
		if !foundEqual {
			prepareContext.result.samples = append(prepareContext.result.samples, &protos.Matrix_Sample{
				Metric: &protos.Matrix_Metric{
					Labels: sample.Metric.Labels,
				},
				Values: []*protos.Matrix_Value{},
			})
		}
	}
	return prepareContext, true
}

func (m *MatrixValue) checkLabelExists(key string) bool {
	labels := m.Labels()
	for _, l := range labels {
		if l == key {
			return true
		}
	}
	return false
}

func (m *MatrixValue) setSampleValue(values []*protos.Matrix_Value, labels map[string]string) {
	for _, s := range m.samples {
		if len(s.Metric.Labels) != len(labels) {
			continue
		}
		if !reflect.DeepEqual(s.Metric.Labels, labels) {
			continue
		}
		s.Values = values
		return
	}
}

func (m *MatrixValue) GetValues() []*protos.Matrix_Sample {
	return m.samples
}

type Context struct {
	Values        map[string]Value
	LabelMaxCount int
}

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func Evaluate(ctx Context, expression Expression) (result Value, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("panic when evaluating expression %s: %v", expression.ToString(), r)
			result = nil
			err = errors.Errorf("failed to evaluate expression %s", expression.ToString())
		}
	}()

	switch expr := expression.(type) {
	case *Constant:
		return &ScalarValue{Value: expr.Value}, nil
	case *Identifier:
		value := ctx.Values[expr.Name]
		if value == nil {
			return nil, errors.Errorf("unknown identifier %s", expr.Name)
		}
		return value, nil
	case *BinaryExpression:
		left, err := Evaluate(ctx, expr.Left)
		if err != nil {
			return nil, err
		}
		right, err := Evaluate(ctx, expr.Right)
		if err != nil {
			return nil, err
		}
		return evaluateBinaryExpression(ctx, left, right, expr.Op)
	case *BracketExpression:
		return Evaluate(ctx, expr.Expr)
	case *AggregateExpression:
		value, err := Evaluate(ctx, expr.Expr)
		if err != nil {
			return nil, err
		}
		return evaluateAggregationExpression(ctx, value, expr.Op)
	}
	return nil, errors.Errorf("Unknown expression type %s", expression)
}

func GetIdentifierNames(expression Expression) (identifiers []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Warnf("panic when getting identifier names %s: %v", expression.ToString(), r)
		}
	}()
	switch expr := expression.(type) {
	case *Constant:
		return []string{}
	case *Identifier:
		return []string{expr.Name}
	case *BinaryExpression:
		identifiers = append(identifiers, GetIdentifierNames(expr.Left)...)
		identifiers = append(identifiers, GetIdentifierNames(expr.Right)...)
		return identifiers
	case *BracketExpression:
		return GetIdentifierNames(expr.Expr)
	case *AggregateExpression:
		return GetIdentifierNames(expr.Expr)
	}
	return []string{}
}

func evaluateVectorAggregate(value *VectorValue, op AggregateOp) Value {
	var aggrResult float64
	switch op {
	case SUM:
		aggrResult = 0
		for _, v := range value.sample.Values {
			aggrResult += v.Value
		}
		return &ScalarValue{Value: aggrResult}
	case MIN:
		aggrResult = math.MaxFloat64
		for _, v := range value.sample.Values {
			aggrResult = math.Min(aggrResult, v.Value)
		}
		return &ScalarValue{Value: aggrResult}
	case MAX:
		aggrResult = -math.MaxFloat64
		for _, v := range value.sample.Values {
			aggrResult = math.Max(aggrResult, v.Value)
		}
		return &ScalarValue{Value: aggrResult}
	case AVG:
		aggrResult = 0
		for _, v := range value.sample.Values {
			aggrResult += v.Value
		}
		aggrResult /= float64(len(value.sample.Values))
		return &ScalarValue{Value: aggrResult}
	case ABS:
		for _, v := range value.sample.Values {
			v.Value = math.Abs(v.Value)
		}
		return value
	default:
		return value
	}
}

func evaluateMatrixAggregate(value *MatrixValue, op AggregateOp) Value {
	if len(value.samples) == 0 {
		return &ScalarValue{Value: 0}
	}
	var tsMap = make(map[int64][]float64)
	for _, sample := range value.samples {
		for _, v := range sample.Values {
			tsMap[v.Timestamp] = append(tsMap[v.Timestamp], v.Value)
		}
	}
	res := newVectorValue()
	var ts []int64
	for k := range tsMap {
		ts = append(ts, k)
	}
	sort.Slice(ts, func(i, j int) bool {
		return ts[i] < ts[j]
	})
	for _, t := range ts {
		if values, ok := tsMap[t]; ok {
			var aggrResult float64
			switch op {
			case SUM:
				aggrResult = 0
				for _, v := range values {
					aggrResult += v
				}
			case MIN:
				aggrResult = math.MaxFloat64
				for _, v := range values {
					aggrResult = math.Min(aggrResult, v)
				}
			case MAX:
				aggrResult = -math.MaxFloat64
				for _, v := range values {
					aggrResult = math.Max(aggrResult, v)
				}
			case AVG:
				aggrResult = 0
				for _, v := range values {
					aggrResult += v
				}
				aggrResult /= float64(len(values))
			}
			res.sample.Values = append(res.sample.Values, &protos.Matrix_Value{
				Timestamp: t,
				Value:     aggrResult,
			})
		} else {
			res.sample.Values = append(res.sample.Values, &protos.Matrix_Value{
				Timestamp: t,
				Value:     0,
			})
		}
	}
	return res
}

func evaluateMatrixMathOp(value *MatrixValue, op AggregateOp) Value {
	for _, sample := range value.samples {
		switch op {
		case ABS:
			for _, v := range sample.Values {
				v.Value = math.Abs(v.Value)
			}
		}
	}
	return value
}

func evaluateAggregationExpression(ctx Context, value Value, op AggregateOp) (Value, error) {
	switch value := value.(type) {
	case *ScalarValue:
		if mathOpSet[op] {
			switch op {
			case ABS:
				return &ScalarValue{Value: math.Abs(value.Value)}, nil
			}
		}
		return value, nil
	case *VectorValue:
		return evaluateVectorAggregate(value, op), nil
	case *MatrixValue:
		if mathOpSet[op] {
			return evaluateMatrixMathOp(value, op), nil
		}
		return evaluateMatrixAggregate(value, op), nil
	}
	return nil, nil
}

func evaluateScalarScalar(_ Context, left, right *ScalarValue, op BinaryOp) Value {
	res := evaluateBinary(left.Value, right.Value, op)
	return &ScalarValue{Value: res}
}

func evaluateScalarVector(_ Context, left *ScalarValue, right *VectorValue, op BinaryOp) Value {
	res := copyFromVectorValue(right)
	for idx, sampleValue := range right.sample.Values {
		res.sample.Values[idx].Value = evaluateBinary(left.Value, sampleValue.Value, op)
	}
	return res
}

func evaluateScalarMatrix(_ Context, left *ScalarValue, right *MatrixValue, op BinaryOp) Value {
	res := copyFromMatrixValue(right)
	for idx := range right.samples {
		for valueIdx, sampleValue := range right.samples[idx].Values {
			res.samples[idx].Values[valueIdx].Value = evaluateBinary(left.Value, sampleValue.Value, op)
		}
	}
	return res
}

func evaluateVectorScalar(_ Context, left *VectorValue, right *ScalarValue, op BinaryOp) Value {
	res := copyFromVectorValue(left)
	for idx, sampleValue := range left.sample.Values {
		res.sample.Values[idx].Value = evaluateBinary(sampleValue.Value, right.Value, op)
	}
	return res
}

func evaluateVectorVector(_ Context, left, right *VectorValue, op BinaryOp) Value {
	var res *VectorValue
	var smaller, larger *VectorValue
	var sequence int
	if len(left.sample.Values) >= len(right.sample.Values) {
		larger = left
		smaller = right
		sequence = PositiveOrder
	} else {
		larger = right
		smaller = left
		sequence = ReverseOrder
	}
	if len(smaller.sample.Values) == 0 {
		res = copyFromVectorValue(larger)
		res.sample.Values = []*protos.Matrix_Value{}
	}
	res = copyFromVectorValue(larger, smaller)
	var smallerIdx, largerIdx = 0, 0
	for idx := range res.sample.Values {
		t := res.sample.Values[idx].Timestamp
		var l, r float64
		for smallerIdx < len(smaller.sample.Values) && smaller.sample.Values[smallerIdx].Timestamp < t {
			smallerIdx++
		}
		for largerIdx < len(larger.sample.Values) && larger.sample.Values[largerIdx].Timestamp < t {
			largerIdx++
		}
		if smallerIdx == len(smaller.sample.Values) {
			r = 0
		} else {
			if smaller.sample.Values[smallerIdx].Timestamp == t {
				r = smaller.sample.Values[smallerIdx].Value
			} else if smallerIdx == 0 {
				r = 0
			} else {
				r = smaller.sample.Values[smallerIdx-1].Value
			}
		}
		if largerIdx == len(larger.sample.Values) {
			l = 0
		} else {
			if larger.sample.Values[largerIdx].Timestamp == t {
				l = larger.sample.Values[largerIdx].Value
			} else if largerIdx == 0 {
				l = 0
			} else {
				l = larger.sample.Values[largerIdx-1].Value
			}
		}
		if sequence == PositiveOrder {
			res.sample.Values[idx].Value = evaluateBinary(l, r, op)
		} else {
			res.sample.Values[idx].Value = evaluateBinary(r, l, op)
		}
	}
	return res
}

func evaluateVectorMatrix(ctx Context, left *VectorValue, right *MatrixValue, op BinaryOp) Value {
	res := copyFromMatrixValue(right)
	for idx := range right.samples {
		sample := right.samples[idx]
		res.samples[idx].Values = evaluateVectorVector(ctx,
			left, &VectorValue{sample: sample}, op).(*VectorValue).sample.Values
	}
	return res
}

func evaluateMatrixScalar(_ Context, left *MatrixValue, right *ScalarValue, op BinaryOp) Value {
	res := copyFromMatrixValue(left)
	for idx := range left.samples {
		for valueIdx, sampleValue := range left.samples[idx].Values {
			res.samples[idx].Values[valueIdx].Value = evaluateBinary(sampleValue.Value, right.Value, op)
		}
	}
	return res
}

func evaluateMatrixVector(ctx Context, left *MatrixValue, right *VectorValue, op BinaryOp) Value {
	res := copyFromMatrixValue(left)
	for idx := range left.samples {
		sample := left.samples[idx]
		res.samples[idx].Values = evaluateVectorVector(ctx,
			&VectorValue{sample: sample}, right, op).(*VectorValue).sample.Values
	}
	return res
}

func getLargerLabels(left, right map[string]string) map[string]string {
	if len(left) >= len(right) {
		return left
	}
	return right
}

func getSmallerLabels(left, right map[string]string) map[string]string {
	if len(left) < len(right) {
		return left
	}
	return right
}

func getMatchedSampleIdx(v *VectorValue, labelPool []string, index *matrixIndex) int {
	var kvs labelKVs
	var labels []string
	for _, label := range labelPool {
		kvs = append(kvs, labelKV{key: label, value: v.sample.Metric.Labels[label]})
	}
	sort.Sort(kvs)
	for _, kv := range kvs {
		labels = append(labels, labelString(kv.key, kv.value))
	}
	return index.get(labels...)
}

func evaluateMatrixMatrix(ctx Context, left, right *MatrixValue, op BinaryOp) Value {
	prepareContext, support := left.prepareCalculate(right)
	if !support {
		return copyFromMatrixValue(left)
	}
	for idx := range prepareContext.result.samples {
		var sample *VectorValue
		if idx < len(prepareContext.larger.samples) {
			sample = &VectorValue{sample: prepareContext.larger.samples[idx]}
		} else {
			sample = &VectorValue{sample: prepareContext.result.samples[idx]}
		}
		if idx := getMatchedSampleIdx(sample, prepareContext.labelPool, prepareContext.smallerIndex); idx != -1 {
			rhsSample := &VectorValue{sample: prepareContext.smaller.samples[idx]}
			if prepareContext.sequence == PositiveOrder {
				prepareContext.result.setSampleValue(
					evaluateVectorVector(ctx, sample, rhsSample, op).(*VectorValue).sample.Values,
					getLargerLabels(sample.sample.Metric.Labels, rhsSample.sample.Metric.Labels),
				)
			} else {
				prepareContext.result.setSampleValue(evaluateVectorVector(ctx, rhsSample, sample, op).(*VectorValue).sample.Values,
					getLargerLabels(sample.sample.Metric.Labels, rhsSample.sample.Metric.Labels))
			}
		} else {
			if prepareContext.sequence == PositiveOrder {
				prepareContext.result.setSampleValue(evaluateVectorScalar(ctx, sample, &ScalarValue{Value: 0},
					op).(*VectorValue).sample.Values,
					sample.sample.Metric.Labels)
			} else {
				prepareContext.result.setSampleValue(evaluateScalarVector(ctx, &ScalarValue{Value: 0}, sample,
					op).(*VectorValue).sample.Values,
					sample.sample.Metric.Labels)
			}
		}
	}
	return prepareContext.result
}

func evaluateBinaryExpression(ctx Context, left Value, right Value, op BinaryOp) (Value, error) {
	switch vLeft := left.(type) {
	case *ScalarValue:
		switch vRight := right.(type) {
		case *ScalarValue:
			return evaluateScalarScalar(ctx, vLeft, vRight, op), nil
		case *VectorValue:
			return evaluateScalarVector(ctx, vLeft, vRight, op), nil
		case *MatrixValue:
			return evaluateScalarMatrix(ctx, vLeft, vRight, op), nil
		}
	case *VectorValue:
		switch vRight := right.(type) {
		case *ScalarValue:
			return evaluateVectorScalar(ctx, vLeft, vRight, op), nil
		case *VectorValue:
			return evaluateVectorVector(ctx, vLeft, vRight, op), nil
		case *MatrixValue:
			return evaluateVectorMatrix(ctx, vLeft, vRight, op), nil
		}
	case *MatrixValue:
		switch vRight := right.(type) {
		case *ScalarValue:
			return evaluateMatrixScalar(ctx, vLeft, vRight, op), nil
		case *VectorValue:
			return evaluateMatrixVector(ctx, vLeft, vRight, op), nil
		case *MatrixValue:
			return evaluateMatrixMatrix(ctx, vLeft, vRight, op), nil
		}
	}
	return nil, errors.Errorf("unknown expression type, left=%s, right=%s", left, right)
}

func evaluateBinary(left float64, right float64, op BinaryOp) float64 {
	switch op {
	case PLUS:
		return left + right
	case MINUS:
		return left - right
	case MUL:
		return left * right
	case DIV:
		if floatEqual(right, 0) {
			log.Debugf("divide by zero, left=%f, right=%f", left, right)
			return 0
		}
		return left / right
	case POW:
		return math.Pow(left, right)
	}
	log.Warnf("unknown binary operator %s", op)
	return 0
}
