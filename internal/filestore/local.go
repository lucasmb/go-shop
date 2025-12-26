package filestore

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type LocalFileStore struct {
	storagePath string
	baseURL     string
}

func newLocalFileStore(config Config) (*LocalFileStore, error) {
	// Ensure the storage directory exists.
	if err := os.MkdirAll(config.LocalPath, 0755); err != nil {
		return nil, err
	}
	return &LocalFileStore{
		storagePath: config.LocalPath,
		baseURL:     config.BaseURL,
	}, nil
}

func (s *LocalFileStore) Upload(fileHeader *multipart.FileHeader) (string, error) {
	// Open the uploaded file
	src, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Generate a unique filename to prevent collisions
	ext := filepath.Ext(fileHeader.Filename)
	newFilename := fmt.Sprintf("%d-%s%s", time.Now().UnixNano(), uuid.NewString(), ext)

	// Create the destination file on the server
	dstPath := filepath.Join(s.storagePath, newFilename)
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// Copy the uploaded file's content to the destination file
	if _, err = io.Copy(dst, src); err != nil {
		return "", err
	}

	// Return the public URL to the file
	return fmt.Sprintf("%s/%s", s.baseURL, newFilename), nil
}
