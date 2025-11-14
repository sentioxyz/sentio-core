package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/storagesystem"
	"strings"
	"time"

	"github.com/pkg/errors"
)

type IPFSStorageEngine struct {
	client     *http.Client
	apiURL     string
	gatewayURL string
}

type IPFSConfig struct {
	// IPFS node HTTP API URL (e.g., "http://127.0.0.1:5001" or "https://ipfs.infura.io:5001")
	ApiURL string
	// Public Gateway URL for retrieving content
	GatewayURL string
}

type ipfsAddResponse struct {
	Name string `json:"Name"`
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

const IPFSPrefix = "ipfs://"

// NewIPFSStorageEngine creates a new IPFS storage engine
func NewIPFSStorageEngine(config IPFSConfig) (*IPFSStorageEngine, error) {
	if config.ApiURL == "" {
		config.ApiURL = "http://127.0.0.1:5001"
	}
	if config.GatewayURL == "" {
		config.GatewayURL = "https://ipfs.io"
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	engine := &IPFSStorageEngine{
		client:     client,
		apiURL:     strings.TrimSuffix(config.ApiURL, "/"),
		gatewayURL: strings.TrimSuffix(config.GatewayURL, "/"),
	}

	// Test connection
	resp, err := client.Post(engine.apiURL+"/api/v0/version", "application/json", nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to IPFS node at %s", config.ApiURL)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IPFS node returned status %d", resp.StatusCode)
	}

	return engine, nil
}

func (e *IPFSStorageEngine) PreSignedPutUrl(ctx context.Context, bucket, object, contentType string, expireDuration time.Duration) (string, error) {
	uploadURL := fmt.Sprintf("%s/api/v0/add", e.gatewayURL)

	return uploadURL, nil
}

func (e *IPFSStorageEngine) PreSignedGetUrl(ctx context.Context, bucket, object string, expireDuration time.Duration) (string, error) {
	if object == "" {
		return "", fmt.Errorf("object (CID) cannot be empty")
	}

	return fmt.Sprintf("%s/ipfs/%s/%s", e.gatewayURL, bucket, object), nil
}

func (e *IPFSStorageEngine) CopyFile(ctx context.Context, srcBucket, srcObject, destBucket, destObject string) error {
	log.Infof("IPFS CopyFile called (no-op): %s/%s -> %s/%s", srcBucket, srcObject, destBucket, destObject)
	return nil
}

// GetUrl returns the public URL for accessing the object
func (e *IPFSStorageEngine) GetUrl(ctx context.Context, bucket, object string) string {
	if bucket != "" {
		return fmt.Sprintf("%s%s/%s", IPFSPrefix, bucket, object)
	}
	// object is the CID
	return fmt.Sprintf("%s/%s", IPFSPrefix, object)

}

// Delete removes the object from IPFS (unpins it)
func (e *IPFSStorageEngine) Delete(ctx context.Context, bucket, object string) error {
	if object == "" {
		return fmt.Errorf("object (CID) cannot be empty")
	}

	// Unpin the content
	apiURL := fmt.Sprintf("%s/api/v0/rm?arg=%s&force=true", e.apiURL, url.QueryEscape(object))
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to create remove request")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return errors.Wrapf(err, "failed to remove object %s", object)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove object %s, status %d: %s", object, resp.StatusCode, string(body))
	}

	log.Infof("Successfully removed IPFS object: %s/%s", bucket, object)
	return nil
}

// UploadLocalFile uploads a local file to IPFS
func (e *IPFSStorageEngine) UploadLocalFile(ctx context.Context, bucket, object, contentType, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to open file %s", filePath)
	}
	defer file.Close()

	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return "", errors.Wrapf(err, "failed to create form file")
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", errors.Wrapf(err, "failed to copy file content")
	}

	if err := writer.Close(); err != nil {
		return "", errors.Wrapf(err, "failed to close multipart writer")
	}

	// Build API URL with pin parameter (always enabled)
	apiURL := fmt.Sprintf("%s/api/v0/add?pin=true", e.apiURL)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, body)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create add request")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.client.Do(req)
	if err != nil {
		return "", errors.Wrapf(err, "failed to add file to IPFS")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to add file, status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var addResp ipfsAddResponse
	if err := json.NewDecoder(resp.Body).Decode(&addResp); err != nil {
		return "", errors.Wrapf(err, "failed to decode response")
	}

	log.Infof("Successfully uploaded file to IPFS: %s (CID: %s)", filePath, addResp.Hash)
	return addResp.Hash, nil
}

// Name returns the name of the storage engine
func (e *IPFSStorageEngine) Name() string {
	return "ipfs"
}

// ObjectExists checks if an object exists in IPFS
func (e *IPFSStorageEngine) ObjectExists(ctx context.Context, bucket string, object string) (bool, error) {
	if object == "" {
		return false, fmt.Errorf("object (CID) cannot be empty")
	}

	// Try to stat the object
	apiURL := fmt.Sprintf("%s/api/v0/object/stat?arg=%s", e.apiURL, url.QueryEscape(bucket))
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, nil)
	if err != nil {
		return false, errors.Wrapf(err, "failed to create stat request")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check object existence")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	// Check if it's a "not found" error
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return false, nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("failed to check object existence, status %d: %s", resp.StatusCode, string(bodyBytes))
}

// Ensure IPFSStorageEngine implements FileStorageEngine interface
var _ storagesystem.FileStorageEngine = (*IPFSStorageEngine)(nil)
