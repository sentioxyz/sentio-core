package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm/clause"

	"sentioxyz/sentio-core/common/log"

	commonProtos "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/processor/protos"

	"gorm.io/gorm"

	common "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/processor/models"
)

type DBRepository struct {
	DB *gorm.DB
}

type DBRepoInterface interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	GetDB(ctx context.Context) *gorm.DB
}

// WithTransaction implements repository.FileRepoIntf - starts a transaction and puts it in context
func (r *DBRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return fn(ctx)
	} else {
		return r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			ctx := context.WithValue(ctx, txKey{}, tx)
			return fn(ctx)
		})
	}
}

// GetDB returns the transaction from context if available, otherwise returns the regular DB
func (r *DBRepository) GetDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return r.DB.WithContext(ctx)
}

type Repository struct {
	DBRepository
}

func NewRepository(db *gorm.DB) Repository {
	return Repository{
		DBRepository: DBRepository{
			DB: db,
		},
	}
}

// Transaction context key for storing *gorm.DB
type txKey struct{}

func getOwnerID(db *gorm.DB, ownerName string) (string, error) {
	var user common.User
	result := db.Where(&common.User{Username: ownerName}).Select("id").First(&user)
	if result.Error == nil {
		return user.ID, nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", result.Error
	}
	var org common.Organization
	result = db.Where(&common.Organization{Name: ownerName}).Select("id").First(&org)
	return org.ID, result.Error
}

func (r *Repository) GetProjectIDBySlug(ctx context.Context, ownerName string, slug string) (string, error) {
	db := r.DB.WithContext(ctx)
	ownerID, err := getOwnerID(db, ownerName)
	if err != nil {
		return "", err
	}
	var project common.Project
	result := db.Where(&common.Project{OwnerID: ownerID, Slug: slug}).Select("id").First(&project)
	return project.ID, result.Error
}

// GetProjectBySlug returns nil if project not found
func (r *Repository) GetProjectBySlug(ctx context.Context, ownerName string, slug string) (*common.Project, error) {
	var project common.Project
	var user common.User
	db := r.DB.WithContext(ctx)
	result := db.Where(&common.User{Username: ownerName}).First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		var org common.Organization
		result = db.Where(&common.Organization{Name: ownerName}).First(&org)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if result.Error != nil {
			return nil, result.Error
		}
		result = db.Preload("NotificationChannels").
			Preload("Members").
			Preload("Views").
			Preload("Community").
			Where(&common.Project{OwnerID: org.ID, Slug: slug}).
			First(&project)
		return &project, result.Error
	}
	if result.Error != nil {
		return nil, result.Error
	}

	result = db.Preload("NotificationChannels").
		Preload("Members").
		Preload("Views").
		Preload("Community").
		Where(&common.Project{OwnerID: user.ID, Slug: slug}).
		First(&project)
	return &project, result.Error
}

func (r *Repository) SaveProject(tx *gorm.DB, project *common.Project) error {
	return tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(project).Error
}

type resolvingContextKey string

const resolving resolvingContextKey = "resolving"

// GetProcessorsByProjectAndVersion return the processors of the project with the given version
// if the version is less than 1, return the active version
// if there is no active version, return the latest pending version
func (r *Repository) GetProcessorsByProjectAndVersion(
	ctx context.Context,
	projectID string,
	version int32,
) (models.Processors, error) {
	var processors []*models.Processor
	if version > 0 {
		if err := r.DB.WithContext(ctx).
			Where(&models.Processor{
				ProjectID: projectID,
				Version:   version,
			}).
			Find(&processors).Error; err != nil {
			return nil, err
		}
	} else {
		// find the active version
		if err := r.DB.WithContext(ctx).
			Where(&models.Processor{
				ProjectID:    projectID,
				VersionState: int32(protos.ProcessorVersionState_ACTIVE)},
			).
			Find(&processors).Error; err != nil {
			return nil, err
		}

		// backward compatibility, no active version, find the latest pending version
		if len(processors) == 0 {
			if err := r.DB.WithContext(ctx).
				Where(&models.Processor{
					ProjectID:    projectID,
					VersionState: int32(protos.ProcessorVersionState_PENDING)},
				).
				Order("version DESC").
				Limit(1).
				Find(&processors).Error; err != nil {
				return nil, err
			}
		}
	}

	ret := make([]*models.Processor, 0)
	for _, p := range processors {
		if len(p.ReferenceProjectID) > 0 {

			if r, ok := ctx.Value(resolving).(bool); ok && r {
				err := errors.New("circular reference detected")
				log.Errorf("project %s: circular reference detected in processor %s", projectID, p.ID)
				return nil, err
			}

			ctx := context.WithValue(ctx, resolving, true)
			rp, err := r.ResolveReferenceProcessor(ctx, p)
			if err != nil {
				return nil, err
			}
			if rp != nil {
				ret = append(ret, rp)
			}
		} else {
			ret = append(ret, p)
		}
	}

	return ret, nil
}

