package handlers

import (
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

// Session guarda la información de sesión del usuario
type Session struct {
	Username  string
	Timestamp time.Time
}

// isAuthenticated verifica si un usuario está autenticado mediante una cookie de sesión
func isAuthenticated(r *http.Request, config *config.Config) bool {
	sessionCookie, err := r.Cookie("session")
	if err != nil {
		return false
	}

	// Validar formato de la sesión (usuario:timestamp:firma)
	parts := strings.Split(sessionCookie.Value, ":")
	if len(parts) != 3 {
		return false
	}

	username := parts[0]
	timestamp := parts[1]
	signature := parts[2]

	// Verificar la firma de la sesión
	expectedSignature := generateSignature(username, timestamp, config.SecretKey)
	if signature != expectedSignature {
		return false
	}

	return true
}

// generateSignature genera una firma simple para la sesión
func generateSignature(username, timestamp, secretKey string) string {
	// Esta es una implementación básica. En una aplicación real,
	// se recomienda usar HMAC u otro algoritmo criptográfico seguro.
	data := username + ":" + timestamp + ":" + secretKey
	return fmt.Sprintf("%x", len(data)*31)
}

// createSessionCookie crea una cookie de sesión
func createSessionCookie(username string, config *config.Config) *http.Cookie {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	signature := generateSignature(username, timestamp, config.SecretKey)
	sessionValue := fmt.Sprintf("%s:%s:%s", username, timestamp, signature)

	return &http.Cookie{
		Name:     "session",
		Value:    sessionValue,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600 * 24, // 24 horas
		SameSite: http.SameSiteLaxMode,
	}
}

// requireAuth es un middleware que verifica si el usuario está autenticado
func RequireAuth(next http.HandlerFunc, config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r, config) {
			// Redireccionar a la página de login
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

		// Lista de archivos a excluir
		excludeFiles := map[string]bool{
			"config.yaml":     true,
			"shareiscare":     true,
			"shareiscare.exe": true,
		}

		var fileInfos []templates.FileInfo
		for _, file := range files {
			// Filtrar archivos de sistema de ShareIsCare
			if excludeFiles[file.Name()] {
				continue
			}

			filePath := filepath.Join(config.RootDir, file.Name())

			// Obtener información del archivo
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			// Formatear tamaño
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
				size = "directorio"
			}

			fileInfos = append(fileInfos, templates.FileInfo{
				Name:  file.Name(),
				Path:  file.Name(),
				Size:  size,
				IsDir: info.IsDir(),
			})
		}

		// Verificar si el usuario está autenticado
		isLoggedIn := isAuthenticated(r, config)

		// Obtener el nombre de usuario si está autenticado
		username := ""
		if isLoggedIn {
			sessionCookie, _ := r.Cookie("session")
			parts := strings.Split(sessionCookie.Value, ":")
			if len(parts) >= 1 {
				username = parts[0]
			}
		}

		data := templates.IndexData{
			Title:     config.Title,
			Directory: config.RootDir,
			Files:     fileInfos,
		}

		layoutData := templates.LayoutData{
			Title:      config.Title,
			IsLoggedIn: isLoggedIn,
			Username:   username,
		}

		// Renderizar la plantilla con el layout
		component := templates.Index(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Ruta para descargar archivos
func Download(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Validar que el archivo esté dentro del directorio configurado
		fullPath := filepath.Join(config.RootDir, filename)
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			http.Error(w, "Error de configuración", http.StatusInternalServerError)
			return
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			http.Error(w, "Ruta inválida", http.StatusBadRequest)
			return
		}

		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "/../") {
			http.Error(w, "Acceso denegado", http.StatusForbidden)
			return
		}

		// Verificar si el archivo existe y no es un directorio
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			http.Error(w, "Archivo no encontrado", http.StatusNotFound)
			return
		}
		if fileInfo.IsDir() {
			http.Error(w, "No se puede descargar un directorio", http.StatusBadRequest)
			return
		}

		// Abrir el archivo
		file, err := os.Open(fullPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Configurar las cabeceras para forzar la descarga
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filename)))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// Enviar el archivo
		_, err = io.Copy(w, file)
		if err != nil {
			log.Printf("Error al enviar archivo: %v", err)
		}
	}
}

