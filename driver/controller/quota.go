package controller

import (
	"context"
	"fmt"
	"strings"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
)

type OverQuota struct {
	Msg    string
	Detail string
}

type Usage struct {
	TimeSeries    map[timeseries.MetaType]map[string]int
	Export        map[string]int
	EntityCreated map[string]int
	EntityUpdated map[string]int
}

func (u Usage) String() string {
	var parts []string
	metric := utils.SumMap(u.TimeSeries[timeseries.MetaTypeCounter]) +
		utils.SumMap(u.TimeSeries[timeseries.MetaTypeGauge])
	if metric > 0 {
		parts = append(parts, fmt.Sprintf("%d metric points", metric))
	}
	if event := utils.SumMap(u.TimeSeries[timeseries.MetaTypeEvent]); event > 0 {
		parts = append(parts, fmt.Sprintf("%d events", event))
	}
	if entity := utils.SumMap(u.EntityCreated) + utils.SumMap(u.EntityUpdated); entity > 0 {
		parts = append(parts, fmt.Sprintf("%d entity upserts", entity))
	}
	if export := utils.SumMap(u.Export); export > 0 {
		parts = append(parts, fmt.Sprintf("%d export messages", export))
	}
	if len(parts) == 0 {
		return "0 data"
	}
	return strings.Join(parts, " ")
}

type QuotaService interface {
	CheckOverQuota(ctx context.Context) (*OverQuota, error)
	SaveUsage(ctx context.Context, usage Usage, inWatching bool) error
}

type EmptyQuotaService struct {
}

func (u EmptyQuotaService) CheckOverQuota(ctx context.Context) (*OverQuota, error) {
	return nil, nil
}

func (u EmptyQuotaService) SaveUsage(ctx context.Context, usage Usage, inWatching bool) error {
	return nil
}
