package processor

import (
	"context"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	commonerrors "sentioxyz/sentio-core/service/common/errors"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	commonprotos "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/storagesystem"
	"sentioxyz/sentio-core/service/processor/driverjob"
	"sentioxyz/sentio-core/service/processor/models"
	"sentioxyz/sentio-core/service/processor/protos"
	"sentioxyz/sentio-core/service/processor/repository"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func NewService(
	processorRepo repository.ProcessorRepo,
	chainStateRepo repository.ChainStateRepo,
	driverJobManager driverjob.DriverJobManager,
	storageSystem storagesystem.FileStorageSystemInterface,
	processorFactory ProcessorFactory,
	lifecycleHook ProcessorLifecycleHook,
	redisClient *redis.Client,
) *Service {
	s := &Service{
		processorRepo:     processorRepo,
		chainStateRepo:    chainStateRepo,
		driverJobManager:  driverJobManager,
		redisClient:       redisClient,
		FileStorageSystem: storageSystem,
		processorFactory:  processorFactory,
		lifecycleHook:     lifecycleHook,
	}
	return s
}

type Service struct {
	protos.UnimplementedProcessorServiceServer
	protos.UnimplementedProcessorRuntimeServiceServer
	processorRepo     repository.ProcessorRepo
	chainStateRepo    repository.ChainStateRepo
	driverJobManager  driverjob.DriverJobManager
	redisClient       *redis.Client
	FileStorageSystem storagesystem.FileStorageSystemInterface
	processorFactory  ProcessorFactory
	lifecycleHook     ProcessorLifecycleHook
}

func (s *Service) GetProcessor(
	ctx context.Context,
	req *protos.GetProcessorRequest,
) (*protos.GetProcessorResponse, error) {
	if req.ProcessorId == "" {
		return nil, errors.Errorf("empty processor id")
	}
	processor, err := s.processorRepo.GetProcessor(ctx, req.GetProcessorId(), true)
	if err != nil {
		return nil, err
	}
	referencedProcessor, err := s.processorRepo.ResolveReferenceProcessor(ctx, processor)
	if err != nil {
		return nil, err
	}
	pb, err := referencedProcessor.ToPB(referencedProcessor)
	if err != nil {
		return nil, err
	}

	response := protos.GetProcessorResponse{
		Processor: pb,
	}
	return &response, nil
}

func (s *Service) GetProcessorWithProject(
	ctx context.Context,
	req *protos.GetProcessorRequest,
) (*protos.GetProcessorWithProjectResponse, error) {
	if req.ProcessorId == "" {
		return nil, errors.Errorf("empty processor id")
	}
	processor, err := s.processorRepo.GetProcessor(ctx, req.GetProcessorId(), true)
	if err != nil {
		return nil, errors.Wrapf(err, "get processor from db failed")
	}

	referencedProcessor, err := s.processorRepo.ResolveReferenceProcessor(ctx, processor)
	if err != nil {
		return nil, err
	}
	// convert to pb objects
	var response protos.GetProcessorWithProjectResponse

	if response.Processor, err = processor.ToPB(referencedProcessor); err != nil {
		return nil, errors.Wrapf(err, "convert processor model to pb object failed")
	}

	response.Project = processor.Project.ToPB()
	return &response, nil
}

// GetProjectVariables is internal interface, should return secret value
func (s *Service) GetProjectVariables(
	ctx context.Context,
	req *protos.GetProjectVariablesRequest,
) (*commonprotos.ProjectVariables, error) {
	vars, err := s.processorRepo.GetProjectVariables(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	result := &commonprotos.ProjectVariables{
		Variables: make([]*commonprotos.ProjectVariables_Variable, len(vars)),
		ProjectId: req.ProjectId,
	}
	for i, v := range vars {
		result.Variables[i] = v.ToPB(false)
	}
	return result, nil
}

func (s *Service) SetProcessorEntitySchema(ctx context.Context, req *protos.SetProcessorEntitySchemaRequest) (*emptypb.Empty, error) {
	err := s.processorRepo.WithTransaction(ctx, func(ctx context.Context) error {
		processor, err := s.processorRepo.GetProcessor(ctx, req.GetProcessorId(), false)
		if err != nil {
			return err
		}
		if req.GetSchema() == processor.EntitySchema {
			return nil
		}
		processor.EntitySchema = req.GetSchema()
		return s.processorRepo.SaveProcessor(ctx, processor)
	})
	return &emptypb.Empty{}, err
}

func (s *Service) GetProcessors(
	ctx context.Context,
	req *protos.GetProcessorsRequest,
) (*protos.GetProcessorsResponse, error) {
	if req.ProjectId == "" {
		return nil, errors.Errorf("empty project id")
	}
	processors, err := s.processorRepo.GetProcessors(ctx, req.GetProjectId())
	if err != nil {
		return nil, err
	}

	response := protos.GetProcessorsResponse{}
	for _, processor := range processors {
		referenceProcessor, err := s.processorRepo.ResolveReferenceProcessor(ctx, &processor)
		if err != nil {
			return nil, err
		}
		var pb *protos.Processor

		if pb, err = processor.ToPB(referenceProcessor); err != nil {
			return nil, err
		}

		response.Processors = append(response.Processors, pb)
	}
	return &response, nil
}

func (s *Service) UpdateChainProcessorStatus(
	ctx context.Context,
	req *protos.UpdateChainProcessorStatusRequest,
) (*protos.UpdateChainProcessorStatusResponse, error) {
	var cs models.ChainState
	if err := cs.FromPB(req.ChainState, req.Id); err != nil {
		return nil, err
	}
	cs.ProcessorID = req.Id
	err := s.chainStateRepo.UpdateChainState(ctx, &cs)
	if err != nil {
		return nil, err
	}
	return &protos.UpdateChainProcessorStatusResponse{}, nil
}

func (s *Service) GetProcessorStatusV2(
	ctx context.Context,
	req *protos.GetProcessorStatusRequestV2,
) (*protos.GetProcessorStatusResponse, error) {
	project, err := s.processorRepo.PreLoadProject(ctx, req.GetProjectOwner(), req.GetProjectSlug())
	if project == nil || err != nil {
		return nil, status.Error(codes.NotFound, "project not found")
	}

	return s.getProcessorStatus(ctx, project.ID, "", req.GetVersion())
}

func (s *Service) GetProcessorStatus(
	ctx context.Context,
	req *protos.GetProcessorStatusRequest,
) (*protos.GetProcessorStatusResponse, error) {
	return s.GetProcessorStatusInternal(ctx, req)
}

func (s *Service) GetProcessorStatusInternal(
	ctx context.Context,
	req *protos.GetProcessorStatusRequest,
) (*protos.GetProcessorStatusResponse, error) {
	if projectID := req.GetProjectId(); projectID != "" {
		// check if the project exists
		_, err := s.processorRepo.GetProjectByID(ctx, projectID)
		if err != nil {
			return nil, err
		}
		return s.getProcessorStatus(ctx, req.GetProjectId(), "", protos.GetProcessorStatusRequestV2_ALL)
	}
	if processorID := req.GetId(); processorID != "" {
		p, err := s.processorRepo.GetProcessor(ctx, processorID, false)
		if err != nil {
			return nil, err
		}
		return s.getProcessorStatus(ctx, p.ProjectID, processorID, protos.GetProcessorStatusRequestV2_ALL)
	}
	return nil, status.Error(codes.InvalidArgument, "either project_id or id must be provided")
}

func (s *Service) getProcessorStatus(
	ctx context.Context,
	projectID string,
	processorID string,
	versionSelector protos.GetProcessorStatusRequestV2_VersionSelector,
) (*protos.GetProcessorStatusResponse, error) {
	var processors []*models.Processor
	var err error
	switch versionSelector {
	case protos.GetProcessorStatusRequestV2_ACTIVE:
		processors, err = s.processorRepo.GetProcessorsByProjectAndVersionState(ctx, projectID, protos.ProcessorVersionState_ACTIVE)
	case protos.GetProcessorStatusRequestV2_PENDING:
		processors, err = s.processorRepo.GetProcessorsByProjectAndVersionState(ctx, projectID, protos.ProcessorVersionState_PENDING)
	default:
		processors, err = s.processorRepo.GetProcessorsByProjectAndVersionState(ctx, projectID)
	}
	if err != nil {
		return nil, err
	}
	if processorID != "" {
		processors = lo.Filter(processors, func(p *models.Processor, _ int) bool {
			return p.ID == processorID
		})
	}

	response := protos.GetProcessorStatusResponse{
		Processors: make([]*protos.GetProcessorStatusResponse_ProcessorEx, len(processors)),
	}

	for i, processor := range processors {
		originProcessor := processor
		if len(originProcessor.ReferenceProjectID) > 0 && originProcessor.VersionState != int32(protos.ProcessorVersionState_OBSOLETE) {
			rp, err := s.processorRepo.ResolveReferenceProcessor(ctx, originProcessor)
			if err != nil {
				return nil, err
			}
			if rp == nil {
				continue
			}
			processor, err = s.processorRepo.GetProcessor(ctx, rp.ID, true)
			if err != nil {
				return nil, err
			}
		}

		var metaChain *models.ChainState
		var chainStates []*models.ChainState
		for _, state := range processor.ChainStates {
			if state.ChainID == "meta" {
				metaChain = state
				continue
			}
			chainStates = append(chainStates, state)
		}
		processorStatus := &protos.GetProcessorStatusResponse_ProcessorStatus{}
		var states []*protos.ChainState
		if metaChain == nil || (metaChain.State != int32(protos.ChainState_Status_ERROR) &&
			metaChain.State != int32(protos.ChainState_Status_UNKNOWN)) {
			if metaChain == nil {
				processorStatus.State = protos.GetProcessorStatusResponse_ProcessorStatus_STARTING
				if processor.VersionState == int32(protos.ProcessorVersionState_OBSOLETE) {
					processorStatus.State = protos.GetProcessorStatusResponse_ProcessorStatus_UNKNOWN
				}
			} else {
				processorStatus.State = protos.GetProcessorStatusResponse_ProcessorStatus_PROCESSING
			}
			if states, err = utils.MapSlice(chainStates,
				func(t *models.ChainState) (*protos.ChainState, error) {
					return t.ToPB()
				}); err != nil {
				return nil, err
			}
			// If processing, check if any chain is in error state.
			for _, state := range states {
				if state.Status.State == protos.ChainState_Status_ERROR {
					// 1 means processor error. Always show processor error.
					if processorStatus.State == protos.GetProcessorStatusResponse_ProcessorStatus_PROCESSING ||
						state.Status.ErrorRecord.NamespaceCode == 1 {
						processorStatus.State = protos.GetProcessorStatusResponse_ProcessorStatus_ERROR
						processorStatus.ErrorRecord = state.Status.ErrorRecord
					}
				}
			}
		} else {
			processorStatus.State = protos.GetProcessorStatusResponse_ProcessorStatus_ERROR
			processorStatus.ErrorRecord = metaChain.ErrorRecord.ToPB()
			if states, err = utils.MapSlice(chainStates,
				func(t *models.ChainState) (*protos.ChainState, error) {
					t.State = int32(protos.ChainState_Status_ERROR)
					t.ErrorRecord = metaChain.ErrorRecord
					return t.ToPB()
				}); err != nil {
				return nil, err
			}
		}
		var uploadedBy *commonprotos.UserInfo
		if processor.User != nil {
			uploadedBy = processor.User.ToUserInfo()
		}
		// fix processor status by driver job status
		if processorStatus.State != protos.GetProcessorStatusResponse_ProcessorStatus_UNKNOWN &&
			(processorStatus.State != protos.GetProcessorStatusResponse_ProcessorStatus_ERROR ||
				processorStatus.ErrorRecord.GetNamespace() != commonerrors.PROCESSOR) {
			if s.driverJobManager.IsProcessorRunning(processor.ID, int(processor.K8sClusterID)) {
				processorStatus.State = protos.GetProcessorStatusResponse_ProcessorStatus_PROCESSING
			} else {
				processorStatus.State = protos.GetProcessorStatusResponse_ProcessorStatus_STARTING
			}
		}
		// If processor is in error state, mark every chain in error.
		if processorStatus.State == protos.GetProcessorStatusResponse_ProcessorStatus_ERROR {
			for _, state := range states {
				if state.Status.State == protos.ChainState_Status_ERROR {
					continue
				}
				state.Status.State = protos.ChainState_Status_ERROR
			}
		}
		// If processor is starting, every chain state is queuing.
		if processorStatus.State == protos.GetProcessorStatusResponse_ProcessorStatus_STARTING {
			for _, state := range states {
				if state.Status.State == protos.ChainState_Status_ERROR {
					continue
				}
				state.Status.State = protos.ChainState_Status_QUEUING
			}
		}

		networkOverrides := make([]*protos.NetworkOverride, len(processor.NetworkOverrides))
		for i, no := range processor.NetworkOverrides {
			networkOverrides[i] = &protos.NetworkOverride{Chain: no.Chain, Host: no.Host}
		}
		response.Processors[i] = &protos.GetProcessorStatusResponse_ProcessorEx{
			States:           states,
			ProcessorId:      originProcessor.ID,
			CodeHash:         processor.CodeHash,
			CommitSha:        processor.CommitSha,
			UploadedBy:       uploadedBy,
			UploadedAt:       timestamppb.New(processor.UploadedAt),
			ProcessorStatus:  processorStatus,
			Version:          originProcessor.Version,
			CliVersion:       processor.CliVersion,
			SdkVersion:       processor.SdkVersion,
			GitUrl:           processor.GitURL,
			VersionState:     protos.ProcessorVersionState(processor.VersionState),
			VersionLabel:     processor.VersionLabel,
			IpfsHash:         processor.IpfsHash,
			DebugFork:        processor.DebugFork,
			Warnings:         processor.Warnings,
			Pause:            processor.Pause,
			PauseAt:          timestamppb.New(processor.PauseAt),
			PauseReason:      processor.PauseReason,
			NetworkOverrides: models.ParseNetworkOverrides(processor.NetworkOverrides),
			DriverVersion:    strconv.FormatInt(int64(processor.DriverVersion), 10),
			NumWorkers:       strconv.FormatInt(int64(processor.NumWorkers), 10),
		}
		if originProcessor.ID != processor.ID {
			response.Processors[i].ReferenceProjectId = processor.ProjectID
		}
	}

	return &response, nil
}

func (s *Service) RemoveProcessor(
	ctx context.Context,
	req *protos.ProcessorIdRequest,
) (*protos.RemoveProcessorResponse, error) {
	processor := PreloadedProcessor(ctx)
	err := s.processorRepo.WithTransaction(ctx, func(ctx context.Context) error {
		return s.stopProcessor(ctx, req.Id)
	})
	if err != nil {
		return nil, err
	}
	if pb, err := processor.ToPB(nil); err != nil {
		return nil, err
	} else {
		resp := &protos.RemoveProcessorResponse{
			Deleted: pb,
		}
		return resp, nil
	}
}

func (s *Service) stopProcessor(
	ctx context.Context,
	id string,
) error {
	processor, err := s.processorRepo.GetProcessor(ctx, id, false)
	if err != nil {
		return err
	}
	if processor.ProjectID != "" {
		project, err := s.processorRepo.GetProjectByID(ctx, processor.ProjectID)
		if err != nil {
			return err
		}
		processor.Project = project
	}
	logger := log.WithContext(ctx)

	// skip delete job for reference processor
	if processor.ReferenceProjectID == "" {
		err = s.driverJobManager.DeleteJob(ctx, processor)
		if err != nil {
			logger.Errore(err)
			return err
		}
	}

	if err = s.processorRepo.ObsoleteProcessor(ctx, processor.ID); err != nil {
		return err
	}
	if err = s.notifyProcessorStopped(ctx, processor); err != nil {
		return err
	}
	return nil
}

const MaxObsoleteVersion = 10

func (s *Service) RestartProcessor(ctx context.Context, req *protos.GetProcessorRequest) (*emptypb.Empty, error) {
	processor := PreloadedProcessor(ctx)
	if err := s.driverJobManager.RestartProcessorByID(ctx, processor.ID, int(processor.K8sClusterID)); err != nil {
		log.Infofe(err, "restart processor %q.%q.%d failed", processor.ProjectID, processor.ID, processor.Version)
	}
	if err := s.notifyProcessorStopped(ctx, processor); err != nil {
		return nil, err
	}

	if err := s.chainStateRepo.DeleteChainStatesByProcessor(ctx, processor.ID); err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) SetVersionActive(ctx context.Context, req *protos.GetProcessorRequest) (*emptypb.Empty, error) {
	processor := PreloadedProcessor(ctx)

	processor.VersionState = int32(protos.ProcessorVersionState_ACTIVE)
	err := s.processorRepo.WithTransaction(ctx, func(ctx context.Context) error {
		if err := s.activateProcessor(ctx, processor, false); err != nil {
			return err
		}
		return s.processorRepo.SaveProcessor(ctx, processor)
	})
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) ActivatePendingVersion(ctx context.Context, req *protos.ActivatePendingRequest) (*emptypb.Empty, error) {
	project, err := s.processorRepo.PreLoadProject(ctx, req.GetProjectOwner(), req.GetProjectSlug())
	if project == nil || err != nil {
		return nil, status.Error(codes.NotFound, "project not found")
	}
	var processors []*models.Processor
	processors, err = s.processorRepo.GetProcessorsByProjectAndVersionState(ctx, project.ID, protos.ProcessorVersionState_PENDING)
	if err != nil {
		return nil, err
	}
	if len(processors) == 0 {
		return nil, status.Error(codes.NotFound, "no pending processor found")
	}
	// activate the latest pending processor
	latestProcessor := lo.MaxBy(processors, func(a, b *models.Processor) bool {
		return a.Version < b.Version
	})
	err = s.processorRepo.WithTransaction(ctx, func(ctx context.Context) error {
		latestProcessor.VersionState = int32(protos.ProcessorVersionState_ACTIVE)
		if err := s.activateProcessor(ctx, latestProcessor, false); err != nil {
			return err
		}
		return s.processorRepo.SaveProcessor(ctx, latestProcessor)
	})
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) activateProcessor(ctx context.Context, processor *models.Processor, upgrade bool) error {
	var err error
	if processor.VersionState == int32(protos.ProcessorVersionState_PENDING) {
		// stop other pending versions
		pendingProcessors, err := s.processorRepo.GetProcessorsByProjectAndVersionState(
			ctx, processor.ProjectID, protos.ProcessorVersionState_PENDING)
		if err != nil {
			return err
		}
		pendingProcessors = lo.Filter(pendingProcessors, func(p *models.Processor, _ int) bool {
			return p.ID != processor.ID
		})
		for _, p := range pendingProcessors {
			err = s.obsoleteProcessor(ctx, p)
			if err != nil {
				return err
			}
		}
	} else if processor.VersionState == int32(protos.ProcessorVersionState_ACTIVE) {
		// stop other active versions
		activeProcessors, err := s.processorRepo.GetProcessorsByProjectAndVersionState(
			ctx, processor.ProjectID, protos.ProcessorVersionState_ACTIVE)
		if err != nil {
			return err
		}
		activeProcessors = lo.Filter(activeProcessors, func(p *models.Processor, _ int) bool {
			return p.ID != processor.ID
		})
		for _, p := range activeProcessors {
			err = s.obsoleteProcessor(ctx, p)
			if err != nil {
				return err
			}
		}

	}

	// clean up old versions
	obsoleteProcessors, err := s.processorRepo.GetObsoleteProcessors(ctx, processor.ProjectID)
	if err == nil && len(obsoleteProcessors) > MaxObsoleteVersion {
		for _, p := range obsoleteProcessors[MaxObsoleteVersion:] {
			if err = s.processorRepo.RemoveProcessor(ctx, p.ID); err != nil {
				return err
			}
		}
	}

	// start new version
	if processor.VersionState == int32(protos.ProcessorVersionState_PENDING) ||
		processor.VersionState == int32(protos.ProcessorVersionState_ACTIVE) {
		if err = s.driverJobManager.StartOrUpdateDriverJob(ctx, processor); err != nil {
			return err
		}
		if upgrade {
			if err := s.driverJobManager.RestartJob(ctx, processor); err != nil {
				return err
			}
		}
	}
	return s.notifyProcessorActivated(ctx, processor)
}

func (s *Service) obsoleteProcessor(ctx context.Context, p *models.Processor) error {
	p.VersionState = int32(protos.ProcessorVersionState_OBSOLETE)
	p.Pause = false
	err := s.stopProcessor(ctx, p.ID)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) CreateOrUpdateProcessor(
	ctx context.Context,
	identity *commonmodels.Identity,
	project *commonmodels.Project,
	continueFrom int32,
	rollback map[string]uint64, // key may be '*', means all chains should roll back to the block
	numWorkers int32,
	sentioProperties models.SentioProcessorProperties,
	subgraphProperties models.SubgraphProcessorProperties,
) (p *models.Processor, err error) {
	return s.processorFactory.CreateOrUpdateProcessor(
		ctx,
		identity,
		project,
		continueFrom,
		rollback,
		numWorkers,
		sentioProperties,
		subgraphProperties,
		s.activateProcessor,
	)
}

func (s *Service) PauseProcessorInternal(ctx context.Context, req *protos.PauseProcessorRequest) (*emptypb.Empty, error) {
	return s.updateProcessorPause(ctx, req.ProcessorId, true, req.Reason)
}

func (s *Service) PauseProcessor(ctx context.Context, req *protos.PauseProcessorRequest) (*emptypb.Empty, error) {
	return s.updateProcessorPause(ctx, req.ProcessorId, true, req.Reason)
}

func (s *Service) ResumeProcessor(ctx context.Context, req *protos.GetProcessorRequest) (*emptypb.Empty, error) {
	return s.updateProcessorPause(ctx, req.ProcessorId, false, "")
}

func (s *Service) updateProcessorPause(
	ctx context.Context,
	processorID string,
	pause bool,
	reason string,
) (*emptypb.Empty, error) {
	processor, err := s.processorRepo.PreloadProcessor(ctx, processorID)
	if err != nil {
		return nil, err
	}

	if !processor.IsRunningVersion() {
		return nil, status.Error(codes.InvalidArgument, "processor version state is not active or pending")
	}
	if len(processor.ReferenceProjectID) > 0 {
		return nil, status.Error(codes.InvalidArgument, "reference processor can not be paused")
	}
	if processor.Pause == pause {
		return &emptypb.Empty{}, nil
	}
	_, logger := log.FromContext(ctx, "processor_id", processorID)
	if pause {
		processor.PauseAt = time.Now()
		processor.PauseReason = reason
		logger.Infof("Pause now")
	} else {
		logger.Infof("Resume now")
	}
	processor.Pause = pause
	if err := s.processorRepo.SaveProcessor(ctx, processor); err != nil {
		return nil, err
	}
	if err := s.driverJobManager.StartOrUpdateDriverJob(ctx, processor); err != nil {
		log.Errorfe(err, "failed to update driverJob for processor %q", processorID)
		return nil, err
	}
	var hookErr error
	if pause {
		hookErr = s.notifyProcessorPaused(ctx, processor)
	} else {
		hookErr = s.notifyProcessorResumed(ctx, processor)
	}
	if hookErr != nil {
		return nil, hookErr
	}
	return &emptypb.Empty{}, nil
}

func (s *Service) notifyProcessorActivated(ctx context.Context, processor *models.Processor) error {
	if s.lifecycleHook == nil {
		return nil
	}
	return s.lifecycleHook.OnActivate(ctx, processor)
}

func (s *Service) notifyProcessorStopped(ctx context.Context, processor *models.Processor) error {
	if s.lifecycleHook == nil {
		return nil
	}
	return s.lifecycleHook.OnStop(ctx, processor)
}

func (s *Service) notifyProcessorPaused(ctx context.Context, processor *models.Processor) error {
	if s.lifecycleHook == nil {
		return nil
	}
	return s.lifecycleHook.OnPause(ctx, processor)
}

func (s *Service) notifyProcessorResumed(ctx context.Context, processor *models.Processor) error {
	if s.lifecycleHook == nil {
		return nil
	}
	return s.lifecycleHook.OnResume(ctx, processor)
}

func PreloadedProcessor(ctx context.Context) *models.Processor {
	if processor, ok := ctx.Value("processor").(*models.Processor); ok {
		return processor
	}
	return nil
}

func (s *Service) RunProcessor(ctx context.Context, req *protos.RunProcessorRequest) (*protos.Processor, error) {
	var p *models.Processor
	project, err := s.processorRepo.GetProjectByID(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}

	err = s.processorRepo.WithTransaction(ctx, func(ctx context.Context) error {
		p = &models.Processor{
			ID:                  req.ProcessorId,
			UploadedAt:          req.CreatedAt.AsTime(),
			ProjectID:           project.ID,
			Project:             project,
			K8sClusterID:        0,
			EntitySchemaVersion: 0,
			DriverVersion:       1,
			NumWorkers:          req.NumWorkers,
			VersionState:        int32(protos.ProcessorVersionState_ACTIVE),
			SentioProcessorProperties: models.SentioProcessorProperties{
				CliVersion: req.CliVersion,
				SdkVersion: req.SdkVersion,
				CodeURL:    req.CodeUrl,
				Debug:      req.Debug,
				Binary:     req.IsBinary,
			},
		}
		err := s.processorRepo.SaveProcessor(ctx, p)
		if err != nil {
			return err
		}
		if err = s.activateProcessor(ctx, p, false); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return p.ToPB(nil)
}

func (s *Service) StopProcessor(ctx context.Context, req *protos.StopProcessorRequest) (*emptypb.Empty, error) {
	err := s.stopProcessor(ctx, req.ProcessorId)
	return nil, err
}
