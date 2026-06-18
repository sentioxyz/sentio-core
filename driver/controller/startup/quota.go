package startup

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/processor/models"
	"sentioxyz/sentio-core/service/usage/protos"
)

type quotaService struct {
	chainID   string
	processor *models.Processor
	cli       protos.UsageServiceClient
}

func newQuotaService(chainID string, processor *models.Processor, cli protos.UsageServiceClient) *quotaService {
	return &quotaService{chainID: chainID, processor: processor, cli: cli}
}

func (s *quotaService) CheckOverQuota(ctx context.Context) (*controller.OverQuota, error) {
	req := protos.CheckOverLimitRequest{
		ProjectId:   s.processor.ProjectID,
		ProcessorId: &s.processor.ID,
		Now:         timestamppb.Now(),
		Sku:         "metric", // We can use any SKU here, since we only care about the overall quota.
	}
	resp, err := s.cli.CheckOverLimit(ctx, &req)
	if err != nil {
		return nil, errors.Wrapf(err, "check over quota failed")
	}
	if len(resp.GetOver()) == 0 {
		return nil, nil
	}
	return &controller.OverQuota{
		Msg:    strings.Join(resp.Over, "\n"),
		Detail: strings.Join(resp.OverDetail, "\n"),
	}, nil
}

func (s *quotaService) SaveUsage(ctx context.Context, used controller.Usage, inWatching bool) error {
	var version = fmt.Sprintf("%d", s.processor.Version)
	type record struct {
		sku   string
		count int
		tags  map[string]string
	}
	var records []record
	// metric v3
	for _, metricType := range []timeseries.MetaType{timeseries.MetaTypeCounter, timeseries.MetaTypeGauge} {
		for name, count := range used.TimeSeries[metricType] {
			records = append(records, record{
				sku:   "metricv3",
				count: count,
				tags:  map[string]string{"name": name, "version": version, "type": string(metricType)},
			})
			controller.N.DataSaved(ctx, s.processor, s.chainID, "metricV3", string(metricType), name, int64(count))
		}
	}
	// event v3
	for name, count := range used.TimeSeries[timeseries.MetaTypeEvent] {
		records = append(records, record{
			sku:   "eventv3",
			count: count,
			tags:  map[string]string{"name": name, "version": version},
		})
		controller.N.DataSaved(ctx, s.processor, s.chainID, "eventV3", "", name, int64(count))
	}
	// webhook
	for name, count := range used.Export {
		records = append(records, record{
			sku:   "webhook",
			count: count,
			tags:  map[string]string{"name": name, "version": version},
		})
	}
	// entity
	for name, count := range used.EntityCreated {
		records = append(records, record{
			sku:   "entity_created",
			count: count,
			tags:  map[string]string{"name": name, "version": version},
		})
		controller.N.DataSaved(ctx, s.processor, s.chainID, "entity", "created", name, int64(count))
	}
	for name, count := range utils.MergeMapSum(used.EntityCreated, used.EntityUpdated) {
		records = append(records, record{
			sku:   "entity",
			count: count,
			tags:  map[string]string{"name": name, "version": version},
		})
	}
	for name, count := range used.EntityUpdated {
		controller.N.DataSaved(ctx, s.processor, s.chainID, "entity", "updated", name, int64(count))
	}

	// build request and async save
	var req protos.AsyncSaveRequest
	for _, r := range records {
		if r.count == 0 {
			continue
		}
		req.Dialogues = append(req.Dialogues, &protos.Dialogue{
			RequestTime: timestamppb.Now(),
			Succeed:     true,
			Units:       uint64(r.count),
			Tags: &protos.Tags{
				ProjectId:   s.processor.ProjectID,
				ProcessorId: &s.processor.ID,
				Sku:         utils.Select(inWatching, r.sku, r.sku+"_backfill"),
				CustomTags:  r.tags,
			},
		})
	}
	if _, err := s.cli.AsyncSave(ctx, &req); err != nil {
		_, logger := log.FromContext(ctx, "records", records)
		logger.Warnfe(err, "save usage failed")
	}
	return nil
}
