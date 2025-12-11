package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"sentioxyz/sentio-core/common/gonanoid"
	commonmodels "sentioxyz/sentio-core/service/common/models"
	"sentioxyz/sentio-core/service/processor/models"
	"sentioxyz/sentio-core/service/processor/protos"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Redis key prefixes
const (
	processorKeyPrefix        = "processor:"
	processorProjectKeyPrefix = "processor:project:"
	chainStateKeyPrefix       = "chain_state:"
	chainStatesKeyPrefix      = "chain_states:"
	upgradeHistoryKeyPrefix   = "upgrade_history:"
	projectKeyPrefix          = "project:"
	projectVersionsKeyPrefix  = "project:versions:"
	projectVariablesKeyPrefix = "project:variables:"
)

// RedisProcessorRepoInterface defines the processor repository interface for Redis implementations
// This mirrors ProcessorRepo but uses Redis-specific transaction handling
type RedisProcessorRepoInterface interface {
	GetProcessor(ctx context.Context, processorID string, withChainState bool) (*models.Processor, error)
	GetProcessors(ctx context.Context, projectID string) ([]models.Processor, error)
	SaveProcessor(ctx context.Context, processor *models.Processor) error
	RemoveProcessor(ctx context.Context, processorID string) error
	ObsoleteProcessor(ctx context.Context, processorID string) error
	PreloadProcessor(ctx context.Context, id string) (*models.Processor, error)

	FindProcessorByVersion(ctx context.Context, projectID string, version int32) (*models.Processor, error)
	FindActiveProcessor(ctx context.Context, projectID string) (*models.Processor, error)
	FindLatestProcessor(ctx context.Context, projectID string) (*models.Processor, error)
	FindReplacingProcessor(ctx context.Context, project *commonmodels.Project) (*models.Processor, error)
	ResolveReferenceProcessor(ctx context.Context, processor *models.Processor) (*models.Processor, error)

	GetProcessorsByProjectAndVersionState(ctx context.Context, projectID string, versionStates ...protos.ProcessorVersionState) ([]*models.Processor, error)
	GetObsoleteProcessors(ctx context.Context, projectID string) ([]models.Processor, error)

	CreateOrUpdateProcessor(
		ctx context.Context,
		project *commonmodels.Project,
		continueFrom int32,
		pause bool,
		identity *commonmodels.Identity,
		clickhouseShardingIndex int32,
		k8sClusterID int32,
		numWorkers int32,
		sentioProperties models.SentioProcessorProperties,
		subgraphProperties models.SubgraphProcessorProperties,
	) (*models.Processor, error)

	ListProcessorUpgradeHistory(ctx context.Context, processorID string) ([]models.ProcessorUpgradeHistory, error)
	GetProcessorUpgradeHistoryByID(ctx context.Context, historyID string, processorID string) (*models.ProcessorUpgradeHistory, error)
	SaveProcessorUpgradeHistory(ctx context.Context, processor *models.Processor) error

	GetProjectByID(ctx context.Context, projectID string) (*commonmodels.Project, error)
	GetProjectVersions(ctx context.Context, projectID string) ([]*models.Processor, error)
	GetProjectVariables(ctx context.Context, projectID string) ([]*commonmodels.ProjectVariable, error)
	PreLoadProject(ctx context.Context, owner, slug string) (*commonmodels.Project, error)

	// Common repository interface methods
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	GetDB(ctx context.Context) *gorm.DB

	// Redis-specific transaction support
	WithRedisTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	GetRedisClient(ctx context.Context) *redis.Client
}

// RedisChainStateRepoInterface defines the chain state repository interface for Redis implementations
type RedisChainStateRepoInterface interface {
	GetChainState(ctx context.Context, processorID string, chainID string) (models.ChainState, error)
	GetChainStates(ctx context.Context, processorID string) ([]models.ChainState, error)
	UpdateChainState(ctx context.Context, chainState *models.ChainState) error
	DeleteChainStatesByProcessor(ctx context.Context, processorID string) error
	ListChainsByProjects(ctx context.Context, projectIDList []string, versionStatList []protos.ProcessorVersionState) (map[string][]string, error)

	// Common repository interface methods
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	GetDB(ctx context.Context) *gorm.DB

	// Redis-specific transaction support
	WithRedisTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	GetRedisClient(ctx context.Context) *redis.Client
}

