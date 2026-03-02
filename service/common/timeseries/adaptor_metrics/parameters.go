package adaptor_metrics

import (
	"sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/selector"
)

type Parameters struct {
	name          string
	alias         string
	id            string
	operator      *protos.Aggregate_AggregateOps
	groups        []string
	labelSelector selector.Selector
	timeRange     *timerange.TimeRange
}

func NewParameters() *Parameters {
	return &Parameters{}
}

func (p *Parameters) SetName(name string) *Parameters {
	p.name = name
	return p
}

func (p *Parameters) SetAlias(alias string) *Parameters {
	p.alias = alias
	return p
}

func (p *Parameters) SetId(id string) *Parameters {
	p.id = id
	return p
}

func (p *Parameters) SetOperator(operator protos.Aggregate_AggregateOps) *Parameters {
	p.operator = &operator
	return p
}

func (p *Parameters) SetGroups(groups []string) *Parameters {
	p.groups = groups
	return p
}

func (p *Parameters) SetLabelSelector(labelSelector selector.Selector) *Parameters {
	p.labelSelector = labelSelector
	return p
}

func (p *Parameters) SetTimeRange(timeRange *timerange.TimeRange) *Parameters {
	p.timeRange = timeRange
	return p
}
