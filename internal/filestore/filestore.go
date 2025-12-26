package filestore

import (
	"fmt"
	"mime/multipart"
)

// FileStore defines the interface for our storage backend.
type FileStore interface {
	// Upload takes a file and returns its public URL or an error.
	Upload(file *multipart.FileHeader) (string, error)
	// Delete is not implemented yet but would be here.
}

// Config holds the configuration for creating a filestore.
type Config struct {
	Provider string // "local" or "s3"

	// For Local provider
	LocalPath string // e.g., "./uploads"
	BaseURL   string // e.g., "http://localhost:4000/uploads"

	// For S3 provider
	S3Bucket    string
	S3Region    string
	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
}

// New creates a new FileStore based on the provided configuration.
func New(config Config) (FileStore, error) {
	switch config.Provider {
	case "local":
		return newLocalFileStore(config)
	case "s3":
		// We would implement this next
		// return newS3FileStore(config)
		return nil, fmt.Errorf("S3 provider not yet implemented")
	default:
		return nil, fmt.Errorf("unknown filestore provider: %s", config.Provider)
	}
}
