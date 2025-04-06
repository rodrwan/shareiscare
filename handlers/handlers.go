package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/rodrwan/shareiscare/config"
	"github.com/rodrwan/shareiscare/templates"
)

// Session stores the user's session information
type Session struct {
	Username  string
	Timestamp time.Time
}

// isAuthenticated checks if a user is authenticated via a session cookie
func isAuthenticated(r *http.Request, config *config.Config) bool {
	sessionCookie, err := r.Cookie("session")
	if err != nil {
		return false
	}

	// Validate session format (user:timestamp:signature)
	parts := strings.Split(sessionCookie.Value, ":")
	if len(parts) != 3 {
		return false
	}

	username := parts[0]
	timestamp := parts[1]
	signature := parts[2]

	// Verify the session signature
	expectedSignature := generateSignature(username, timestamp, config.SecretKey)
	if signature != expectedSignature {
		return false
	}

	return true
}

// generateSignature generates a simple signature for the session
func generateSignature(username, timestamp, secretKey string) string {
	// This is a basic implementation. In a real application,
	// it's recommended to use HMAC or another secure cryptographic algorithm.
	data := username + ":" + timestamp + ":" + secretKey
	return fmt.Sprintf("%x", len(data)*31)
}

// createSessionCookie creates a session cookie
func createSessionCookie(username string, config *config.Config) *http.Cookie {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(username, timestamp, config.SecretKey)
	sessionValue := fmt.Sprintf("%s:%s:%s", username, timestamp, signature)

	return &http.Cookie{
		Name:     "session",
		Value:    sessionValue,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600 * 24, // 24 hours
		SameSite: http.SameSiteLaxMode,
	}
}

// requireAuth is a middleware that checks if the user is authenticated
func RequireAuth(next http.HandlerFunc, config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r, config) {
			// Redirect to the login page
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func Index(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(config.RootDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// List of files to exclude
		excludeFiles := map[string]bool{
			"config.yaml":     true,
			"shareiscare":     true,
			"shareiscare.exe": true,
		}

		// Check if the user is authenticated and is admin
		isLoggedIn := isAuthenticated(r, config)
		isAdmin := false
		if isLoggedIn {
			sessionCookie, _ := r.Cookie("session")
			parts := strings.Split(sessionCookie.Value, ":")
			if len(parts) >= 1 {
				isAdmin = parts[0] == config.Username
			}
		}

		var fileInfos []templates.FileInfo
		for _, file := range files {
			// Filter ShareIsCare system files
			if excludeFiles[file.Name()] {
				continue
			}

			filePath := filepath.Join(config.RootDir, file.Name())

			// Get file information
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			// Format size
			size := ""
			if !info.IsDir() {
				bytes := info.Size()
				if bytes < 1024 {
					size = fmt.Sprintf("%d B", bytes)
				} else if bytes < 1024*1024 {
					size = fmt.Sprintf("%.1f KB", float64(bytes)/1024)
				} else {
					size = fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
				}
			} else {
				size = "directory"
			}

			fileInfos = append(fileInfos, templates.FileInfo{
				Name:  file.Name(),
				Path:  file.Name(),
				Size:  size,
				IsDir: info.IsDir(),
				IsAdmin: isAdmin,
			})
		}

		// Check if the user is authenticated
		isLoggedIn = isAuthenticated(r, config)

		// Get the username if authenticated
		username := ""
		if isLoggedIn {
			sessionCookie, _ := r.Cookie("session")
			parts := strings.Split(sessionCookie.Value, ":")
			if len(parts) >= 1 {
				username = parts[0]
			}
		}

		// Create breadcrumbs for navigation
		var breadcrumbs []templates.Breadcrumb
		breadcrumbs = append(breadcrumbs, templates.Breadcrumb{
			Name: "Home",
			Path: "",
		})

		data := templates.IndexData{
			Title:       config.Title,
			Directory:   "",
			Files:       fileInfos,
			Breadcrumbs: breadcrumbs,
		}

		layoutData := templates.LayoutData{
			Title:      config.Title,
			IsLoggedIn: isLoggedIn,
			Username:   username,
		}

		// Render the template with the layout
		component := templates.Index(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Route for downloading files
func Download(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Validate that the file is within the configured directory
		fullPath := filepath.Join(config.RootDir, filename)
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			http.Error(w, "Configuration error", http.StatusInternalServerError)
			return
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "/../") {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Check if the file exists
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		// If it's a directory, create a zip file
		if fileInfo.IsDir() {
			// Set headers for zip download
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", filepath.Base(filename)))
			w.Header().Set("Content-Type", "application/zip")

			// Create a zip writer
			zipWriter := zip.NewWriter(w)
			defer zipWriter.Close()

			// Walk through the directory and add files to the zip
			err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip directories
				if info.IsDir() {
					return nil
				}

				// Create a relative path for the file in the zip
				relPath, err := filepath.Rel(fullPath, path)
				if err != nil {
					return err
				}

				// Create a new file in the zip
				zipFile, err := zipWriter.Create(relPath)
				if err != nil {
					return err
				}

				// Open the file
				file, err := os.Open(path)
				if err != nil {
					return err
				}
				defer file.Close()

				// Copy the file content to the zip
				_, err = io.Copy(zipFile, file)
				return err
			})

			if err != nil {
				log.Printf("Error creating zip file: %v", err)
				http.Error(w, "Error creating zip file", http.StatusInternalServerError)
				return
			}

			return
		}

		// For regular files, serve them directly
		file, err := os.Open(fullPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Configure headers to force download
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filename)))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// Send the file
		_, err = io.Copy(w, file)
		if err != nil {
			log.Printf("Error sending file: %v", err)
		}
	}
}