// RedisProcessorRepo implements RedisProcessorRepoInterface using Redis as the storage backend
type RedisProcessorRepo struct {
	client *redis.Client
}

// Ensure RedisProcessorRepo implements RedisProcessorRepoInterface
var _ RedisProcessorRepoInterface = (*RedisProcessorRepo)(nil)

// NewRedisProcessorRepo creates a new Redis-based processor repository
func NewRedisProcessorRepo(client *redis.Client) *RedisProcessorRepo {
	return &RedisProcessorRepo{
		client: client,
	}
}

// Redis transaction context key
type redisTxKey struct{}

// WithRedisTransaction implements a simple transaction pattern for Redis using MULTI/EXEC
func (r *RedisProcessorRepo) WithRedisTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// Check if we're already in a transaction
	if _, ok := ctx.Value(redisTxKey{}).(*redis.Tx); ok {
		return fn(ctx)
	}

	// Start a new transaction (optimistic locking with WATCH/MULTI/EXEC)
	err := r.client.Watch(ctx, func(tx *redis.Tx) error {
		// Store transaction in context
		ctx = context.WithValue(ctx, redisTxKey{}, tx)
		return fn(ctx)
	})

	return err
}

// WithTransaction implements repository.DBRepoInterface for compatibility
// This is an alias for WithRedisTransaction
func (r *RedisProcessorRepo) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.WithRedisTransaction(ctx, fn)
}

// GetDB implements repository.DBRepoInterface for compatibility
// Returns nil since Redis repositories don't use GORM
func (r *RedisProcessorRepo) GetDB(ctx context.Context) *gorm.DB {
	return nil
}

// GetRedisClient returns the Redis client from context or the default client
func (r *RedisProcessorRepo) GetRedisClient(ctx context.Context) *redis.Client {
	// Note: In Redis transactions, we still use the same client
	// The transaction context ensures proper WATCH/MULTI/EXEC semantics
	return r.client
}

// GetProcessor retrieves a processor by ID
func (r *RedisProcessorRepo) GetProcessor(ctx context.Context, processorID string, withChainState bool) (*models.Processor, error) {
	key := processorKeyPrefix + processorID
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("processor not found: %s", processorID)
	}
	if err != nil {
		return nil, err
	}

	var processor models.Processor
	if err := json.Unmarshal([]byte(data), &processor); err != nil {
		return nil, err
	}

	if withChainState {
		chainStates, err := r.getChainStatesByProcessor(ctx, processorID)
		if err != nil {
			return nil, err
		}
		processor.ChainStates = chainStates
	}

	return &processor, nil
}

// GetProcessors retrieves all processors for a project
func (r *RedisProcessorRepo) GetProcessors(ctx context.Context, projectID string) ([]models.Processor, error) {
	pattern := processorProjectKeyPrefix + projectID + ":*"
	var processors []models.Processor

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		data, err := r.client.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}

		var processor models.Processor
		if err := json.Unmarshal([]byte(data), &processor); err != nil {
			continue
		}
		processors = append(processors, processor)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return processors, nil
}

