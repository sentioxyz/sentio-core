package processor

import (
	"context"
	"fmt"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/processor"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/common/preloader"
	corestorage "sentioxyz/sentio-core/service/common/storagesystem"
	"sentioxyz/sentio-core/service/processor/models"
	"sentioxyz/sentio-core/service/processor/protos"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) InitUpload(
	ctx context.Context,
	req *protos.InitUploadRequest,
) (*protos.InitUploadResponse, error) {
	logger := log.WithContext(ctx)
	var project *commonmodels.Project
	var err error
	owner, slug, ok := strings.Cut(req.ProjectSlug, "/")
	if ok {
		// if project owner&slug is provided, load the project by slug
		project, err = s.processorRepo.PreLoadProject(ctx, owner, slug)
	} else {
		// otherwise,project preloader load the project by slug and owner by from identity
		project = preloader.PreLoadedProject(ctx)
	}
	if err != nil || project == nil {
		return nil, status.Errorf(codes.NotFound, "Project not found: %v", err)
	}
	var warning string
	if project.Type == commonmodels.ProjectTypeSubgraph {
		if req.Sequence != 2 {
			return nil, fmt.Errorf("invalid sequence: %d", req.Sequence)
		}
	} else {
		if req.Sequence > 1 {
			return nil, fmt.Errorf("invalid sequence: %d", req.Sequence)
		}

		clientVersion := req.SdkVersion
		serverVersion := processor.HostProcessorVersion()
		err := processor.VersionCheck(serverVersion, clientVersion)
		if err != nil {
			warning = err.Error()
		}
	}

	key := fmt.Sprintf("processor-upload-%s-%d", project.ID, req.Sequence)
	fileID, err := s.generateFileID()
	if err != nil {
		logger.Errorf("generate file id failed, err=%v", err)
		return nil, err
	}
	ret := s.redisClient.Set(ctx, key, fileID, 10*time.Minute)
	if err := ret.Err(); err != nil {
		logger.Errorf("redis set failed, err=%v", err)
		return nil, err
	}

	contentType := "application/octet-stream"
	if req.ContentType != "" {
		contentType = req.ContentType
	}
	file, err := s.FileStorageSystem.NewUploadFile(ctx, fileID, contentType)
	if err != nil {
		return nil, err
	}
	url, err := file.PreSignedUploadUrl(ctx, 5*time.Minute)
	if err != nil {
		logger.Errorf("sign url failed, err=%v", err)
		return nil, err
	}

	var replacingVersion int32
	replacingProcessor, _ := s.processorRepo.FindReplacingProcessor(ctx, project)
	if replacingProcessor != nil {
		replacingVersion = replacingProcessor.Version
	}
	return &protos.InitUploadResponse{
		Url:              url,
		Warning:          warning,
		ReplacingVersion: replacingVersion,
		MultiVersion:     project.MultiVersion,
		ProjectId:        project.ID,
	}, nil
}

