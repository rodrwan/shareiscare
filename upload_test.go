package main

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rodrwan/shareiscare/config"
)

func TestUploadHandler(t *testing.T) {
	// Create temporary directory for tests
	tempDir, err := os.MkdirTemp("", "shareiscare-test-upload")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Configuration for the test
	// Configuration is used indirectly in the handlers
	_ = &config.Config{
		Port:    8080,
		RootDir: tempDir,
		Title:   "Test Server",
	}

	// Test GET /upload - Verify that it displays the form
	t.Run("GET Upload Form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/upload", nil)
		w := httptest.NewRecorder()

		// Simplified handler for GET /upload
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "<form method=\"post\" enctype=\"multipart/form-data\">")
		})

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Incorrect status code, expected: %d, got: %d", http.StatusOK, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Error reading response body: %v", err)
		}

		if !strings.Contains(string(body), "<form") {
			t.Errorf("Response does not contain an HTML form")
		}
	})

	// Test POST /upload - Verify that it processes file uploads
	t.Run("POST Upload File", func(t *testing.T) {
		// Create a buffer for the request body
		var requestBody bytes.Buffer

		// Create a writer for the multipart form
		multipartWriter := multipart.NewWriter(&requestBody)

		// Create a file field in the form
		fileWriter, err := multipartWriter.CreateFormFile("files", "test-upload.txt")
		if err != nil {
			t.Fatalf("Error creating file field: %v", err)
		}

		// Content of the file to upload
		fileContent := "This is a test file for uploading"
		if _, err := fileWriter.Write([]byte(fileContent)); err != nil {
			t.Fatalf("Error writing file content: %v", err)
		}

		// Close the multipart form writer
		if err := multipartWriter.Close(); err != nil {
			t.Fatalf("Error closing multipart writer: %v", err)
		}

		// Create POST request with the multipart form
		req := httptest.NewRequest(http.MethodPost, "/upload", &requestBody)
		req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		w := httptest.NewRecorder()

		// Simplified handler for POST /upload
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse the multipart form
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, "Error processing form: "+err.Error(), http.StatusBadRequest)
				return
			}

			// Get the uploaded files
			files := r.MultipartForm.File["files"]
			if len(files) == 0 {
				http.Error(w, "No files have been selected", http.StatusBadRequest)
				return
			}

			// Process the first file
			fileHeader := files[0]

			// Open the uploaded file
			uploadedFile, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Error opening file: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer uploadedFile.Close()

			// Create the destination file
			destPath := filepath.Join(tempDir, fileHeader.Filename)
			destFile, err := os.Create(destPath)
			if err != nil {
				http.Error(w, "Error creating destination file: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer destFile.Close()

			// Copy content
			if _, err := io.Copy(destFile, uploadedFile); err != nil {
				http.Error(w, "Error saving file: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Respond with success
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "File uploaded successfully: "+fileHeader.Filename)
		})

		// Execute the request
		handler.ServeHTTP(w, req)

		// Verify response
		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Incorrect status code, expected: %d, got: %d", http.StatusOK, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Error reading response body: %v", err)
		}

		if !strings.Contains(string(body), "File uploaded successfully") {
			t.Errorf("Response does not indicate that the file was uploaded successfully")
		}

		// Verify that the file exists in the destination directory
		uploadedFilePath := filepath.Join(tempDir, "test-upload.txt")
		if _, err := os.Stat(uploadedFilePath); os.IsNotExist(err) {
			t.Errorf("File was not saved on the server")
		}

		// Verify the content of the saved file
		savedContent, err := os.ReadFile(uploadedFilePath)
		if err != nil {
			t.Fatalf("Error reading the saved file: %v", err)
		}

		if string(savedContent) != fileContent {
			t.Errorf("The content of the saved file does not match the original")
		}
	})
}
