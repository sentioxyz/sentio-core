package storagesystem

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sentioxyz/sentio-core/common/log"
	"time"
)

type FileStorageEngine interface {
	PreSignedPutUrl(ctx context.Context, bucket, object, contentType string, expireDuration time.Duration) (string, error)
	PreSignedGetUrl(ctx context.Context, bucket, object string, expireDuration time.Duration) (string, error)
	CopyFile(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) error
	GetUrl(ctx context.Context, bucket, object string) string
	Delete(ctx context.Context, bucket, object string) error
	UploadLocalFile(ctx context.Context, bucket, object, contentType, filePath string) (string, error)
	Name() string
	ObjectExists(ctx context.Context, bucket string, object string) (bool, error)
}

type FileStorageSystemInterface interface {
	CreateDefaultStorage(ctx context.Context, preferEngine string) (FileStorageEngine, error)
	GetUploadFileByID(engine FileStorageEngine, fileID string) *FileObject
	NewUploadFileWithEngine(engine FileStorageEngine, fileId string, contentType string) *FileObject
	FinalizeUpload(ctx context.Context, fileID string, storage FileStorageEngine) (*FileObject, error)
	GetFromUrl(ctx context.Context, url string) (*FileObject, error)
	NewUploadFile(ctx context.Context, fileId string, contentType string) (*FileObject, error)
}

type FileObject struct {
	Engine      FileStorageEngine `json:"-"`
	CacheEngine FileStorageEngine `json:"-"`
	Object      string
	Bucket      string
	ContentType string
}

func (f *FileObject) PreSignedUploadUrl(ctx context.Context, expireDuration time.Duration) (string, error) {
	return f.Engine.PreSignedPutUrl(ctx, f.Bucket, f.Object, f.ContentType, expireDuration)
}

func (f *FileObject) PreSignedDownloadUrl(ctx context.Context, expireDuration time.Duration) (string, error) {
	if f.CacheEngine != nil {
		// If a cache engine is set, check if the Object exists in the cache
		exists, _ := f.CacheEngine.ObjectExists(ctx, f.Bucket, f.Object)
		if exists {
			// If it exists in the cache, return the cached URL
			return f.CacheEngine.PreSignedGetUrl(ctx, f.Bucket, f.Object, expireDuration)
		} else {
			// If it does not exist in the cache, copy it from the main storage
			downloadUrl, err := f.Engine.PreSignedGetUrl(ctx, f.Bucket, f.Object, expireDuration)
			if err != nil {
				return "", fmt.Errorf("failed to get pre-signed download URL: %w", err)
			}

			// Download the file from the URL to a temporary file
			tmpFile, err := os.CreateTemp(os.TempDir(), "cache_download_*")
			if err != nil {
				return "", fmt.Errorf("failed to create temp file: %w", err)
			}

			resp, err := http.Get(downloadUrl)
			if err != nil {
				return "", fmt.Errorf("failed to download file from url: %w", err)
			}
			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
			}
			defer resp.Body.Close()
			defer os.Remove(tmpFile.Name())

			contentType := resp.Header.Get("Content-Type")
			_, err = io.Copy(tmpFile, resp.Body)
			if err != nil {
				return "", fmt.Errorf("failed to write to temp file: %w", err)
			}
			_ = tmpFile.Close()

			// Upload the file to the cache engine
			_, err = f.CacheEngine.UploadLocalFile(ctx, f.Bucket, f.Object, contentType, tmpFile.Name())
			if err != nil {
				return "", fmt.Errorf("failed to upload file to cache engine: %w", err)
			}

			return f.CacheEngine.PreSignedGetUrl(ctx, f.Bucket, f.Object, expireDuration)
		}
	}

	return f.Engine.PreSignedGetUrl(ctx, f.Bucket, f.Object, expireDuration)
}

func (f *FileObject) MoveTo(ctx context.Context, destBucket, destObject string) error {
	if err := f.Engine.CopyFile(ctx, f.Bucket, f.Object, destBucket, destObject); err != nil {
		return err
	}
	return f.Engine.Delete(ctx, f.Bucket, f.Object)
}

func (f *FileObject) GetUrl(ctx context.Context) string {
	return f.Engine.GetUrl(ctx, f.Bucket, f.Object)
}

func (f *FileObject) Delete(ctx context.Context) error {
	if f.CacheEngine != nil {
		exists, _ := f.CacheEngine.ObjectExists(ctx, f.Bucket, f.Object)
		if exists {
			err := f.CacheEngine.Delete(ctx, f.Bucket, f.Object)
			if err != nil {
				log.Errorfe(err, "failed to delete Object from cache storage %s/%s", f.Bucket, f.Object)
			}
		}
	}
	return f.Engine.Delete(ctx, f.Bucket, f.Object)
}

func (f *FileObject) GetObject() string {
	return f.Object
}

func (f *FileObject) GetBucket() string {
	return f.Bucket
}

func (f *FileObject) UploadLocalFile(ctx context.Context, filePath string) error {
	newObject, err := f.Engine.UploadLocalFile(ctx, f.Bucket, f.Object, f.ContentType, filePath)
	if err != nil {
		return err
	}
	if newObject != "" {
		f.Object = newObject
	}
	return nil
}