func (s *Service) FinishUpload(
	ctx context.Context,
	req *protos.FinishUploadRequest,
) (*protos.FinishUploadResponse, error) {
	logger := log.WithContext(ctx)
	owner, slug, ok := strings.Cut(req.ProjectSlug, "/")
	var project *commonmodels.Project
	var err error
	if ok {
		// if project owner&slug is provided, load the project by slug
		project, err = s.processorRepo.PreLoadProject(ctx, owner, slug)
	} else {
		// otherwise,project preloader load the project by slug and owner by from identity
		project = preloader.PreLoadedProject(ctx)
	}
	if err != nil || project == nil {
		return nil, status.Errorf(codes.NotFound, "Project not found: %v", err)
	}
	identity := preloader.PreLoadedIdentity(ctx)
	renameFile := func(seq int32) (*corestorage.FileObject, error) {
		key := fmt.Sprintf("processor-upload-%s-%d", project.ID, seq)
		fileID, err := s.redisClient.Get(ctx, key).Result()
		if err != nil {
			logger.Errorf("redis get failed, key=%s, err=%v", key, err)
			return nil, err
		}
		defaultEngine, _ := s.FileStorageSystem.CreateDefaultStorage(ctx, "")
		file, err := s.FileStorageSystem.FinalizeUpload(ctx, fileID, defaultEngine)
		if err == nil {
			s.redisClient.Del(ctx, key)
		}
		return file, err
	}

	if project.Type == commonmodels.ProjectTypeSubgraph {
		// for subgraph project, will only exist the file with sequence 2
		if req.Sequence != 2 {
			return nil, fmt.Errorf("invalid sequence: %d", req.Sequence)
		}
		var p *models.Processor
		if req.ContinueFrom > 0 {
			log.Infof("specified continue from for project %s, will update properties of processor %d",
				req.ProjectSlug, req.ContinueFrom)
			p, err = s.processorRepo.FindProcessorByVersion(ctx, project.ID, req.ContinueFrom)
			if err != nil {
				return nil, fmt.Errorf("find processor with version %d failed: %w", req.ContinueFrom, err)
			}
		} else {
			p, err = s.processorRepo.FindLatestProcessor(ctx, project.ID)
			if err != nil {
				return nil, fmt.Errorf("find last processor failed: %w", err)
			}
		}

		file, err := renameFile(2)
		if err != nil {
			return nil, err
		}
		p.ZipURL = file.GetUrl(ctx)
		p.CliVersion = req.CliVersion
		p.SdkVersion = req.SdkVersion
		p.Warnings = req.Warnings
		// cannot change p.DriverVersion here because driver maybe already started
		if err := s.processorRepo.SaveProcessor(ctx, p); err != nil {
			err = errors.Wrapf(err, "update zipURL for processor %s/%s/%d/%s failed",
				project.GetOwnerName(), project.Slug, p.Version, p.ID)
			logger.Errore(err)
			return nil, err
		}
		log.Infof("updated properties for processor %s with version %d of subgraph project %s",
			p.ID, p.Version, req.ProjectSlug)
		return &protos.FinishUploadResponse{
			ProjectFullSlug: project.GetOwnerName() + "/" + project.Slug,
			ProcessorId:     p.ID,
			Version:         p.Version,
		}, nil
	}

	if req.ContinueFrom > 0 {
		var p *models.Processor
		p, err = s.processorRepo.FindProcessorByVersion(ctx, project.ID, req.ContinueFrom)
		if err != nil {
			return nil, errors.Wrapf(err, "find processor with version %d failed", req.ContinueFrom)
		}
		sdkVerBefore, _ := processor.ParseVersion(p.SdkVersion)
		sdkVerAfter, _ := processor.ParseVersion(req.GetSdkVersion())
		if sdkVerBefore.Major != sdkVerAfter.Major {
			return nil, fmt.Errorf("cannnot update sdk version from %s to %s: cannot change major version",
				p.SdkVersion, req.GetSdkVersion())
		}
	}

	if req.Sequence > 1 {
		return nil, fmt.Errorf("invalid sequence: %d", req.Sequence)
	}

	var files [2]*corestorage.FileObject
	// move processor file to permanent storage
	for seq := int32(0); seq <= req.Sequence; seq++ {
		files[seq], err = renameFile(seq)
		if err != nil {
			return nil, err
		}
	}
	var zipURL string

	if files[1] != nil {
		zipURL = files[1].GetUrl(ctx)
	}

	if req.ContinueFrom > 0 {
		log.Infof("specified continue from for project %s, will try to upgrade processor %d",
			req.ProjectSlug, req.ContinueFrom)
	}
	p, err := s.CreateOrUpdateProcessor(
		ctx,
		identity,
		project,
		req.ContinueFrom,
		req.Rollback,
		req.NumWorkers,
		models.SentioProcessorProperties{
			CliVersion:       req.CliVersion,
			SdkVersion:       req.SdkVersion,
			CodeURL:          files[0].GetUrl(ctx),
			CodeHash:         req.Sha256,
			CommitSha:        req.CommitSha,
			GitURL:           req.GitUrl,
			ZipURL:           zipURL,
			Debug:            req.Debug,
			NetworkOverrides: models.BuildNetworkOverrides(req.NetworkOverrides),
			Binary:           req.Binary,
		},
		models.SubgraphProcessorProperties{},
		models.SentioNetworkProperties{ChainID: chains.ChainID(req.GetSentioNetwork())},
	)
	if err != nil {
		return nil, err
	}

	return &protos.FinishUploadResponse{
		ProjectFullSlug: project.GetOwnerName() + "/" + project.Slug,
		ProcessorId:     p.ID,
		Version:         p.Version,
	}, nil
}

