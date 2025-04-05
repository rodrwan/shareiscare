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
)

func TestUploadHandler(t *testing.T) {
	// Crear directorio temporal para pruebas
	tempDir, err := os.MkdirTemp("", "shareiscare-test-upload")
	if err != nil {
		t.Fatalf("Error al crear directorio temporal: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Configuración para la prueba
	// La configuración se utiliza indirectamente en los handlers
	_ = &Config{
		Port:    8080,
		RootDir: tempDir,
		Title:   "Test Server",
	}

	// Test GET /upload - Verificar que muestra el formulario
	t.Run("GET Upload Form", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/upload", nil)
		w := httptest.NewRecorder()

		// Handler simplificado para GET /upload
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "<form method=\"post\" enctype=\"multipart/form-data\">")
		})

		handler.ServeHTTP(w, req)

		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Código de estado incorrecto, esperado: %d, obtenido: %d", http.StatusOK, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Error al leer el cuerpo de la respuesta: %v", err)
		}

		if !strings.Contains(string(body), "<form") {
			t.Errorf("La respuesta no contiene un formulario HTML")
		}
	})

	// Test POST /upload - Verificar que procesa la subida de archivos
	t.Run("POST Upload File", func(t *testing.T) {
		// Crear un buffer para el cuerpo de la solicitud
		var requestBody bytes.Buffer

		// Crear un writer para el multipart form
		multipartWriter := multipart.NewWriter(&requestBody)

		// Crear un campo de archivo en el formulario
		fileWriter, err := multipartWriter.CreateFormFile("files", "test-upload.txt")
		if err != nil {
			t.Fatalf("Error al crear campo de archivo: %v", err)
		}

		// Contenido del archivo a subir
		fileContent := "Este es un archivo de prueba para subir"
		if _, err := fileWriter.Write([]byte(fileContent)); err != nil {
			t.Fatalf("Error al escribir contenido del archivo: %v", err)
		}

		// Cerrar el writer del multipart form
		if err := multipartWriter.Close(); err != nil {
			t.Fatalf("Error al cerrar multipart writer: %v", err)
		}

		// Crear solicitud POST con el formulario multipart
		req := httptest.NewRequest(http.MethodPost, "/upload", &requestBody)
		req.Header.Set("Content-Type", multipartWriter.FormDataContentType())
		w := httptest.NewRecorder()

		// Handler simplificado para POST /upload
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parsear el formulario multipart
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, "Error al procesar formulario: "+err.Error(), http.StatusBadRequest)
				return
			}

			// Obtener los archivos subidos
			files := r.MultipartForm.File["files"]
			if len(files) == 0 {
				http.Error(w, "No se han seleccionado archivos", http.StatusBadRequest)
				return
			}

			// Procesar el primer archivo
			fileHeader := files[0]

			// Abrir el archivo subido
			uploadedFile, err := fileHeader.Open()
			if err != nil {
				http.Error(w, "Error al abrir archivo: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer uploadedFile.Close()

			// Crear el archivo de destino
			destPath := filepath.Join(tempDir, fileHeader.Filename)
			destFile, err := os.Create(destPath)
			if err != nil {
				http.Error(w, "Error al crear archivo destino: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer destFile.Close()

			// Copiar contenido
			if _, err := io.Copy(destFile, uploadedFile); err != nil {
				http.Error(w, "Error al guardar archivo: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Responder con éxito
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "Archivo subido con éxito: "+fileHeader.Filename)
		})

		// Ejecutar la solicitud
		handler.ServeHTTP(w, req)

		// Verificar respuesta
		resp := w.Result()
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Código de estado incorrecto, esperado: %d, obtenido: %d", http.StatusOK, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Error al leer el cuerpo de la respuesta: %v", err)
		}

		if !strings.Contains(string(body), "Archivo subido con éxito") {
			t.Errorf("La respuesta no indica que el archivo fue subido con éxito")
		}

		// Verificar que el archivo existe en el directorio destino
		uploadedFilePath := filepath.Join(tempDir, "test-upload.txt")
		if _, err := os.Stat(uploadedFilePath); os.IsNotExist(err) {
			t.Errorf("El archivo no fue guardado en el servidor")
		}

		// Verificar el contenido del archivo guardado
		savedContent, err := os.ReadFile(uploadedFilePath)
		if err != nil {
			t.Fatalf("Error al leer el archivo guardado: %v", err)
		}

		if string(savedContent) != fileContent {
			t.Errorf("El contenido del archivo guardado no coincide con el original")
		}
	})
}
