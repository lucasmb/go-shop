package testutils

import (
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// NewTestDB creates a new in-memory SQLite database for testing.
func NewTestDB(t *testing.T) *sql.DB {
	// Find the project root to locate the schema/seed files
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../..")

	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("Failed to open memory database: %v", err)
	}

	// Read and execute the main schema file.
	schema, err := os.ReadFile(filepath.Join(projectRoot, "schema.sql"))
	if err != nil {
		t.Fatalf("Failed to read schema.sql: %v", err)
	}
	_, err = db.Exec(string(schema))
	if err != nil {
		t.Fatalf("Failed to execute schema.sql: %v", err)
	}

	// Read and execute the DEDICATED test seed file.
	seed, err := os.ReadFile(filepath.Join(projectRoot, "internal/testutils/test_seed.sql"))
	if err != nil {
		t.Fatalf("Failed to read test_seed.sql: %v", err)
	}
	_, err = db.Exec(string(seed))
	if err != nil {
		t.Fatalf("Failed to execute test_seed.sql: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// NewTestLogger returns a logger that discards output for clean test runs.
func NewTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
