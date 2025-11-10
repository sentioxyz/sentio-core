package preloader

import (
	"context"
	"sentioxyz/sentio-core/service/common/models"
)

type ProjectKey string

const ProjectKeyName ProjectKey = "project"

func PreLoadedProject(ctx context.Context) *models.Project {
	if project, ok := ctx.Value(ProjectKeyName).(*models.Project); ok {
		return project
	}
	return nil // Return nil if no project is found in the context
}
