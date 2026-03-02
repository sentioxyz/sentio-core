package converter

import (
	"sentioxyz/sentio-core/service/common/protos"
	protoso11y "sentioxyz/sentio-core/service/observability/protos"
)

type Metric interface {
	Hash() string
	ToProto() *protos.Matrix_Metric
	ToO11yProto() *protoso11y.MetricsQueryResponse_Metric
}

type metric struct {
	name        string
	displayName string
	label       Label
}

func NewMetric(name string, displayName string, label Label) Metric {
	return &metric{
		name:        name,
		displayName: displayName,
		label:       label,
	}
}

func (m *metric) Hash() string {
	if m.label == nil {
		return ""
	}
	return m.label.Hash()
}

func (m *metric) ToProto() *protos.Matrix_Metric {
	return &protos.Matrix_Metric{
		Name:        m.name,
		DisplayName: m.displayName,
		Labels:      m.label.ToProto(),
	}
}

func (m *metric) ToO11yProto() *protoso11y.MetricsQueryResponse_Metric {
	return &protoso11y.MetricsQueryResponse_Metric{
		Name:        m.name,
		DisplayName: m.displayName,
		Labels:      m.label.ToProto(),
	}
}