// SaveProcessor saves a processor to Redis
func (r *RedisProcessorRepo) SaveProcessor(ctx context.Context, processor *models.Processor) error {
	data, err := json.Marshal(processor)
	if err != nil {
		return err
	}

	key := processorKeyPrefix + processor.ID
	projectKey := processorProjectKeyPrefix + processor.ProjectID + ":" + processor.ID

	pipe := r.client.Pipeline()
	pipe.Set(ctx, key, data, 0)
	pipe.Set(ctx, projectKey, data, 0)

	// Save chain states separately
	if processor.ChainStates != nil {
		for _, cs := range processor.ChainStates {
			csData, err := json.Marshal(cs)
			if err != nil {
				return err
			}
			csKey := chainStateKeyPrefix + cs.ProcessorID + ":" + cs.ChainID
			pipe.Set(ctx, csKey, csData, 0)
			pipe.SAdd(ctx, chainStatesKeyPrefix+processor.ID, cs.ChainID)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

// RemoveProcessor removes a processor from Redis
func (r *RedisProcessorRepo) RemoveProcessor(ctx context.Context, processorID string) error {
	key := processorKeyPrefix + processorID

	// Get processor to find project ID
	processor, err := r.GetProcessor(ctx, processorID, false)
	if err != nil {
		return err
	}

	projectKey := processorProjectKeyPrefix + processor.ProjectID + ":" + processorID

	pipe := r.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, projectKey)
	pipe.Del(ctx, chainStatesKeyPrefix+processorID)

	// Remove chain states
	chainStatePattern := chainStateKeyPrefix + processorID + ":*"
	iter := r.client.Scan(ctx, 0, chainStatePattern, 0).Iterator()
	for iter.Next(ctx) {
		pipe.Del(ctx, iter.Val())
	}

	_, err = pipe.Exec(ctx)
	return err
}

// ObsoleteProcessor marks a processor as obsolete
func (r *RedisProcessorRepo) ObsoleteProcessor(ctx context.Context, processorID string) error {
	processor, err := r.GetProcessor(ctx, processorID, false)
	if err != nil {
		return err
	}

	processor.VersionState = int32(protos.ProcessorVersionState_OBSOLETE)
	return r.SaveProcessor(ctx, processor)
}

// PreloadProcessor loads a processor with all its relationships
func (r *RedisProcessorRepo) PreloadProcessor(ctx context.Context, id string) (*models.Processor, error) {
	return r.GetProcessor(ctx, id, true)
}

// FindProcessorByVersion finds a processor by project ID and version
func (r *RedisProcessorRepo) FindProcessorByVersion(ctx context.Context, projectID string, version int32) (*models.Processor, error) {
	processors, err := r.GetProcessors(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for _, p := range processors {
		if p.Version == version {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("processor not found for project %s version %d", projectID, version)
}

// FindActiveProcessor finds the active processor for a project
func (r *RedisProcessorRepo) FindActiveProcessor(ctx context.Context, projectID string) (*models.Processor, error) {
	processors, err := r.GetProcessors(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for _, p := range processors {
		if p.VersionState == int32(protos.ProcessorVersionState_ACTIVE) {
			return &p, nil
		}
	}

	return nil, fmt.Errorf("no active processor found for project %s", projectID)
}

// FindLatestProcessor finds the latest processor for a project
func (r *RedisProcessorRepo) FindLatestProcessor(ctx context.Context, projectID string) (*models.Processor, error) {
	processors, err := r.GetProcessors(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if len(processors) == 0 {
		return nil, fmt.Errorf("no processor found for project %s", projectID)
	}

	// Sort by version descending
	sort.Slice(processors, func(i, j int) bool {
		return processors[i].Version > processors[j].Version
	})

	return &processors[0], nil
}

// FindReplacingProcessor finds a replacing processor for a project
// Note: As of the current proto definition, there is no REPLACING state.
// This method returns nil if no replacing processor is found.
func (r *RedisProcessorRepo) FindReplacingProcessor(ctx context.Context, project *commonmodels.Project) (*models.Processor, error) {
	processors, err := r.GetProcessors(ctx, project.ID)
	if err != nil {
		return nil, err
	}

	// Check for pending processors that might be considered "replacing"
	for _, p := range processors {
		if p.VersionState == int32(protos.ProcessorVersionState_PENDING) {
			// Get active processor
			active, err := r.FindActiveProcessor(ctx, project.ID)
			if err == nil && active != nil && p.Version > active.Version {
				// This pending processor is newer than active, could be replacing
				return &p, nil
			}
		}
	}

	return nil, nil
}

// ResolveReferenceProcessor resolves a reference processor
func (r *RedisProcessorRepo) ResolveReferenceProcessor(ctx context.Context, processor *models.Processor) (*models.Processor, error) {
	if processor.ReferenceProjectID == "" {
		return processor, nil
	}

	// Get the active processor from the referenced project
	return r.FindActiveProcessor(ctx, processor.ReferenceProjectID)
}

// GetProcessorsByProjectAndVersionState gets processors by project and version states
func (r *RedisProcessorRepo) GetProcessorsByProjectAndVersionState(ctx context.Context, projectID string, versionStates ...protos.ProcessorVersionState) ([]*models.Processor, error) {
	processors, err := r.GetProcessors(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var result []*models.Processor
	for _, p := range processors {
		if len(versionStates) == 0 {
			pCopy := p
			result = append(result, &pCopy)
			continue
		}
		for _, state := range versionStates {
			if p.VersionState == int32(state) {
				pCopy := p
				result = append(result, &pCopy)
				break
			}
		}
	}

	return result, nil
}

// GetObsoleteProcessors gets obsolete processors for a project
func (r *RedisProcessorRepo) GetObsoleteProcessors(ctx context.Context, projectID string) ([]models.Processor, error) {
	processors, err := r.GetProcessors(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var obsolete []models.Processor
	for _, p := range processors {
		if p.VersionState == int32(protos.ProcessorVersionState_OBSOLETE) {
			obsolete = append(obsolete, p)
		}
	}

	return obsolete, nil
}

// CreateOrUpdateProcessor creates or updates a processor
func (r *RedisProcessorRepo) CreateOrUpdateProcessor(
	ctx context.Context,
	project *commonmodels.Project,
	continueFrom int32,
	pause bool,
	identity *commonmodels.Identity,
	clickhouseShardingIndex int32,
	k8sClusterID int32,
	numWorkers int32,
	sentioProperties models.SentioProcessorProperties,
	subgraphProperties models.SubgraphProcessorProperties,
) (*models.Processor, error) {
	// If continueFrom > 0, update existing processor
	if continueFrom > 0 {
		existingProcessor, err := r.FindProcessorByVersion(ctx, project.ID, continueFrom)
		if err != nil {
			return nil, fmt.Errorf("processor with version %d not found for project %s: %w", continueFrom, project.ID, err)
		}

		// Update the properties
		existingProcessor.NumWorkers = numWorkers
		existingProcessor.SentioProcessorProperties = sentioProperties
		existingProcessor.SubgraphProcessorProperties = subgraphProperties
		existingProcessor.UploadedAt = time.Now()

		// Update user if identity is provided
		if identity != nil {
			existingProcessor.UserID = &identity.UserID
		}

		return existingProcessor, r.SaveProcessor(ctx, existingProcessor)
	}

	// Create new processor
	// Find the latest version to determine the new version number
	latestProcessor, err := r.FindLatestProcessor(ctx, project.ID)
	newVersion := int32(1)
	if err == nil && latestProcessor != nil {
		newVersion = latestProcessor.Version + 1
	}

	// Generate processor ID using gonanoid
	processorID, err := gonanoid.GenerateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate processor ID: %w", err)
	}

	processor := &models.Processor{
		ID:                          processorID,
		ProjectID:                   project.ID,
		Project:                     project,
		Version:                     newVersion,
		UploadedAt:                  time.Now(),
		VersionState:                int32(protos.ProcessorVersionState_PENDING),
		ClickhouseShardingIndex:     clickhouseShardingIndex,
		K8sClusterID:                k8sClusterID,
		Pause:                       pause,
		SentioProcessorProperties:   sentioProperties,
		SubgraphProcessorProperties: subgraphProperties,
		DriverVersion:               1,
		NumWorkers:                  numWorkers,
		EntitySchemaVersion:         0,
	}

	if identity != nil {
		processor.UserID = &identity.UserID
	}

	return processor, r.SaveProcessor(ctx, processor)
}

// ListProcessorUpgradeHistory lists processor upgrade history
func (r *RedisProcessorRepo) ListProcessorUpgradeHistory(ctx context.Context, processorID string) ([]models.ProcessorUpgradeHistory, error) {
	pattern := upgradeHistoryKeyPrefix + processorID + ":*"
	var history []models.ProcessorUpgradeHistory

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		data, err := r.client.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}

		var h models.ProcessorUpgradeHistory
		if err := json.Unmarshal([]byte(data), &h); err != nil {
			continue
		}
		history = append(history, h)
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	// Sort by uploaded time
	sort.Slice(history, func(i, j int) bool {
		return history[i].UploadedAt.After(history[j].UploadedAt)
	})

	return history, nil
}

// GetProcessorUpgradeHistoryByID gets a specific upgrade history entry
func (r *RedisProcessorRepo) GetProcessorUpgradeHistoryByID(ctx context.Context, historyID string, processorID string) (*models.ProcessorUpgradeHistory, error) {
	key := upgradeHistoryKeyPrefix + processorID + ":" + historyID
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("upgrade history not found: %s", historyID)
	}
	if err != nil {
		return nil, err
	}

	var history models.ProcessorUpgradeHistory
	if err := json.Unmarshal([]byte(data), &history); err != nil {
		return nil, err
	}

	return &history, nil
}

// SaveProcessorUpgradeHistory saves processor upgrade history
func (r *RedisProcessorRepo) SaveProcessorUpgradeHistory(ctx context.Context, processor *models.Processor) error {
	history := &models.ProcessorUpgradeHistory{
		ID:                          fmt.Sprintf("hist_%d", time.Now().UnixNano()),
		ProcessorID:                 processor.ID,
		UserID:                      processor.UserID,
		UploadedAt:                  processor.UploadedAt,
		ObsoleteAt:                  time.Now(),
		SentioProcessorProperties:   processor.SentioProcessorProperties,
		SubgraphProcessorProperties: processor.SubgraphProcessorProperties,
	}

	data, err := json.Marshal(history)
	if err != nil {
		return err
	}

	key := upgradeHistoryKeyPrefix + processor.ID + ":" + history.ID
	return r.client.Set(ctx, key, data, 0).Err()
}

// GetProjectByID gets a project by ID
func (r *RedisProcessorRepo) GetProjectByID(ctx context.Context, projectID string) (*commonmodels.Project, error) {
	key := projectKeyPrefix + projectID
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
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

// GetProjectVersions gets all processor versions for a project
func (r *RedisProcessorRepo) GetProjectVersions(ctx context.Context, projectID string) ([]*models.Processor, error) {
	processors, err := r.GetProcessors(ctx, projectID)
	if err != nil {
		return nil, err
	}

	result := make([]*models.Processor, len(processors))
	for i := range processors {
		result[i] = &processors[i]
	}

	// Sort by version
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

// GetProjectVariables gets project variables
func (r *RedisProcessorRepo) GetProjectVariables(ctx context.Context, projectID string) ([]*commonmodels.ProjectVariable, error) {
	key := projectVariablesKeyPrefix + projectID
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return []*commonmodels.ProjectVariable{}, nil
	}
	if err != nil {
		return nil, err
	}

	var variables []*commonmodels.ProjectVariable
	if err := json.Unmarshal([]byte(data), &variables); err != nil {
		return nil, err
	}

	return variables, nil
}

// PreLoadProject preloads a project with owner and slug
func (r *RedisProcessorRepo) PreLoadProject(ctx context.Context, owner, slug string) (*commonmodels.Project, error) {
	// In Redis, we need to scan for projects and match by owner/slug
	// This is not efficient - in production you'd want an index
	pattern := projectKeyPrefix + "*"

	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		data, err := r.client.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}

		var project commonmodels.Project
		if err := json.Unmarshal([]byte(data), &project); err != nil {
			continue
		}

		if project.Slug == slug && project.GetOwnerName() == owner {
			return &project, nil
		}
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return nil, fmt.Errorf("project not found: %s/%s", owner, slug)
}

// Helper method to get chain states by processor
func (r *RedisProcessorRepo) getChainStatesByProcessor(ctx context.Context, processorID string) ([]*models.ChainState, error) {
	chainIDs, err := r.client.SMembers(ctx, chainStatesKeyPrefix+processorID).Result()
	if err != nil {
		return nil, err
	}

	var chainStates []*models.ChainState
	for _, chainID := range chainIDs {
		key := chainStateKeyPrefix + processorID + ":" + chainID
		data, err := r.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var cs models.ChainState
		if err := json.Unmarshal([]byte(data), &cs); err != nil {
			continue
		}
		chainStates = append(chainStates, &cs)
	}

	return chainStates, nil
}

// RedisChainStateRepo implements RedisChainStateRepoInterface using Redis as the storage backend
type RedisChainStateRepo struct {
	client *redis.Client
}

// Ensure RedisChainStateRepo implements RedisChainStateRepoInterface
var _ RedisChainStateRepoInterface = (*RedisChainStateRepo)(nil)

// NewRedisChainStateRepo creates a new Redis-based chain state repository
func NewRedisChainStateRepo(client *redis.Client) *RedisChainStateRepo {
	return &RedisChainStateRepo{
		client: client,
	}
}

// WithRedisTransaction implements transaction support for chain state repo
func (r *RedisChainStateRepo) WithRedisTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(redisTxKey{}).(*redis.Tx); ok {
		return fn(ctx)
	}

	err := r.client.Watch(ctx, func(tx *redis.Tx) error {
		ctx = context.WithValue(ctx, redisTxKey{}, tx)
		return fn(ctx)
	})

	return err
}

// WithTransaction implements repository.DBRepoInterface for compatibility
// This is an alias for WithRedisTransaction
func (r *RedisChainStateRepo) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.WithRedisTransaction(ctx, fn)
}

// GetDB implements repository.DBRepoInterface for compatibility
// Returns nil since Redis repositories don't use GORM
func (r *RedisChainStateRepo) GetDB(ctx context.Context) *gorm.DB {
	return nil
}

// GetRedisClient returns the Redis client
func (r *RedisChainStateRepo) GetRedisClient(ctx context.Context) *redis.Client {
	// Note: In Redis transactions, we still use the same client
	// The transaction context ensures proper WATCH/MULTI/EXEC semantics
	return r.client
}

// GetChainState retrieves a chain state for a specific processor and chain
func (r *RedisChainStateRepo) GetChainState(ctx context.Context, processorID string, chainID string) (models.ChainState, error) {
	key := chainStateKeyPrefix + processorID + ":" + chainID
	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return models.ChainState{}, fmt.Errorf("chain state not found: %s/%s", processorID, chainID)
	}
	if err != nil {
		return models.ChainState{}, err
	}

	var chainState models.ChainState
	if err := json.Unmarshal([]byte(data), &chainState); err != nil {
		return models.ChainState{}, err
	}

	return chainState, nil
}

// GetChainStates retrieves all chain states for a processor
func (r *RedisChainStateRepo) GetChainStates(ctx context.Context, processorID string) ([]models.ChainState, error) {
	chainIDs, err := r.client.SMembers(ctx, chainStatesKeyPrefix+processorID).Result()
	if err != nil {
		return nil, err
	}

	var chainStates []models.ChainState
	for _, chainID := range chainIDs {
		cs, err := r.GetChainState(ctx, processorID, chainID)
		if err != nil {
			continue
		}
		chainStates = append(chainStates, cs)
	}

	return chainStates, nil
}

// UpdateChainState updates a chain state
func (r *RedisChainStateRepo) UpdateChainState(ctx context.Context, chainState *models.ChainState) error {
	data, err := json.Marshal(chainState)
	if err != nil {
		return err
	}

	key := chainStateKeyPrefix + chainState.ProcessorID + ":" + chainState.ChainID

	pipe := r.client.Pipeline()
	pipe.Set(ctx, key, data, 0)
	pipe.SAdd(ctx, chainStatesKeyPrefix+chainState.ProcessorID, chainState.ChainID)
	_, err = pipe.Exec(ctx)

	return err
}

// DeleteChainStatesByProcessor deletes all chain states for a processor
func (r *RedisChainStateRepo) DeleteChainStatesByProcessor(ctx context.Context, processorID string) error {
	chainIDs, err := r.client.SMembers(ctx, chainStatesKeyPrefix+processorID).Result()
	if err != nil {
		return err
	}

	pipe := r.client.Pipeline()
	for _, chainID := range chainIDs {
		key := chainStateKeyPrefix + processorID + ":" + chainID
		pipe.Del(ctx, key)
	}
	pipe.Del(ctx, chainStatesKeyPrefix+processorID)

	_, err = pipe.Exec(ctx)
	return err
}

// ListChainsByProjects lists chains grouped by project
func (r *RedisChainStateRepo) ListChainsByProjects(ctx context.Context, projectIDList []string, versionStatList []protos.ProcessorVersionState) (map[string][]string, error) {
	result := make(map[string][]string)

	for _, projectID := range projectIDList {
		// Get all processors for this project
		pattern := processorProjectKeyPrefix + projectID + ":*"
		iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()

		chainsSet := make(map[string]bool)

		for iter.Next(ctx) {
			data, err := r.client.Get(ctx, iter.Val()).Result()
			if err != nil {
				continue
			}

			var processor models.Processor
			if err := json.Unmarshal([]byte(data), &processor); err != nil {
				continue
			}

			// Check if processor version state matches
			matchesState := false
			for _, state := range versionStatList {
				if processor.VersionState == int32(state) {
					matchesState = true
					break
				}
			}

			if !matchesState {
				continue
			}

			// Get chain states for this processor
			chainIDs, err := r.client.SMembers(ctx, chainStatesKeyPrefix+processor.ID).Result()
			if err != nil {
				continue
			}

			for _, chainID := range chainIDs {
				chainsSet[chainID] = true
			}
		}

		if err := iter.Err(); err != nil {
			return nil, err
		}

		// Convert set to slice
		chains := make([]string, 0, len(chainsSet))
		for chain := range chainsSet {
			chains = append(chains, chain)
		}
		sort.Strings(chains)

		if len(chains) > 0 {
			result[projectID] = chains
		}
	}

	return result, nil
}
