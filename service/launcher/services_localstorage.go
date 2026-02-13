package launcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"

	"sentioxyz/sentio-core/common/log"
)

// LocalStorageService implements the Service interface for local storage HTTP endpoints
type LocalStorageService struct{}

// NewLocalStorageService creates a new local storage service factory
func NewLocalStorageService() Service {
	return &LocalStorageService{}
}

// Create creates a new local storage service instance
func (ls *LocalStorageService) Create(name string, serviceConfig *ServiceConfig, sharedConfig *SharedConfig) (ServiceInstance, error) {
	return &LocalStorageServiceInstance{
		name:          name,
		serviceConfig: serviceConfig,
		sharedConfig:  sharedConfig,
		status:        StatusStopped,
	}, nil
}

// LocalStorageServiceInstance represents a running local storage service instance
type LocalStorageServiceInstance struct {
	name          string
	serviceConfig *ServiceConfig
	sharedConfig  *SharedConfig
	status        ServiceStatus
	mutex         sync.RWMutex
}

// Initialize initializes the local storage service
func (lsi *LocalStorageServiceInstance) Initialize(ctx context.Context) error {
	lsi.mutex.Lock()
	defer lsi.mutex.Unlock()

	if lsi.status == StatusRunning {
		return fmt.Errorf("service %s is already initialized", lsi.name)
	}

	lsi.status = StatusStarting
	log.Infof("Initializing local storage service %s", lsi.name)

	// Get storage path from shared config
	localStoragePath := lsi.sharedConfig.Storage.LocalStoragePath
	if localStoragePath == "" {
		return fmt.Errorf("local_storage_path is required for localstorage service")
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(localStoragePath, 0755); err != nil {
		return fmt.Errorf("failed to create local storage directory %s: %v", localStoragePath, err)
	}

	lsi.status = StatusStopped
	log.Infof("%s initialized with path: %s", lsi.name, localStoragePath)

	return nil
}

// Register registers the local storage HTTP endpoints on the HTTP mux
func (lsi *LocalStorageServiceInstance) Register(grpcServer *grpc.Server, mux *runtime.ServeMux, httpPort int) error {
	lsi.mutex.Lock()
	defer lsi.mutex.Unlock()

	// Register upload handler using HandlePath
	// The grpc-gateway mux needs a pattern-based registration
	// Support both POST and PUT for upload
	err := mux.HandlePath("POST", "/upload/**", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		lsi.handleUpload(w, r)
	})
	if err != nil {
		return fmt.Errorf("failed to register upload POST handler: %v", err)
	}
	err = mux.HandlePath("PUT", "/upload/**", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		lsi.handleUpload(w, r)
	})
	if err != nil {
		return fmt.Errorf("failed to register upload PUT handler: %v", err)
	}

	// Register download handler
	err = mux.HandlePath("GET", "/download/**", func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		lsi.handleDownload(w, r)
	})
	if err != nil {
		return fmt.Errorf("failed to register download handler: %v", err)
	}

	log.Infof("Local storage service %s registered HTTP endpoints on port %d", lsi.name, httpPort)
	return nil
}

// Start starts any background processes (none needed for local storage)
func (lsi *LocalStorageServiceInstance) Start(ctx context.Context) error {
	lsi.mutex.Lock()
	defer lsi.mutex.Unlock()

	if lsi.status == StatusRunning {
		return fmt.Errorf("service %s is already running", lsi.name)
	}

	lsi.status = StatusRunning
	log.Infof("%s started", lsi.name)

	return nil
}

// Stop stops the local storage service
func (lsi *LocalStorageServiceInstance) Stop(ctx context.Context) error {
	lsi.mutex.Lock()
	defer lsi.mutex.Unlock()

	if lsi.status == StatusStopped {
		return nil
	}

	lsi.status = StatusStopping
	log.Infof("Stopping service %s", lsi.name)

	lsi.status = StatusStopped
	log.Infof("%s stopped", lsi.name)

	return nil
}

// Status returns the current status of the service
func (lsi *LocalStorageServiceInstance) Status() string {
	lsi.mutex.RLock()
	defer lsi.mutex.RUnlock()
	return string(lsi.status)
}

// Name returns the name of the service instance
func (lsi *LocalStorageServiceInstance) Name() string {
	return lsi.name
}

// Type returns the type of the service
func (lsi *LocalStorageServiceInstance) Type() string {
	return "localstorage"
}

// handleUpload handles file upload requests
func (lsi *LocalStorageServiceInstance) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Parse URL path: /upload/{bucket}/{object}
	path := strings.TrimPrefix(r.URL.Path, "/upload/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "Invalid upload path, expected /upload/{bucket}/{object}", http.StatusBadRequest)
		return
	}
	bucket := parts[0]
	object := parts[1]

	log.Infof("Upload request: bucket=%s, object=%s, method=%s, content-length=%d", bucket, object, r.Method, r.ContentLength)

	// Support both PUT (S3-style) and POST (Multipart)
	var fileReader io.Reader

	if r.Method == http.MethodPut {
		// S3-compatible upload: Body is the file content
		fileReader = r.Body
		defer r.Body.Close()
	} else {
		// Legacy Multipart upload
		// Parse multipart form
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse multipart form: %v", err), http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get file from form: %v", err), http.StatusBadRequest)
			return
		}
		defer file.Close()
		fileReader = file
	}

	// Get base path from storage system
	basePath := lsi.sharedConfig.Storage.LocalStoragePath

	// Create directory path
	bucketPath := filepath.Join(basePath, bucket)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create bucket directory: %v", err), http.StatusInternalServerError)
		return
	}

	destPath := filepath.Join(basePath, bucket, object)
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create destination directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create destination file: %v", err), http.StatusInternalServerError)
		return
	}
	defer destFile.Close()

	// Copy file contents
	if _, err := io.Copy(destFile, fileReader); err != nil {
		http.Error(w, fmt.Sprintf("Failed to write file: %v", err), http.StatusInternalServerError)
		return
	}

	log.Infof("Successfully uploaded file: %s/%s", bucket, object)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "File uploaded successfully: %s/%s\n", bucket, object)
}

// handleDownload handles file download requests
func (lsi *LocalStorageServiceInstance) handleDownload(w http.ResponseWriter, r *http.Request) {
	// Parse URL path: /download/{bucket}/{object}
	path := strings.TrimPrefix(r.URL.Path, "/download/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		http.Error(w, "Invalid download path, expected /download/{bucket}/{object}", http.StatusBadRequest)
		return
	}
	bucket := parts[0]
	object := parts[1]

	log.Infof("Download request: bucket=%s, object=%s", bucket, object)

	// Get base path from storage system
	basePath := lsi.sharedConfig.Storage.LocalStoragePath

	filePath := filepath.Join(basePath, bucket, object)

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		}
		return
	}
	defer file.Close()

	// Get file info for ServeContent
	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to stat file: %v", err), http.StatusInternalServerError)
		return
	}

	// Set Content-Disposition header for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(object)))
	http.ServeContent(w, r, filepath.Base(object), fileInfo.ModTime(), file)

	log.Infof("Successfully served file: %s/%s", bucket, object)
}