// Login route (GET)
func Login(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If already authenticated, redirect to the upload page
		if isAuthenticated(r, config) {
			http.Redirect(w, r, "/upload", http.StatusSeeOther)
			return
		}

		data := templates.LoginData{
			Title:        config.Title,
			Username:     "",
			ErrorMessage: "",
		}

		layoutData := templates.LayoutData{
			Title:      config.Title + " - Log in",
			IsLoggedIn: false,
			Username:   "",
		}

		// Render the template with the layout
		component := templates.Login(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Login route (POST)
func LoginPost(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error processing the form", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		// Verify credentials
		if username == config.Username && password == config.Password {
			// Create session cookie
			sessionCookie := createSessionCookie(username, config)
			http.SetCookie(w, sessionCookie)

			// Redirect to the upload page
			http.Redirect(w, r, "/upload", http.StatusSeeOther)
			return
		}

		// Incorrect credentials
		data := templates.LoginData{
			Title:        config.Title,
			Username:     username,
			ErrorMessage: "Incorrect username or password",
		}

		layoutData := templates.LayoutData{
			Title:      config.Title + " - Log in",
			IsLoggedIn: false,
			Username:   "",
		}

		// Render the template with the layout
		component := templates.Login(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Logout route
func Logout(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Delete the session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})

		// Redirect to the main page
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// Route to display the file upload form (GET) - protected
func Upload(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the username
		username := ""
		sessionCookie, _ := r.Cookie("session")
		parts := strings.Split(sessionCookie.Value, ":")
		if len(parts) >= 1 {
			username = parts[0]
		}

		data := templates.UploadData{
			Title:     config.Title,
			Directory: config.RootDir,
			Success:   false,
			Message:   "",
		}

		layoutData := templates.LayoutData{
			Title:      config.Title + " - Upload files",
			IsLoggedIn: true,
			Username:   username,
		}

		// Render the template with the layout
		component := templates.Upload(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Route to process file uploads (POST) - protected
func UploadPost(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit the maximum form size to 32MB
		r.ParseMultipartForm(32 << 20)

		// Get the uploaded files
		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "No files have been selected",
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		// Validation: ensure that the destination directory exists and has permissions
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "Configuration error: " + err.Error(),
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		// Check write permissions
		if _, err := os.Stat(absRoot); err != nil {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "Error accessing destination directory: " + err.Error(),
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		uploadedFiles := []string{}
		var errorMessage string

		// Process each file
		for _, fileHeader := range files {
			// Get the file
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Error opening file: %v", err)
				continue
			}
			defer file.Close()

			// Create the destination
			dst, err := os.Create(filepath.Join(absRoot, fileHeader.Filename))
			if err != nil {
				errorMessage = "Error creating destination file: " + err.Error()
				log.Printf("%s", errorMessage)
				continue
			}
			defer dst.Close()

			// Copy content
			if _, err = io.Copy(dst, file); err != nil {
				errorMessage = "Error saving file: " + err.Error()
				log.Printf("%s", errorMessage)
				continue
			}

			uploadedFiles = append(uploadedFiles, fileHeader.Filename)
		}

		// Prepare response
		data := templates.UploadData{
			Title:     config.Title,
			Directory: config.RootDir,
			Success:   len(uploadedFiles) > 0,
			Message:   "",
		}

		if len(uploadedFiles) > 0 {
			if len(uploadedFiles) == 1 {
				data.Message = "File uploaded successfully: " + uploadedFiles[0]
			} else {
				data.Message = fmt.Sprintf("%d files uploaded successfully", len(uploadedFiles))
			}
		} else if errorMessage != "" {
			data.Message = errorMessage
		} else {
			data.Message = "No files could be processed"
		}

		// Get the username
		username := ""
		sessionCookie, _ := r.Cookie("session")
		parts := strings.Split(sessionCookie.Value, ":")
		if len(parts) >= 1 {
			username = parts[0]
		}

		layoutData := templates.LayoutData{
			Title:      config.Title + " - Upload files",
			IsLoggedIn: true,
			Username:   username,
		}

		// Render the template with the layout
		component := templates.Upload(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Browse handles directory navigation
func Browse(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract the path from the URL
		path := strings.TrimPrefix(r.URL.Path, "/browse/")
		if path == "" {
			// If no path is specified, redirect to the root
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Validate that the path is within the configured directory
		fullPath := filepath.Join(config.RootDir, path)
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			http.Error(w, "Configuration error", http.StatusInternalServerError)
			return
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "/../") {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Check if the path exists and is a directory
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			http.Error(w, "Path not found", http.StatusNotFound)
			return
		}
		if !fileInfo.IsDir() {
			// If it's a file, redirect to download
			http.Redirect(w, r, "/download?filename="+path, http.StatusSeeOther)
			return
		}

		// List files in the directory
		files, err := os.ReadDir(fullPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// List of files to exclude
		excludeFiles := map[string]bool{
			"config.yaml":     true,
			"shareiscare":     true,
			"shareiscare.exe": true,
		}

		// Check if the user is authenticated and is admin
		isLoggedIn := isAuthenticated(r, config)
		isAdmin := false
		if isLoggedIn {
			sessionCookie, _ := r.Cookie("session")
			parts := strings.Split(sessionCookie.Value, ":")
			if len(parts) >= 1 {
				isAdmin = parts[0] == config.Username
			}
		}

		var fileInfos []templates.FileInfo
		for _, file := range files {
			// Filter ShareIsCare system files
			if excludeFiles[file.Name()] {
				continue
			}

			filePath := filepath.Join(fullPath, file.Name())

			// Get file information
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			// Format size
			size := ""
			if !info.IsDir() {
				bytes := info.Size()
				if bytes < 1024 {
					size = fmt.Sprintf("%d B", bytes)
				} else if bytes < 1024*1024 {
					size = fmt.Sprintf("%.1f KB", float64(bytes)/1024)
				} else {
					size = fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
				}
			} else {
				size = "directory"
			}

			// Create relative path for links
			relPath := filepath.Join(path, file.Name())

			fileInfos = append(fileInfos, templates.FileInfo{
				Name:  file.Name(),
				Path:  relPath,
				Size:  size,
				IsDir: info.IsDir(),
				IsAdmin: isAdmin,
			})
		}

		// Get the username if authenticated
		username := ""
		if isLoggedIn {
			sessionCookie, _ := r.Cookie("session")
			parts := strings.Split(sessionCookie.Value, ":")
			if len(parts) >= 1 {
				username = parts[0]
			}
		}

		// Create breadcrumbs for navigation
		var breadcrumbs []templates.Breadcrumb
		breadcrumbs = append(breadcrumbs, templates.Breadcrumb{
			Name: "Home",
			Path: "",
		})

		if path != "" {
			parts := strings.Split(path, "/")
			currentPath := ""
			for _, part := range parts {
				currentPath = filepath.Join(currentPath, part)
				breadcrumbs = append(breadcrumbs, templates.Breadcrumb{
					Name: part,
					Path: currentPath,
				})
			}
		}

		data := templates.IndexData{
			Title:       config.Title,
			Directory:   path,
			Files:       fileInfos,
			Breadcrumbs: breadcrumbs,
		}

		layoutData := templates.LayoutData{
			Title:      config.Title + " - Browse",
			IsLoggedIn: isLoggedIn,
			Username:   username,
		}

		// Render the template with the layout
		component := templates.Index(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// requireAdmin is a middleware that checks if the user is an admin
func RequireAdmin(next http.HandlerFunc, config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionCookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(sessionCookie.Value, ":")
		if len(parts) < 1 {
			http.Error(w, "Invalid session", http.StatusUnauthorized)
			return
		}

		username := parts[0]
		if username != config.Username {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

// Delete handles file deletion
func Delete(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error processing form", http.StatusBadRequest)
			return
		}

		filename := r.FormValue("filename")
		if filename == "" {
			http.Error(w, "Filename is required", http.StatusBadRequest)
			return
		}

		// Validate that the file is within the configured directory
		fullPath := filepath.Join(config.RootDir, filename)
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			http.Error(w, "Configuration error", http.StatusInternalServerError)
			return
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "/../") {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Check if the file exists
		if _, err := os.Stat(fullPath); err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		// Delete the file
		err = os.Remove(fullPath)
		if err != nil {
			http.Error(w, "Error deleting file", http.StatusInternalServerError)
			return
		}

		// Redirect back to the directory
		dir := filepath.Dir(filename)
		if dir == "." {
			http.Redirect(w, r, "/", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/browse/"+dir, http.StatusSeeOther)
		}
	}
}
