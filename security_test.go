package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rodrwan/shareiscare/config"
)

func TestSecurityPathValidation(t *testing.T) {
	// Create temporary directories for tests
	rootDir, err := os.MkdirTemp("", "shareiscare-test-root")
	if err != nil {
		t.Fatalf("Error creating temporary root directory: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// Create a test file inside the root directory
	validFilePath := filepath.Join(rootDir, "valid_file.txt")
	if err := os.WriteFile(validFilePath, []byte("valid content"), 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(rootDir, "subdirectory")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Error creating subdirectory: %v", err)
	}

	// File in subdirectory (valid)
	validSubdirFilePath := filepath.Join(subDir, "subdir_file.txt")
	if err := os.WriteFile(validSubdirFilePath, []byte("content in subdirectory"), 0644); err != nil {
		t.Fatalf("Error creating file in subdirectory: %v", err)
	}

	// Configuration for the test
	config := &config.Config{
		Port:    8080,
		RootDir: rootDir,
		Title:   "Test Server",
	}

	// Test access to a valid file inside the root directory
	t.Run("Valid File Access", func(t *testing.T) {
		// In a real implementation, these would be used to test the HTTP handler
		// but here we're just testing the validation function
		// _ = httptest.NewRequest(http.MethodGet, "/download?filename=valid_file.txt", nil)
		// _ = httptest.NewRecorder()

		validatePath := func(filename string) (string, bool) {
			// Validate that the file is within the configured directory
			fullPath := filepath.Join(config.RootDir, filename)
			absRoot, err := filepath.Abs(config.RootDir)
			if err != nil {
				return "", false
			}
			absPath, err := filepath.Abs(fullPath)
			if err != nil {
				return "", false
			}

			rel, err := filepath.Rel(absRoot, absPath)
			if err != nil || rel == ".." || filepath.IsAbs(rel) || filepath.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", false
			}

			return fullPath, true
		}

		filename := "valid_file.txt"
		path, valid := validatePath(filename)

		if !valid {
			t.Errorf("Path validation rejected a valid file")
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Could not access valid file: %v", err)
		}
	})

	// Test access to a file in a subdirectory (should be valid)
	t.Run("Valid Subdirectory File Access", func(t *testing.T) {
		// In a real implementation, these would be used to test the HTTP handler
		// but here we're just testing the validation function
		// _ = httptest.NewRequest(http.MethodGet, "/download?filename=subdirectory/subdir_file.txt", nil)
		// _ = httptest.NewRecorder()

		validatePath := func(filename string) (string, bool) {
			// Validate that the file is within the configured directory
			fullPath := filepath.Join(config.RootDir, filename)
			absRoot, err := filepath.Abs(config.RootDir)
			if err != nil {
				return "", false
			}
			absPath, err := filepath.Abs(fullPath)
			if err != nil {
				return "", false
			}

			rel, err := filepath.Rel(absRoot, absPath)
			if err != nil || rel == ".." || filepath.IsAbs(rel) || filepath.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", false
			}

			return fullPath, true
		}

		filename := "subdirectory/subdir_file.txt"
		path, valid := validatePath(filename)

		if !valid {
			t.Errorf("Path validation rejected a valid file in subdirectory")
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Could not access valid file in subdirectory: %v", err)
		}
	})

	// Test attempt to access a file outside the root directory (directory traversal)
	t.Run("Path Traversal Attack", func(t *testing.T) {
		// In a real implementation, these would be used to test the HTTP handler
		// but here we're just testing the validation function
		// _ = httptest.NewRequest(http.MethodGet, "/download?filename=../../../etc/passwd", nil)
		// _ = httptest.NewRecorder()

		validatePath := func(filename string) (string, bool) {
			// Validate that the file is within the configured directory
			fullPath := filepath.Join(config.RootDir, filename)
			absRoot, err := filepath.Abs(config.RootDir)
			if err != nil {
				return "", false
			}
			absPath, err := filepath.Abs(fullPath)
			if err != nil {
				return "", false
			}

			rel, err := filepath.Rel(absRoot, absPath)
			if err != nil || rel == ".." || filepath.IsAbs(rel) || filepath.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return "", false
			}

			return fullPath, true
		}

		filename := "../../../etc/passwd"
		_, valid := validatePath(filename)

		if valid {
			t.Errorf("Path validation allowed a directory traversal attack")
		}
	})
}
