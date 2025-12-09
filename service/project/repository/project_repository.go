package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	commonmodels "sentioxyz/sentio-core/service/common/models"

	"github.com/redis/go-redis/v9"
)

const (
	projectKeyPrefix = "project:"
	projectSlugsKey  = "project:slugs"
)

type ProjectRepository interface {
	GetProject(ctx context.Context, owner, slug string) (*commonmodels.Project, error)
	GetProjectById(ctx context.Context, id string) (*commonmodels.Project, error)
	SaveProject(ctx context.Context, project *commonmodels.Project) error
	DeleteProject(ctx context.Context, id string) error
}

type RedisProjectRepository struct {
	client *redis.Client
}

func NewRedisProjectRepository(client *redis.Client) *RedisProjectRepository {
	return &RedisProjectRepository{
		client: client,
	}
}

func (r *RedisProjectRepository) GetProject(ctx context.Context, owner, slug string) (*commonmodels.Project, error) {
	slugKey := fmt.Sprintf("%s/%s", owner, slug)
	id, err := r.client.HGet(ctx, projectSlugsKey, slugKey).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("project not found: %s/%s", owner, slug)
	}
	if err != nil {
		return nil, err
	}

	return r.GetProjectById(ctx, id)
}

func (r *RedisProjectRepository) GetProjectById(ctx context.Context, id string) (*commonmodels.Project, error) {
	key := projectKeyPrefix + id
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("project not found: %s", id)
	}
	if err != nil {
		return nil, err
	}

	var project commonmodels.Project
	if err := json.Unmarshal([]byte(data), &project); err != nil {
		return nil, err
	}

	return &project, nil
}

func (r *RedisProjectRepository) SaveProject(ctx context.Context, project *commonmodels.Project) error {
	// Check if this is a new project (no existing record)
	_, err := r.GetProjectById(ctx, project.ID)
	isNew := err != nil

	// Set timestamps
	now := time.Now()
	if isNew {
		project.CreatedAt = now
	}
	project.UpdatedAt = now

	data, err := json.Marshal(project)
	if err != nil {
		return err
	}

	key := projectKeyPrefix + project.ID
	slugKey := fmt.Sprintf("%s/%s", project.GetOwnerName(), project.Slug)

	pipe := r.client.Pipeline()
	pipe.Set(ctx, key, data, 0)
	pipe.HSet(ctx, projectSlugsKey, slugKey, project.ID)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisProjectRepository) DeleteProject(ctx context.Context, id string) error {
	// We need the project to know the slug for removing from HMap
	project, err := r.GetProjectById(ctx, id)
	if err != nil {
		return err
	}

	key := projectKeyPrefix + id
	slugKey := fmt.Sprintf("%s/%s", project.GetOwnerName(), project.Slug)

	pipe := r.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.HDel(ctx, projectSlugsKey, slugKey)
	_, err = pipe.Exec(ctx)
	return err
}