// Ruta de login (GET)
func Login(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Si ya está autenticado, redirigir a la página de subida
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
			Title:      config.Title + " - Iniciar sesión",
			IsLoggedIn: false,
			Username:   "",
		}

		// Renderizar la plantilla con el layout
		component := templates.Login(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Ruta de login (POST)
func LoginPost(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Error al procesar el formulario", http.StatusBadRequest)
			return
		}

		username := r.FormValue("username")
		password := r.FormValue("password")

		// Verificar credenciales
		if username == config.Username && password == config.Password {
			// Crear cookie de sesión
			sessionCookie := createSessionCookie(username, config)
			http.SetCookie(w, sessionCookie)

			// Redirigir a la página de subida
			http.Redirect(w, r, "/upload", http.StatusSeeOther)
			return
		}

		// Credenciales incorrectas
		data := templates.LoginData{
			Title:        config.Title,
			Username:     username,
			ErrorMessage: "Usuario o contraseña incorrectos",
		}

		layoutData := templates.LayoutData{
			Title:      config.Title + " - Iniciar sesión",
			IsLoggedIn: false,
			Username:   "",
		}

		// Renderizar la plantilla con el layout
		component := templates.Login(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Ruta de logout
func Logout(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Borrar la cookie de sesión
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})

		// Redirigir a la página principal
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// Ruta para mostrar el formulario de subida de archivos (GET) - protegida
func Upload(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Obtener el nombre de usuario
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
			Title:      config.Title + " - Subir archivos",
			IsLoggedIn: true,
			Username:   username,
		}

		// Renderizar la plantilla con el layout
		component := templates.Upload(data)
		ctx := r.Context()
		handler := templates.LayoutWithData(layoutData)

		templ.Handler(handler).ServeHTTP(w, r.WithContext(templ.WithChildren(ctx, component)))
	}
}

// Ruta para procesar la subida de archivos (POST) - protegida
func UploadPost(config *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limitar el tamaño máximo del formulario a 32MB
		r.ParseMultipartForm(32 << 20)

		// Obtener los archivos subidos
		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "No se han seleccionado archivos",
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		// Validación: asegurarse de que el directorio de destino exista y tenga permisos
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "Error de configuración: " + err.Error(),
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		// Verificar permisos de escritura
		if _, err := os.Stat(absRoot); err != nil {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "Error al acceder al directorio de destino: " + err.Error(),
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		uploadedFiles := []string{}
		var errorMessage string

		// Procesar cada archivo
		for _, fileHeader := range files {
			// Obtener el archivo
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Error al abrir archivo: %v", err)
				continue
			}
			defer file.Close()

			// Crear el destino
			dst, err := os.Create(filepath.Join(absRoot, fileHeader.Filename))
			if err != nil {
				errorMessage = "Error al crear archivo de destino: " + err.Error()
				log.Printf("%s", errorMessage)
				continue
			}
			defer dst.Close()

			// Copiar contenido
			if _, err = io.Copy(dst, file); err != nil {
				errorMessage = "Error al guardar archivo: " + err.Error()
				log.Printf("%s", errorMessage)
				continue
			}

			uploadedFiles = append(uploadedFiles, fileHeader.Filename)
		}

		// Preparar respuesta
		data := templates.UploadData{
			Title:     config.Title,
			Directory: config.RootDir,
			Success:   len(uploadedFiles) > 0,
			Message:   "",
		}

		if len(uploadedFiles) > 0 {
			if len(uploadedFiles) == 1 {
				data.Message = "Archivo subido con éxito: " + uploadedFiles[0]
			} else {
				data.Message = fmt.Sprintf("%d archivos subidos con éxito", len(uploadedFiles))
			}
		} else if errorMessage != "" {
			data.Message = errorMessage
		} else {
			data.Message = "No se pudo procesar ningún archivo"
		}

		// Renderizar la plantilla con el resultado
		templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
	}
}
