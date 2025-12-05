package storage

import (
	"context"
	"fmt"
	"net/url"
	"sentioxyz/sentio-core/service/common/storagesystem"
	"strings"

	"github.com/pkg/errors"
)

// DefaultIPFSStorageSystem implements FileStorageSystemInterface for IPFS
// In this implementation:
// - Bucket = CID (directory CID in IPFS)
// - Object = file path within the directory (e.g., "path/to/file.json")
type DefaultIPFSStorageSystem struct {
	config        IPFSConfig
	defaultEngine *IPFSStorageEngine
}

// DefaultIPFSStorageSystemConfig configures the IPFS storage system
type DefaultIPFSStorageSystemConfig struct {
	IPFSConfig IPFSConfig
}

// NewDefaultIPFSStorageSystem creates a new IPFS storage system
func NewDefaultIPFSStorageSystem(config DefaultIPFSStorageSystemConfig) (*DefaultIPFSStorageSystem, error) {
	engine, err := NewIPFSStorageEngine(config.IPFSConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create default IPFS engine")
	}

	return &DefaultIPFSStorageSystem{
		config:        config.IPFSConfig,
		defaultEngine: engine,
	}, nil
}

// CreateDefaultStorage creates and returns the default IPFS storage engine
func (s *DefaultIPFSStorageSystem) CreateDefaultStorage(ctx context.Context) (storagesystem.FileStorageEngine, error) {
	// Return a new engine instance
	engine, err := NewIPFSStorageEngine(s.config)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create IPFS storage engine")
	}
	return engine, nil
}

// GetUploadFileByID retrieves a file object by ID
// The fileID should be in the format "{cid}" or "{cid}/path/to/file"
// - If fileID contains "/", the first part is used as bucket (CID) and the rest as object (path)
// - If fileID doesn't contain "/", it's used as the bucket (CID) with empty object
func (s *DefaultIPFSStorageSystem) GetUploadFileByID(engine storagesystem.FileStorageEngine, fileID string) *storagesystem.FileObject {
	if engine == nil {
		engine = s.defaultEngine
	}

	// Parse fileID to extract CID (bucket) and path (object)
	bucket, object := s.parseFileID(fileID)

	return &storagesystem.FileObject{
		Engine:      engine,
		Bucket:      bucket, // CID
		Object:      object, // Path within the CID directory
		ContentType: "",     // Unknown at this point
	}
}

// NewUploadFileWithEngine creates a new file object for upload with a specific engine
// The fileId can be in the format "{cid}/path" where CID is the bucket and path is the object
func (s *DefaultIPFSStorageSystem) NewUploadFileWithEngine(engine storagesystem.FileStorageEngine, fileId string, contentType string) *storagesystem.FileObject {
	if engine == nil {
		engine = s.defaultEngine
	}

	// Parse fileId to extract CID (bucket) and path (object)
	bucket, object := s.parseFileID(fileId)

	return &storagesystem.FileObject{
		Engine:      engine,
		Bucket:      bucket, // CID
		Object:      object, // Path within the CID directory
		ContentType: contentType,
	}
}

// FinalizeUpload finalizes an upload
// The fileID should be in format "{cid}" or "{cid}/path"
func (s *DefaultIPFSStorageSystem) FinalizeUpload(ctx context.Context, fileID string, storage storagesystem.FileStorageEngine) (*storagesystem.FileObject, error) {
	if storage == nil {
		storage = s.defaultEngine
	}

	// Parse fileID to extract CID (bucket) and path (object)
	bucket, object := s.parseFileID(fileID)

	// Check if the CID exists
	exists, err := storage.ObjectExists(ctx, bucket, object)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to verify file existence")
	}

	if !exists {
		return nil, fmt.Errorf("file %s does not exist", fileID)
	}

	return &storagesystem.FileObject{
		Engine:      storage,
		Bucket:      bucket, // CID
		Object:      object, // Path within the CID directory
		ContentType: "",     // Unknown after upload
	}, nil
}

// GetFromUrl gets a file object from an IPFS URL
// Supports IPFS URLs like:
// - ipfs://{cid}
// - ipfs://{cid}/{path/to/file}
// Returns a FileObject with Bucket={cid} and Object={path}
func (s *DefaultIPFSStorageSystem) GetFromUrl(ctx context.Context, urlStr string) (*storagesystem.FileObject, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse URL")
	}

	// Check if it's an ipfs:// URL
	if parsedURL.Scheme != "ipfs" {
		return nil, fmt.Errorf("invalid IPFS URL scheme, expected ipfs://")
	}

	// The host is the CID
	cid := parsedURL.Host
	if cid == "" {
		return nil, fmt.Errorf("CID not found in URL")
	}

	// The path is the file path within the CID directory
	filePath := strings.TrimPrefix(parsedURL.Path, "/")

	return &storagesystem.FileObject{
		Engine: s.defaultEngine,
		Bucket: cid, // CID
		Object: filePath,
	}, nil
}

// NewUploadFile creates a new file object for upload using the default engine
// The fileId can be in format "{cid}/path" where CID is bucket and path is object
func (s *DefaultIPFSStorageSystem) NewUploadFile(ctx context.Context, fileId string, contentType string) (*storagesystem.FileObject, error) {
	// Parse fileId to extract CID (bucket) and path (object)
	bucket, object := s.parseFileID(fileId)

	return &storagesystem.FileObject{
		Engine:      s.defaultEngine,
		Bucket:      bucket, // CID
		Object:      object, // Path within the CID directory
		ContentType: contentType,
	}, nil
}

// parseFileID parses a fileID into bucket (CID) and object (path)
// Format: "{cid}" or "{cid}/path/to/file"
// - If fileID contains "/", the first part is bucket (CID) and rest is object (path)
// - If fileID doesn't contain "/", it's used as bucket (CID) with empty object
// - If rootCID is set and fileID doesn't contain "/", rootCID is used as bucket
func (s *DefaultIPFSStorageSystem) parseFileID(fileID string) (bucket, object string) {
	parts := strings.SplitN(fileID, "/", 2)

	if len(parts) == 2 {
		// Format: "{cid}/path/to/file"
		bucket = parts[0]
		object = parts[1]
	} else {
		bucket = fileID
		object = fileID
	}

	return bucket, object
}

// Ensure DefaultIPFSStorageSystem implements FileStorageSystemInterface
var _ storagesystem.FileStorageSystemInterface = (*DefaultIPFSStorageSystem)(nil)
