package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sentioxyz/sentio-core/service/common/storagesystem"
	"sentioxyz/sentio-core/service/processor/protos"
	"time"

	"github.com/pkg/errors"
)

// LocalStorageEngine implements FileStorageEngine for local filesystem storage
type LocalStorageEngine struct {
	basePath string
	baseURL  string
}

// LocalStorageConfig configures the local storage engine
type LocalStorageConfig struct {
	// BasePath is the root directory for file storage
	BasePath string
	// BaseURL is the base HTTP URL for the storage service (e.g., "http://localhost:10000")
	BaseURL string
}

// NewLocalStorageEngine creates a new local storage engine
func NewLocalStorageEngine(config LocalStorageConfig) (*LocalStorageEngine, error) {
	if config.BasePath == "" {
		return nil, fmt.Errorf("base path cannot be empty")
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(config.BasePath, 0755); err != nil {
		return nil, errors.Wrapf(err, "failed to create base storage directory")
	}

	baseURL := config.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:10000" // Default base URL
	}

	return &LocalStorageEngine{
		basePath: config.BasePath,
		baseURL:  baseURL,
	}, nil
}

// PreSignedPutUrl returns a URL for uploading a file
func (e *LocalStorageEngine) PreSignedPutUrl(ctx context.Context, bucket, object, contentType string, expireDuration time.Duration) (string, error) {
	return fmt.Sprintf("%s/upload/%s/%s", e.baseURL, bucket, object), nil
}

// PreSignedGetUrl returns a URL for downloading a file
func (e *LocalStorageEngine) PreSignedGetUrl(ctx context.Context, bucket, object string, expireDuration time.Duration) (string, error) {
	return fmt.Sprintf("%s/download/%s/%s", e.baseURL, bucket, object), nil
}

// CopyFile copies a file from source to destination within local storage
func (e *LocalStorageEngine) CopyFile(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) error {
	srcPath := filepath.Join(e.basePath, srcBucket, srcObject)
	destPath := filepath.Join(e.basePath, destBucket, destObject)

	// Create destination directory
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create destination directory")
	}

	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open source file")
	}
	defer srcFile.Close()

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create destination file")
	}
	defer destFile.Close()

	// Copy contents
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return errors.Wrapf(err, "failed to copy file contents")
	}

	return nil
}

// GetUrl returns the URL for accessing the file
func (e *LocalStorageEngine) GetUrl(ctx context.Context, bucket, object string) string {
	return fmt.Sprintf("local://%s/%s", bucket, object)
}

// Delete removes a file from local storage
func (e *LocalStorageEngine) Delete(ctx context.Context, bucket, object string) error {
	filePath := filepath.Join(e.basePath, bucket, object)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, consider it deleted
		}
		return errors.Wrapf(err, "failed to delete file")
	}

	return nil
}

// UploadLocalFile copies a local file to the storage directory
func (e *LocalStorageEngine) UploadLocalFile(ctx context.Context, bucket, object, contentType, filePath string) (string, error) {
	// Create bucket directory
	bucketPath := filepath.Join(e.basePath, bucket)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		return "", errors.Wrapf(err, "failed to create bucket directory")
	}

	destPath := filepath.Join(e.basePath, bucket, object)
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", errors.Wrapf(err, "failed to create destination directory")
	}

	// Open source file
	srcFile, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open source file")
	}
	defer srcFile.Close()

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create destination file")
	}
	defer destFile.Close()

	// Copy contents
	if _, err := io.Copy(destFile, srcFile); err != nil {
		return "", errors.Wrapf(err, "failed to copy file contents")
	}

	return object, nil
}

// Name returns the name of the storage engine
func (e *LocalStorageEngine) Name() string {
	return "local"
}

// ObjectExists checks if a file exists in local storage
func (e *LocalStorageEngine) ObjectExists(ctx context.Context, bucket string, object string) (bool, error) {
	filePath := filepath.Join(e.basePath, bucket, object)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to check file existence")
	}

	return true, nil
}
func (e *LocalStorageEngine) ToPayload(file *storagesystem.FileObject, fileID, url string, fileType protos.FileType) *protos.UploadPayload {
	return &protos.UploadPayload{
		Payload: &protos.UploadPayload_Object{
			Object: &protos.UploadPayload_ObjectPayload{
				PutUrl:   url,
				Bucket:   file.Bucket,
				ObjectId: file.GetObject(),
				FileId:   fileID,
			},
		},
		FileType: fileType,
	}
}

// Ensure LocalStorageEngine implements FileStorageEngine interface
var _ storagesystem.FileStorageEngine = (*LocalStorageEngine)(nil)
