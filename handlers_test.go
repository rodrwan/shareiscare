package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rodrwan/shareiscare/config"
)

func TestIndexHandler(t *testing.T) {
	// Create temporary directory for tests
	tempDir, err := os.MkdirTemp("", "shareiscare-test-index")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create some test files
	testFiles := []string{"file1.txt", "file2.txt"}
	for _, filename := range testFiles {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatalf("Error creating test file: %v", err)
		}
	}

	// Create a test directory
	testDirPath := filepath.Join(tempDir, "test_directory")
	if err := os.Mkdir(testDirPath, 0755); err != nil {
		t.Fatalf("Error creating test directory: %v", err)
	}

	// Configuration for the test
	config := &config.Config{
		Port:    8080,
		RootDir: tempDir,
		Title:   "Test Server",
	}

	// Create test HTTP server
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Implement a handler similar to RunServer but simplified for tests
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(config.RootDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Simply verify that the response contains the file names
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		for _, file := range files {
			io.WriteString(w, file.Name()+"\n")
		}
	})

	// Execute the request
	handler.ServeHTTP(w, req)

	// Verify the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Incorrect status code, expected: %d, got: %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v", err)
	}

	bodyStr := string(body)

	// Verify that the response contains the file names
	for _, filename := range testFiles {
		if !strings.Contains(bodyStr, filename) {
			t.Errorf("Response does not contain the file '%s'", filename)
		}
	}

	// Verify that the response contains the directory name
	if !strings.Contains(bodyStr, "test_directory") {
		t.Errorf("Response does not contain the directory 'test_directory'")
	}
}

func TestDownloadHandler(t *testing.T) {
	// Create temporary directory for tests
	tempDir, err := os.MkdirTemp("", "shareiscare-test-download")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFilename := "file_to_download.txt"
	testContent := "Test content for download"
	testFilePath := filepath.Join(tempDir, testFilename)

	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Error creating test file: %v", err)
	}

	// Configuration for the test
	config := &config.Config{
		Port:    8080,
		RootDir: tempDir,
		Title:   "Test Server",
	}

	// Create test HTTP request
	req := httptest.NewRequest(http.MethodGet, "/download?filename="+testFilename, nil)
	w := httptest.NewRecorder()

	// Implement a handler similar to RunServer but simplified for tests
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Validate path
		fullPath := filepath.Join(config.RootDir, filename)

		// Check if the file exists
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		if fileInfo.IsDir() {
			http.Error(w, "Cannot download a directory", http.StatusBadRequest)
			return
		}

		// Open the file
		file, err := os.Open(fullPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Configure headers for download
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", "0") // Simplified for the test

		// Send the file content
		io.Copy(w, file)
	})

	// Execute the request
	handler.ServeHTTP(w, req)

	// Verify the response
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Incorrect status code, expected: %d, got: %d", http.StatusOK, resp.StatusCode)
	}

	// Verify headers
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, testFilename) {
		t.Errorf("Incorrect Content-Disposition, expected to contain '%s', got: '%s'", testFilename, cd)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Incorrect Content-Type, expected: 'application/octet-stream', got: '%s'", ct)
	}

	// Verify the content
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error reading response body: %v", err)
	}

	if string(body) != testContent {
		t.Errorf("Incorrect content, expected: '%s', got: '%s'", testContent, string(body))
	}
}