func (r *Repository) GetProcessor(ctx context.Context, processorID string) (*models.Processor, error) {
	var processor models.Processor
	err := r.DB.WithContext(ctx).Preload("User").Preload("Project").First(&processor, "id = ?", processorID).Error
	if err != nil {
		return nil, err
	}
	return &processor, nil
}

func (r *Repository) GetProcessors(ctx context.Context, processorIDs []string) (models.Processors, error) {
	var processors models.Processors
	err := r.DB.WithContext(ctx).Where("id IN ?", processorIDs).Find(&processors).Error
	if err != nil {
		return nil, err
	}
	return processors, nil
}

func (r *Repository) FindLastProcessor(ctx context.Context, projectID string) (*models.Processor, error) {
	processor := &models.Processor{}
	err := r.DB.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("version desc").
		First(&processor).
		Error
	if err != nil {
		return nil, err
	}
	return processor, nil
}

func (r *Repository) FindRunningProcessor(ctx context.Context, projectID string, version int32) (*models.Processor, error) {
	if version <= 0 {
		return r.FindLastActiveProcessor(ctx, projectID)
	}
	processor := &models.Processor{}
	err := r.DB.WithContext(ctx).
		Where("project_id = ?", projectID).
		Where("version = ?", version).
		Where("(version_state = ? or version_state = ?)", int32(protos.ProcessorVersionState_ACTIVE), int32(protos.ProcessorVersionState_PENDING)).
		First(&processor).
		Error
	if err != nil {
		return nil, err
	}
	return processor, nil
}

func (r *Repository) FindLastActiveProcessor(ctx context.Context, projectID string) (*models.Processor, error) {
	processor := &models.Processor{}
	err := r.DB.WithContext(ctx).
		Where("project_id = ?", projectID).
		Where("version_state = ?", int32(protos.ProcessorVersionState_ACTIVE)).
		Order("version desc").
		First(&processor).
		Error
	if err != nil {
		return nil, err
	}
	return processor, nil
}

func CreateProcessor(
	projectID string,
	contractID string,
) (*models.Processor, error) {
	processor := models.Processor{ProjectID: projectID}
	processor.Version += 1
	processor.SdkVersion = ""
	processor.CodeURL = ""
	if contractID != "" {
		processor.ContractID = &contractID
	}

	return &processor, nil
}

func (r *Repository) FindTelegramChannelByReference(ctx context.Context, ref string) (*common.Channel, error) {
	db := r.DB.WithContext(ctx)
	var channel common.Channel
	err := db.First(&channel, " type = ? and telegram_reference = ?", commonProtos.Channel_TELEGRAM.String(), ref).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *Repository) UpdateTelegramChannelChatID(ctx context.Context, channel *common.Channel, chatID string) error {
	db := r.DB.WithContext(ctx)
	channel.TelegramChatID = chatID
	channel.TelegramReference = ""
	return db.Save(channel).Error
}

