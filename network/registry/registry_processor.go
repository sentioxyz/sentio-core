package registry

import (
	"context"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/statemirror"
	"sentioxyz/sentio-core/network/state"

	"github.com/go-faster/errors"
)

type processorRegistry struct {
	mirror                    statemirror.Mirror
	processorAllocationMirror statemirror.MirrorReadOnlyState[string, []state.ProcessorAllocation]
	processorInfoMirror       statemirror.MirrorReadOnlyState[string, state.ProcessorInfo]
}

func NewProcessorRegistry(mirror statemirror.Mirror) ProcessorRegistry {
	if mirror == nil {
		return &processorRegistry{}
	}
	return &processorRegistry{
		mirror: mirror,
		processorAllocationMirror: statemirror.NewTypedMirror(mirror, statemirror.MappingProcessorAllocations, statemirror.JSONCodec[string, []state.ProcessorAllocation]{
			FieldFunc: func(k string) (string, error) {
				return k, nil
			},
			ParseFunc: func(s string) (string, error) {
				return s, nil
			},
		}),
		processorInfoMirror: statemirror.NewTypedMirror(mirror, statemirror.MappingProcessorInfos, statemirror.JSONCodec[string, state.ProcessorInfo]{
			FieldFunc: func(k string) (string, error) {
				return k, nil
			},
			ParseFunc: func(s string) (string, error) {
				return s, nil
			},
		}),
	}
}

func (r *processorRegistry) RetrieveProcessorAllocations(ctx context.Context, processorId ProcessorId) ([]state.ProcessorAllocation, error) {
	if r.mirror == nil {
		return nil, errors.New("processor allocation state is not initialized")
	}
	ctx, logger := log.FromContext(ctx)
	pa, ok, err := r.processorAllocationMirror.Get(ctx, string(processorId))
	if err != nil {
		logger.Errorf("failed to get processor allocation: %s", err.Error())
		return nil, errors.Wrap(err, "failed to get processor allocation")
	}
	if !ok {
		logger.Errorf("processor allocation not found for processor %s", processorId)
		return nil, errors.Errorf("processor allocation not found for processor %s", processorId)
	}
	return pa, nil
}

func (r *processorRegistry) RetrieveProcessorInfo(ctx context.Context, processorId ProcessorId) (state.ProcessorInfo, error) {
	if r.mirror == nil {
		return state.ProcessorInfo{}, errors.New("processor info state is not initialized")
	}
	ctx, logger := log.FromContext(ctx)
	pi, ok, err := r.processorInfoMirror.Get(ctx, string(processorId))
	if err != nil {
		logger.Errorf("failed to get processor info: %s", err.Error())
		return state.ProcessorInfo{}, errors.Wrap(err, "failed to get processor info")
	}
	if !ok {
		logger.Errorf("processor info not found for processor %s", processorId)
		return state.ProcessorInfo{}, errors.Errorf("processor info not found for processor %s", processorId)
	}
	return pi, nil
}
