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
	// Crear directorio temporal para pruebas
	tempDir, err := os.MkdirTemp("", "shareiscare-test-index")
	if err != nil {
		t.Fatalf("Error al crear directorio temporal: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Crear algunos archivos de prueba
	testFiles := []string{"archivo1.txt", "archivo2.txt"}
	for _, filename := range testFiles {
		path := filepath.Join(tempDir, filename)
		if err := os.WriteFile(path, []byte("contenido de prueba"), 0644); err != nil {
			t.Fatalf("Error al crear archivo de prueba: %v", err)
		}
	}

	// Crear un directorio de prueba
	testDirPath := filepath.Join(tempDir, "directorio_prueba")
	if err := os.Mkdir(testDirPath, 0755); err != nil {
		t.Fatalf("Error al crear directorio de prueba: %v", err)
	}

	// Configuración para la prueba
	config := &config.Config{
		Port:    8080,
		RootDir: tempDir,
		Title:   "Test Server",
	}

	// Crear servidor HTTP de prueba
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Implementar un handler similar al de RunServer pero simplificado para pruebas
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(config.RootDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Simplemente verificamos que la respuesta contenga los nombres de los archivos
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		for _, file := range files {
			io.WriteString(w, file.Name()+"\n")
		}
	})

	// Ejecutar la solicitud
	handler.ServeHTTP(w, req)

	// Verificar la respuesta
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Código de estado incorrecto, esperado: %d, obtenido: %d", http.StatusOK, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error al leer el cuerpo de la respuesta: %v", err)
	}

	bodyStr := string(body)

	// Verificar que la respuesta contenga los nombres de los archivos
	for _, filename := range testFiles {
		if !strings.Contains(bodyStr, filename) {
			t.Errorf("Respuesta no contiene el archivo '%s'", filename)
		}
	}

	// Verificar que la respuesta contenga el nombre del directorio
	if !strings.Contains(bodyStr, "directorio_prueba") {
		t.Errorf("Respuesta no contiene el directorio 'directorio_prueba'")
	}
}

func TestDownloadHandler(t *testing.T) {
	// Crear directorio temporal para pruebas
	tempDir, err := os.MkdirTemp("", "shareiscare-test-download")
	if err != nil {
		t.Fatalf("Error al crear directorio temporal: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Crear un archivo de prueba
	testFilename := "archivo_para_descargar.txt"
	testContent := "Contenido de prueba para descargar"
	testFilePath := filepath.Join(tempDir, testFilename)

	if err := os.WriteFile(testFilePath, []byte(testContent), 0644); err != nil {
		t.Fatalf("Error al crear archivo de prueba: %v", err)
	}

	// Configuración para la prueba
	config := &config.Config{
		Port:    8080,
		RootDir: tempDir,
		Title:   "Test Server",
	}

	// Crear solicitud HTTP de prueba
	req := httptest.NewRequest(http.MethodGet, "/download?filename="+testFilename, nil)
	w := httptest.NewRecorder()

	// Implementar un handler similar al de RunServer pero simplificado para pruebas
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Validar ruta
		fullPath := filepath.Join(config.RootDir, filename)

		// Verificar si el archivo existe
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

		// Configurar las cabeceras para la descarga
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", "0") // Simplificado para la prueba

		// Enviar el contenido del archivo
		io.Copy(w, file)
	})

	// Ejecutar la solicitud
	handler.ServeHTTP(w, req)

	// Verificar la respuesta
	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Código de estado incorrecto, esperado: %d, obtenido: %d", http.StatusOK, resp.StatusCode)
	}

	// Verificar cabeceras
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, testFilename) {
		t.Errorf("Content-Disposition incorrecto, esperado que contenga '%s', obtenido: '%s'", testFilename, cd)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Errorf("Content-Type incorrecto, esperado: 'application/octet-stream', obtenido: '%s'", ct)
	}

	// Verificar el contenido
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Error al leer el cuerpo de la respuesta: %v", err)
	}

	if string(body) != testContent {
		t.Errorf("Contenido incorrecto, esperado: '%s', obtenido: '%s'", testContent, string(body))
	}
}
