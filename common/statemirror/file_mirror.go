package statemirror

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-faster/errors"
)

const (
	defaultFileExtension = ".json"
	defaultScanBatchSize = 1000
)

type FileMirrorOption func(*fileMirror)

// WithBaseDir sets the base directory for storing mirror files.
// Each OnChainKey will be stored as a separate JSON file in this directory.
func WithBaseDir(dir string) FileMirrorOption {
	return func(f *fileMirror) {
		f.baseDir = dir
	}
}

// WithFileExtension sets the file extension for mirror files (default: ".json").
func WithFileExtension(ext string) FileMirrorOption {
	return func(f *fileMirror) {
		f.fileExt = ext
	}
}

type fileMirror struct {
	baseDir  string
	fileExt  string
	mu       sync.RWMutex // protects file operations
	scanSize int
}

// NewFileMirror creates a new file-based mirror that stores data as JSON files.
// Each OnChainKey maps to a separate file containing a map[string]string.
func NewFileMirror(baseDir string, opts ...FileMirrorOption) (Mirror, error) {
	f := &fileMirror{
		baseDir:  baseDir,
		fileExt:  defaultFileExtension,
		scanSize: defaultScanBatchSize,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(f)
		}
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(f.baseDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create base directory")
	}

	return f, nil
}

func (f *fileMirror) filePath(key OnChainKey) string {
	// Sanitize key to be filesystem-safe
	safeKey := strings.ReplaceAll(string(key), "/", "_")
	safeKey = strings.ReplaceAll(safeKey, "\\", "_")
	return filepath.Join(f.baseDir, safeKey+f.fileExt)
}

// readData reads the data from file for the given key
func (f *fileMirror) readData(key OnChainKey) (map[string]string, error) {
	path := f.filePath(key)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, errors.Wrap(err, "failed to read file")
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal data")
	}

	if m == nil {
		return make(map[string]string), nil
	}
	return m, nil
}

// writeData writes the data to file for the given key
func (f *fileMirror) writeData(key OnChainKey, data map[string]string) error {
	path := f.filePath(key)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal data")
	}

	if err := os.WriteFile(path, jsonData, 0644); err != nil {
		return errors.Wrap(err, "failed to write file")
	}

	return nil
}

func (f *fileMirror) Upsert(ctx context.Context, key OnChainKey, syncF SyncFunc) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	desired, err := syncF(ctx, key)
	if err != nil {
		return err
	}

	// Simply write the desired state, replacing any existing data
	return f.writeData(key, desired)
}

func (f *fileMirror) UpsertStreaming(ctx context.Context, key OnChainKey, syncF StreamingSyncFunc) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	seen := make(map[string]struct{})
	data := make(map[string]string)

	emit := func(ctx context.Context, field, value string) error {
		seen[field] = struct{}{}
		data[field] = value
		return nil
	}

	if err := syncF(ctx, key, emit); err != nil {
		return err
	}

	// Write all emitted data
	return f.writeData(key, data)
}

func (f *fileMirror) Apply(ctx context.Context, key OnChainKey, diffF DiffFunc) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	diff, err := diffF(ctx, key)
	if err != nil {
		return err
	}
	if diff == nil {
		return errors.Errorf("diffF returned nil diff")
	}

	existing, err := f.readData(key)
	if err != nil {
		return err
	}

	// Apply deletions
	for _, field := range diff.Deleted {
		delete(existing, field)
	}

	// Apply additions/updates
	for field, value := range diff.Added {
		existing[field] = value
	}

	return f.writeData(key, existing)
}

func (f *fileMirror) Get(ctx context.Context, key OnChainKey, field string) (value string, ok bool, err error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := f.readData(key)
	if err != nil {
		return "", false, err
	}

	value, ok = data[field]
	return value, ok, nil
}

func (f *fileMirror) MGet(ctx context.Context, key OnChainKey, fields ...string) (map[string]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	out := make(map[string]string)
	if len(fields) == 0 {
		return out, nil
	}

	data, err := f.readData(key)
	if err != nil {
		return nil, err
	}

	for _, field := range fields {
		if value, ok := data[field]; ok {
			out[field] = value
		}
	}

	return out, nil
}

func (f *fileMirror) GetAll(ctx context.Context, key OnChainKey) (map[string]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := f.readData(key)
	if err != nil {
		return nil, err
	}

	// Return a copy to avoid concurrent modification issues
	result := make(map[string]string, len(data))
	for k, v := range data {
		result[k] = v
	}

	return result, nil
}

func (f *fileMirror) Scan(ctx context.Context, key OnChainKey, cursor uint64, match string, count int) (
	nextCursor uint64, kv map[string]string, err error,
) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	data, err := f.readData(key)
	if err != nil {
		return 0, nil, err
	}

	if count <= 0 {
		count = f.scanSize
		if count <= 0 {
			count = defaultScanBatchSize
		}
	}

	// Convert map to slice for pagination
	var keys []string
	for k := range data {
		// Apply pattern matching if specified
		if match != "" && !matchPattern(k, match) {
			continue
		}
		keys = append(keys, k)
	}

	// Handle cursor-based pagination
	start := int(cursor)
	if start >= len(keys) {
		return 0, make(map[string]string), nil
	}

	end := start + count
	if end > len(keys) {
		end = len(keys)
	}

	result := make(map[string]string)
	for i := start; i < end; i++ {
		k := keys[i]
		result[k] = data[k]
	}

	var next uint64
	if end < len(keys) {
		next = uint64(end)
	} else {
		next = 0 // No more data
	}

	return next, result, nil
}

// matchPattern performs simple glob-style pattern matching.
// Supports * as wildcard for any characters.
func matchPattern(str, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}

	// Simple implementation: split by * and check if all parts are present in order
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		// No wildcard, exact match
		return str == pattern
	}

	// Check if first part matches start (if not empty)
	if parts[0] != "" && !strings.HasPrefix(str, parts[0]) {
		return false
	}

	// Check if last part matches end (if not empty)
	lastIdx := len(parts) - 1
	if parts[lastIdx] != "" && !strings.HasSuffix(str, parts[lastIdx]) {
		return false
	}

	// Check if middle parts exist in order
	currentPos := len(parts[0])
	for i := 1; i < lastIdx; i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(str[currentPos:], parts[i])
		if idx == -1 {
			return false
		}
		currentPos += idx + len(parts[i])
	}

	return true
}