func (s *Service) DownloadProcessor(
	ctx context.Context,
	req *protos.DownloadProcessorRequest,
) (*protos.DownloadProcessorResponse, error) {
	p, err := s.processorRepo.GetProcessor(ctx, req.ProcessorId, false)
	if err != nil {
		return nil, err
	}

	obj, err := s.FileStorageSystem.GetFromUrl(ctx, p.CodeURL)
	if err != nil {
		return nil, err
	}

	url, err := obj.PreSignedDownloadUrl(ctx, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	return &protos.DownloadProcessorResponse{
		Url: url,
	}, nil
}

func (s *Service) deleteProcessorGcs(ctx context.Context, processor *models.Processor) error {
	obj, err := s.FileStorageSystem.GetFromUrl(ctx, processor.CodeURL)
	if err != nil {
		return err
	}
	err = obj.Delete(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) InitBatchUpload(
	ctx context.Context,
	req *protos.InitBatchUploadRequest,
) (*protos.InitBatchUploadResponse, error) {
	logger := log.WithContext(ctx)
	var project *commonmodels.Project
	var err error

	project, err = s.processorRepo.PreLoadProject(ctx, req.ProjectOwner, req.ProjectSlug)
	if err != nil || project == nil {
		return nil, status.Errorf(codes.NotFound, "Project not found: %v", err)
	}

	var warning string
	engine := req.Engine

	if project.Type == commonmodels.ProjectTypeSubgraph {
		// For subgraph projects, only IPFS storage is supported
		if req.Engine != protos.StorageEngine_IPFS && req.Engine != protos.StorageEngine_DEFAULT {
			return nil, fmt.Errorf("subgraph projects only support IPFS storage, got: %v", req.Engine)
		}
	} else {
		if project.SentioNetworkProject {
			engine = protos.StorageEngine_IPFS
		}

		clientVersion := req.SdkVersion
		serverVersion := processor.HostProcessorVersion()
		err := processor.VersionCheck(serverVersion, clientVersion)
		if err != nil {
			warning = err.Error()
		}
	}

	var storageEngine corestorage.FileStorageEngine

	storageEngine, err = s.FileStorageSystem.CreateDefaultStorage(ctx, engine.String())

	if err != nil {
		return nil, err
	}

	var replacingVersion int32
	replacingProcessor, _ := s.processorRepo.FindReplacingProcessor(ctx, project)
	if replacingProcessor != nil {
		replacingVersion = replacingProcessor.Version
	}

	payloads := make(map[string]*protos.UploadPayload)
	contentType := "application/octet-stream"

	for fileKey, fileType := range req.FileTypes {
		fileID, err := s.generateFileID()
		if err != nil {
			logger.Errorf("generate file id failed for %s, err=%v", fileKey, err)
			return nil, err
		}

		file := s.FileStorageSystem.NewUploadFileWithEngine(storageEngine, fileID, contentType)

		url, err := file.PreSignedUploadUrl(ctx, 5*time.Minute)
		if err != nil {
			logger.Errorf("sign url failed for %s, err=%v", fileKey, err)
			return nil, err
		}

		// Create payload for this file
		payloads[fileKey] = storageEngine.ToPayload(file, fileID, url, fileType)
	}

	return &protos.InitBatchUploadResponse{
		Warning:          warning,
		ReplacingVersion: replacingVersion,
		MultiVersion:     project.MultiVersion,
		ProjectId:        project.ID,
		Engine:           engine,
		Payloads:         payloads,
	}, nil
}

func (s *Service) FinishBatchUpload(
	ctx context.Context,
	req *protos.FinishBatchUploadRequest,
) (*protos.FinishBatchUploadResponse, error) {
	logger := log.WithContext(ctx)
	var project *commonmodels.Project
	var err error

	project, err = s.processorRepo.PreLoadProject(ctx, req.ProjectOwner, req.ProjectSlug)
	if err != nil || project == nil {
		return nil, status.Errorf(codes.NotFound, "Project not found: %v", err)
	}

	// Validate storage engine based on project type
	if project.Type == commonmodels.ProjectTypeSubgraph {
		// For subgraph projects, only IPFS storage is supported
		if req.Engine != protos.StorageEngine_IPFS && req.Engine != protos.StorageEngine_DEFAULT {
			return nil, fmt.Errorf("subgraph projects only support IPFS storage, got: %v", req.Engine)
		}
	} else {
		// For non-subgraph projects, IPFS storage is  chosen automatically if SentioNetworkProject is true
		if project.SentioNetworkProject {
			req.Engine = protos.StorageEngine_IPFS
		}
	}

	identity := preloader.PreLoadedIdentity(ctx)

	// Create storage based on engine from request
	var storageEngine corestorage.FileStorageEngine

	storageEngine, err = s.FileStorageSystem.CreateDefaultStorage(ctx, req.Engine.String())

	if err != nil {
		return nil, err
	}

	if len(req.Payloads) == 0 {
		return nil, fmt.Errorf("no file payloads provided")
	}

	// Finalize uploads for all files
	finalizedFiles := make(map[string]*corestorage.FileObject)
	for fileKey, payload := range req.Payloads {
		var fileID string
		switch p := payload.Payload.(type) {
		case *protos.UploadPayload_Object:
			fileID = p.Object.FileId
		case *protos.UploadPayload_Walrus:
			fileID = p.Walrus.BlobId + "/" + p.Walrus.QuiltPatchId
		case *protos.UploadPayload_Ipfs:
			fileID = p.Ipfs.Cid + "/" + p.Ipfs.Path
		default:
			return nil, fmt.Errorf("invalid payload for file %s", fileKey)
		}

		file, err := s.FileStorageSystem.FinalizeUpload(ctx, fileID, storageEngine)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to finalize upload for file %s", fileKey)
		}
		finalizedFiles[fileKey] = file
	}

	// Handle subgraph projects
	if project.Type == commonmodels.ProjectTypeSubgraph {
		var p *models.Processor
		if req.ContinueFrom > 0 {
			log.Infof("specified continue from for project %s, will update properties of processor %d",
				req.ProjectSlug, req.ContinueFrom)
			p, err = s.processorRepo.FindProcessorByVersion(ctx, project.ID, req.ContinueFrom)
			if err != nil {
				return nil, fmt.Errorf("find processor with version %d failed: %w", req.ContinueFrom, err)
			}
		} else {
			p, err = s.processorRepo.FindLatestProcessor(ctx, project.ID)
			if err != nil {
				return nil, fmt.Errorf("find last processor failed: %w", err)
			}
		}

		// For subgraph, use the first available file as ZipURL
		for _, file := range finalizedFiles {
			p.ZipURL = file.GetUrl(ctx)
			break
		}
		p.CliVersion = req.CliVersion
		p.SdkVersion = req.SdkVersion
		p.Warnings = req.Warnings
		if err := s.processorRepo.SaveProcessor(ctx, p); err != nil {
			err = errors.Wrapf(err, "update zipURL for processor %s/%s/%d/%s failed",
				project.GetOwnerName(), project.Slug, p.Version, p.ID)
			logger.Errore(err)
			return nil, err
		}
		log.Infof("updated properties for processor %s with version %d of subgraph project %s",
			p.ID, p.Version, req.ProjectSlug)
		return &protos.FinishBatchUploadResponse{
			ProjectFullSlug: project.GetOwnerName() + "/" + project.Slug,
			ProcessorId:     p.ID,
			Version:         p.Version,
		}, nil
	}

	// Handle regular processor project with multiple files
	var codeURL, zipURL string
	var codeHash string

	// Extract URLs from finalized files
	for fileKey, file := range finalizedFiles {
		payload := req.Payloads[fileKey]
		fileType := payload.FileType
		switch fileType {
		case protos.FileType_PROCESSOR:
			codeURL = file.GetUrl(ctx)
			codeHash = req.Sha256Map[fileKey]
		case protos.FileType_SOURCE:
			zipURL = file.GetUrl(ctx)
		}
	}

	if req.ContinueFrom > 0 {
		log.Infof("specified continue from for project %s, will try to upgrade processor %d",
			req.ProjectSlug, req.ContinueFrom)
	}
	p, err := s.CreateOrUpdateProcessor(
		ctx,
		identity,
		project,
		req.ContinueFrom,
		req.Rollback,
		req.NumWorkers,
		models.SentioProcessorProperties{
			CliVersion:       req.CliVersion,
			SdkVersion:       req.SdkVersion,
			CodeURL:          codeURL,
			CodeHash:         codeHash,
			CommitSha:        req.CommitSha,
			GitURL:           req.GitUrl,
			ZipURL:           zipURL,
			Debug:            req.Debug,
			NetworkOverrides: models.BuildNetworkOverrides(req.NetworkOverrides),
			Binary:           req.Binary,
		},
		models.SubgraphProcessorProperties{},
		models.SentioNetworkProperties{ChainID: chains.ChainID(req.GetSentioNetwork())},
	)
	if err != nil {
		return nil, err
	}

	return &protos.FinishBatchUploadResponse{
		ProjectFullSlug: project.GetOwnerName() + "/" + project.Slug,
		ProcessorId:     p.ID,
		Version:         p.Version,
	}, nil
}

func (s *Service) generateFileID() (string, error) {
	id, err := gonanoid.GenerateID()
	if err != nil {
		return "", err
	}
	return id, nil
}
