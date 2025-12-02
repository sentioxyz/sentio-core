package repository

import (
	"context"
	"testing"

	commonmodels "sentioxyz/sentio-core/service/common/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRedisProjectRepository(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	repo := NewRedisProjectRepository(client)
	ctx := context.Background()

	project := &commonmodels.Project{
		ID:        "test-id",
		Slug:      "test-slug",
		OwnerName: "test-owner",
	}

	// Test SaveProject
	err = repo.SaveProject(ctx, project)
	assert.NoError(t, err)

	// Verify HMap entry exists
	slugKey := "test-owner/test-slug"
	id, err := client.HGet(ctx, "project:slugs", slugKey).Result()
	assert.NoError(t, err)
	assert.Equal(t, project.ID, id)

	// Test GetProjectById
	savedProject, err := repo.GetProjectById(ctx, "test-id")
	assert.NoError(t, err)
	assert.Equal(t, project.ID, savedProject.ID)
	assert.Equal(t, project.Slug, savedProject.Slug)

	// Test GetProject (by owner/slug)
	savedProject, err = repo.GetProject(ctx, "test-owner", "test-slug")
	assert.NoError(t, err)
	assert.Equal(t, project.ID, savedProject.ID)

	// Test DeleteProject
	err = repo.DeleteProject(ctx, "test-id")
	assert.NoError(t, err)

	// Verify deletion
	_, err = repo.GetProjectById(ctx, "test-id")
	assert.Error(t, err)

	// Verify HMap entry removed
	_, err = client.HGet(ctx, "project:slugs", slugKey).Result()
	assert.Equal(t, redis.Nil, err)
}
