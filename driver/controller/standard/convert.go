package standard

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"

	"sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/processor/protos"
)

func (b *task) ConvertTimeSeriesData(data []*protos.TimeseriesResult) (
	[]timeseries.Dataset,
	*controller.ExternalError,
) {
	dss, err := timeseries.Convert(
		b.chainID,
		b.GetBlockNumber(),
		b.GetBlockHash(),
		b.GetBlockTime(),
		b.metricConfigs,
		data)
	if err == nil {
		return dss, nil
	}
	if errors.Is(err, timeseries.ErrInvalidMetaDiff) {
		return nil, controller.NewExternalError(controller.ErrCodeTimeSeriesDataSchemaChanged, err)
	}
	if errors.Is(err, timeseries.ErrInvalidMeta) {
		return nil, controller.NewExternalError(controller.ErrCodeInvalidTimeSeriesData, err)
	}
	// unreachable
	panic(err)
}

func (b *task) ConvertExportData(r []*protos.ExportResult) []controller.WebhookMessage {
	result := make([]controller.WebhookMessage, len(r))
	for i, er := range r {
		result[i] = controller.WebhookMessage{
			Name:      er.GetMetadata().GetName(),
			BlockTime: b.GetBlockTime(),
			Channel:   b.webhookChannels[er.GetMetadata().GetName()],
			Payload:   er.GetPayload(),
		}
	}
	return result
}

func ConvertTemplateInstance(r []*protos.TemplateInstance, remove bool) []controller.TemplateInstance {
	return utils.MapSliceNoError(r, func(t *protos.TemplateInstance) controller.TemplateInstance {
		var labels []byte
		if len(t.GetBaseLabels().GetFields()) > 0 {
			if raw, err := protojson.Marshal(t.GetBaseLabels()); err == nil {
				var buf bytes.Buffer
				if json.Compact(&buf, raw) == nil {
					labels = buf.Bytes()
				}
			}
		}
		return controller.TemplateInstance{
			TemplateID:   t.GetTemplateId(),
			TemplateName: t.GetContract().GetName(),
			Address:      t.GetContract().GetAddress(),
			BlockRange: controller.BlockRange{
				StartBlock: t.GetStartBlock(),
				EndBlock:   utils.Select(t.GetEndBlock() == 0, nil, utils.WrapPointer(t.GetEndBlock())),
			},
			Labels:  string(labels),
			Removed: remove,
		}
	})
}

func ConvertTemplateInstanceBack(
	chainID string,
	templates map[uint64][]controller.TemplateInstance,
) []*protos.TemplateInstance {
	dict := make(map[string][]controller.TemplateInstance)
	for _, bn := range utils.GetOrderedMapKeys(templates) {
		for _, tpl := range templates[bn] {
			dict[tpl.UniqID()] = append(dict[tpl.UniqID()], tpl)
		}
	}
	var result []controller.TemplateInstance
	for _, tpls := range dict {
		on := controller.EmptyBlockRangeSet
		for _, tpl := range tpls {
			if tpl.Removed {
				on = on.Remove(tpl.BlockRange)
			} else {
				on = on.Union(tpl.BlockRange)
			}
		}
		if on.IsEmpty() {
			continue
		}
		result = append(result, controller.TemplateInstance{
			TemplateID:   tpls[0].TemplateID,
			TemplateName: tpls[0].TemplateName,
			Address:      tpls[0].Address,
			Labels:       tpls[0].Labels,
			BlockRange:   on.Last(),
		})
	}
	// sort by (StartBlock, EndBlock, TemplateID, Address, Labels) ASC, result always stable
	sort.Slice(result, func(i, j int) bool {
		if result[i].BlockRange.StartBlock != result[j].BlockRange.StartBlock {
			return result[i].BlockRange.StartBlock < result[j].BlockRange.StartBlock
		}
		if controller.EqualNilAsInf(result[i].BlockRange.EndBlock, result[j].BlockRange.EndBlock) {
			return controller.LessNilAsInf(result[i].BlockRange.EndBlock, result[j].BlockRange.EndBlock)
		}
		if result[i].TemplateID != result[j].TemplateID {
			return result[i].TemplateID < result[j].TemplateID
		}
		if result[i].Address != result[j].Address {
			return result[i].Address < result[j].Address
		}
		return result[i].Labels < result[j].Labels
	})
	return utils.MapSliceNoError(result, func(t controller.TemplateInstance) *protos.TemplateInstance {
		var labels *structpb.Struct
		if t.Labels != "" {
			_ = json.Unmarshal([]byte(t.Labels), &labels)
		}
		return &protos.TemplateInstance{
			Contract:   &protos.ContractInfo{Name: t.TemplateName, ChainId: chainID, Address: t.Address},
			StartBlock: t.StartBlock,
			EndBlock:   t.EndOrZero(),
			TemplateId: t.TemplateID,
			BaseLabels: labels,
		}
	})
}
