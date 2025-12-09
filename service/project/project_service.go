package project

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"sentioxyz/sentio-core/common/gonanoid"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	commonprotos "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/project/protos"
	"sentioxyz/sentio-core/service/project/repository"
)

type ProjectService struct {
	protos.UnimplementedProjectServiceServer
	repo repository.ProjectRepository
}

func NewProjectService(repo repository.ProjectRepository) *ProjectService {
	return &ProjectService{
		repo: repo,
	}
}

func (s *ProjectService) GetProject(ctx context.Context, req *protos.ProjectOwnerAndSlug) (*protos.GetProjectResponse, error) {
	project, err := s.repo.GetProject(ctx, req.OwnerName, req.Slug)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "project not found: %v", err)
	}

	return &protos.GetProjectResponse{
		Project: project.ToPB(),
	}, nil
}

func (s *ProjectService) GetProjectById(ctx context.Context, req *protos.GetProjectByIdRequest) (*commonprotos.ProjectInfo, error) {
	project, err := s.repo.GetProjectById(ctx, req.ProjectId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "project not found: %v", err)
	}

	return project.ToProjectInfo(), nil
}

func (s *ProjectService) SaveProject(ctx context.Context, req *commonprotos.Project) (*commonprotos.Project, error) {
	// Create or Update logic
	if req.Id == "" {
		// Creating a new project

		// Check if project with same owner/slug already exists
		existingProject, err := s.repo.GetProject(ctx, req.OwnerName, req.Slug)
		if err == nil && existingProject != nil {
			return nil, status.Errorf(codes.AlreadyExists, "project with owner %s and slug %s already exists", req.OwnerName, req.Slug)
		}

		// Generate new project ID
		project := &commonmodels.Project{}
		project.FromPB(req)

		// Generate ID using the same method as BeforeCreate
		id, err := gonanoid.GenerateID()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to generate project ID: %v", err)
		}
		project.ID = id

		// Validate slug pattern
		if !gonanoid.CheckIDMatchPattern(project.Slug, false, true) {
			return nil, status.Errorf(codes.InvalidArgument, "invalid project slug")
		}

		// Save the new project
		if err := s.repo.SaveProject(ctx, project); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to save project: %v", err)
		}

		return project.ToPB(), nil
	} else {
		// Updating existing project

		// Get existing project
		existingProject, err := s.repo.GetProjectById(ctx, req.Id)
		if err != nil {
			return nil, status.Errorf(codes.NotFound, "project not found: %v", err)
		}

		// Prevent changing owner and slug
		if existingProject.GetOwnerName() != req.OwnerName || existingProject.Slug != req.Slug {
			return nil, status.Errorf(codes.InvalidArgument, "cannot change owner or slug of existing project")
		}

		// Update project
		project := &commonmodels.Project{}
		project.FromPB(req)

		if err := s.repo.SaveProject(ctx, project); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to update project: %v", err)
		}

		return project.ToPB(), nil
	}
}

func (s *ProjectService) DeleteProject(ctx context.Context, req *protos.GetProjectByIdRequest) (*commonprotos.Project, error) {
	project, err := s.repo.GetProjectById(ctx, req.ProjectId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "project not found: %v", err)
	}

	if err := s.repo.DeleteProject(ctx, req.ProjectId); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete project: %v", err)
	}

	return project.ToPB(), nil
}