func (r *Repository) FindWebhookChannel(ctx context.Context,
	projectID *string, id *string, name *string) (*common.Channel, error) {
	var channel common.Channel
	db := r.DB.WithContext(ctx).Preload("Project")
	if id != nil {
		db = db.Where("id = ?", id)
	} else {
		if projectID == nil || name == nil {
			return nil, fmt.Errorf("missing project id or channel name")
		}
		db = db.Where("project_id = ? and name = ?", projectID, name)
	}
	err := db.
		Where("type = ?", commonProtos.Channel_WEBHOOK.String()).
		First(&channel).Error
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func CleanupName(name string) string {
	ret := name
	re, err := regexp.Compile(`[^0-9A-Za-z]`)
	if err != nil {
		log.Fatale(err)
	}
	ret = re.ReplaceAllString(ret, "")
	ret = strings.ToLower(ret)
	return ret
}

func (r *Repository) MakeJobName(processor *models.Processor) (string, error) {
	if processor.Project == nil {
		var project common.Project
		// it's possible that the project is soft deleted.
		err := r.DB.Unscoped().Where(common.Project{ID: processor.ProjectID}).First(&project).Error
		if err != nil {
			return "", err
		}
		// set processor.Project
		processor.Project = &project
	}

	if processor.Project.OwnerAsUser == nil && processor.Project.OwnerAsOrg == nil {
		processor.Project.GetOwner(r.DB)
	}

	// the deleting project slug has been appended with the project id
	slug := strings.TrimSuffix(processor.Project.Slug, fmt.Sprintf("-%s", processor.Project.ID))
	return fmt.Sprintf(
		"driver-%s-%s-%s", CleanupName(processor.ID),
		CleanupName(processor.Project.GetOwnerName()), CleanupName(slug),
	), nil
}

func (r *Repository) GetIdentity(db *gorm.DB, sub string) (identity *common.Identity, err error) {
	identity = &common.Identity{Sub: sub}

	err = db.Preload("User").First(identity).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return
}

func (r *Repository) GetAPIKey(db *gorm.DB, id string) (*common.APIKey, error) {
	var apiKey common.APIKey
	err := db.First(&apiKey, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	apiKey.GetOwner(db)
	return &apiKey, nil
}

func (r *Repository) GetProjectByID(db *gorm.DB, id string) (*common.Project, error) {
	var project common.Project
	err := db.
		Preload("NotificationChannels").
		Preload("Members").
		Preload("Views").
		Preload("Community").
		First(&project, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	project.GetOwner(db)
	return &project, nil
}

func (r *Repository) GetProjectByIDUnscoped(db *gorm.DB, id string) (*common.Project, error) {
	var project common.Project
	err := db.
		Unscoped().
		First(&project, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	project.GetOwner(db)
	return &project, nil
}

func (r *Repository) GetOrganization(db *gorm.DB, idOrName string) (*common.Organization, error) {
	var org common.Organization
	err := db.Preload("Members").Preload("Members.User").First(&org, "id = ? or name = ?", idOrName, idOrName).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (r *Repository) GetUser(db *gorm.DB, idEmailUsername string) (*common.User, error) {
	var user common.User
	err := db.First(
		&user,
		"id = ? or LOWER(email) = LOWER(?) or LOWER(username) = LOWER(?)",
		idEmailUsername,
		idEmailUsername,
		idEmailUsername,
	).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) DeleteProject(ctx context.Context, tx *gorm.DB, project *common.Project) error {
	if err := tx.Delete(project).Error; err != nil {
		return err
	}
	return nil
}

func (r *Repository) FindImportedProject(
	ctx context.Context,
	projectID string,
	name string,
) (*common.ImportedProject, error) {
	var importedProject common.ImportedProject
	err := r.DB.WithContext(ctx).Find(&importedProject, "project_id = ? and name = ?", projectID, name).Error
	return &importedProject, err
}

func (r *Repository) ListImportedProjects(
	ctx context.Context,
	projectID string) ([]*common.ImportedProject, error) {
	var importedProjects []*common.ImportedProject
	err := r.DB.WithContext(ctx).
		Preload("ImportProject").
		Preload("Project").
		Find(&importedProjects, "project_id = ?", projectID).Error
	return importedProjects, err
}

func (r *Repository) GetProjectVariables(ctx context.Context, projectID string) ([]*common.ProjectVariable, error) {
	var projectVariables []*common.ProjectVariable
	err := r.DB.WithContext(ctx).Preload("Project").Find(&projectVariables, "project_id = ?", projectID).Error

	return projectVariables, err
}

func (r *Repository) ResolveReferenceProcessor(ctx context.Context, processor *models.Processor) (*models.Processor, error) {
	if len(processor.ReferenceProjectID) > 0 {
		// resolve reference project
		ps, err := r.GetProcessorsByProjectAndVersion(ctx, processor.ReferenceProjectID, 0)
		if err != nil {
			return nil, err
		}
		if len(ps) > 0 {
			rp := ps[0]
			if rp.ID == processor.ID {
				return nil, fmt.Errorf("processor %s references itself", processor.ID)
			}
			log.Debugf("project %s: resolved reference processor %s to %s", processor.ProjectID, processor.ID, rp.ID)
			return rp, nil
		}
		// no active processor found
		return nil, nil
	}

	return processor, nil
}
