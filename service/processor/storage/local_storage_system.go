package storage

import (
	"context"
	"fmt"
	"net/url"
	"sentioxyz/sentio-core/service/common/storagesystem"
	"strings"

	"github.com/pkg/errors"
)

// LocalStorageSystem implements FileStorageSystemInterface for local filesystem storage
type LocalStorageSystem struct {
	config        LocalStorageConfig
	defaultEngine *LocalStorageEngine
}

// LocalStorageSystemConfig configures the local storage system
type LocalStorageSystemConfig struct {
	LocalStorageConfig LocalStorageConfig
}

// NewLocalStorageSystem creates a new local storage system
func NewLocalStorageSystem(config LocalStorageSystemConfig) (*LocalStorageSystem, error) {
	engine, err := NewLocalStorageEngine(config.LocalStorageConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create default local storage engine")
	}

	return &LocalStorageSystem{
		config:        config.LocalStorageConfig,
		defaultEngine: engine,
	}, nil
}

// CreateDefaultStorage creates and returns the default local storage engine
func (s *LocalStorageSystem) CreateDefaultStorage(ctx context.Context, preferEngine string) (storagesystem.FileStorageEngine, error) {
	engine, err := NewLocalStorageEngine(s.config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create local storage engine")
	}
	return engine, nil
}

// GetUploadFileByID retrieves a file object by ID
// The fileID should be in the format "{bucket}/{object}"
func (s *LocalStorageSystem) GetUploadFileByID(engine storagesystem.FileStorageEngine, fileID string) *storagesystem.FileObject {
	if engine == nil {
		engine = s.defaultEngine
	}

	bucket, object := s.parseFileID(fileID)

	return &storagesystem.FileObject{
		Engine:      engine,
		Bucket:      bucket,
		Object:      object,
		ContentType: "",
	}
}

// NewUploadFileWithEngine creates a new file object for upload with a specific engine
func (s *LocalStorageSystem) NewUploadFileWithEngine(engine storagesystem.FileStorageEngine, fileId string, contentType string) *storagesystem.FileObject {
	if engine == nil {
		engine = s.defaultEngine
	}

	bucket, object := s.parseFileID(fileId)

	return &storagesystem.FileObject{
		Engine:      engine,
		Bucket:      bucket,
		Object:      object,
		ContentType: contentType,
	}
}

// FinalizeUpload finalizes an upload
func (s *LocalStorageSystem) FinalizeUpload(ctx context.Context, fileID string, storage storagesystem.FileStorageEngine) (*storagesystem.FileObject, error) {
	if storage == nil {
		storage = s.defaultEngine
	}

	bucket, object := s.parseFileID(fileID)

	// Check if the file exists
	exists, err := storage.ObjectExists(ctx, bucket, object)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to verify file existence")
	}

	if !exists {
		return nil, fmt.Errorf("file %s does not exist", fileID)
	}

	return &storagesystem.FileObject{
		Engine:      storage,
		Bucket:      bucket,
		Object:      object,
		ContentType: "",
	}, nil
}

// GetFromUrl gets a file object from a local URL
// Supports URLs like: local://{bucket}/{object}
func (s *LocalStorageSystem) GetFromUrl(ctx context.Context, urlStr string) (*storagesystem.FileObject, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse URL")
	}

	if parsedURL.Scheme != "local" {
		return nil, fmt.Errorf("invalid local URL scheme, expected local://")
	}

	// Parse bucket and object from URL
	// Format: local://{bucket}/{object}
	fullPath := parsedURL.Host + parsedURL.Path
	bucket, object := s.parseFileID(fullPath)

	return &storagesystem.FileObject{
		Engine: s.defaultEngine,
		Bucket: bucket,
		Object: object,
	}, nil
}

// NewUploadFile creates a new file object for upload using the default engine
func (s *LocalStorageSystem) NewUploadFile(ctx context.Context, fileId string, contentType string) (*storagesystem.FileObject, error) {
	bucket, object := s.parseFileID(fileId)

	return &storagesystem.FileObject{
		Engine:      s.defaultEngine,
		Bucket:      bucket,
		Object:      object,
		ContentType: contentType,
	}, nil
}

// parseFileID parses a fileID into bucket and object
// Format: "{bucket}/{object}" or just "{object}" (uses "default" as bucket)
func (s *LocalStorageSystem) parseFileID(fileID string) (bucket, object string) {
	parts := strings.SplitN(fileID, "/", 2)

	if len(parts) == 2 {
		bucket = parts[0]
		object = parts[1]
	} else {
		bucket = "default"
		object = fileID
	}

	return bucket, object
}

// UpdateBaseURL updates the base URL for the default storage engine
func (s *LocalStorageSystem) UpdateBaseURL(baseURL string) error {
	if s.defaultEngine == nil {
		return fmt.Errorf("default engine not initialized")
	}
	s.defaultEngine.baseURL = baseURL
	return nil
}

// Ensure LocalStorageSystem implements FileStorageSystemInterface
var _ storagesystem.FileStorageSystemInterface = (*LocalStorageSystem)(nil)
