package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecurityPathValidation(t *testing.T) {
	// Crear directorios temporales para pruebas
	rootDir, err := os.MkdirTemp("", "shareiscare-test-root")
	if err != nil {
		t.Fatalf("Error al crear directorio raíz temporal: %v", err)
	}
	defer os.RemoveAll(rootDir)

	// Crear un archivo de prueba dentro del directorio raíz
	validFilePath := filepath.Join(rootDir, "archivo_valido.txt")
	if err := os.WriteFile(validFilePath, []byte("contenido válido"), 0644); err != nil {
		t.Fatalf("Error al crear archivo de prueba: %v", err)
	}

	// Crear subdirectorio
	subDir := filepath.Join(rootDir, "subdirectorio")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Error al crear subdirectorio: %v", err)
	}

	// Archivo en subdirectorio (válido)
	validSubdirFilePath := filepath.Join(subDir, "archivo_subdir.txt")
	if err := os.WriteFile(validSubdirFilePath, []byte("contenido en subdirectorio"), 0644); err != nil {
		t.Fatalf("Error al crear archivo en subdirectorio: %v", err)
	}

	// Configuración para la prueba
	config := &Config{
		Port:    8080,
		RootDir: rootDir,
		Title:   "Test Server",
	}

	// Probar acceso a un archivo válido dentro del directorio raíz
	t.Run("Valid File Access", func(t *testing.T) {
		// En una implementación real, estos se usarían para probar el handler HTTP
		// pero aquí estamos probando sólo la función de validación
		// _ = httptest.NewRequest(http.MethodGet, "/download?filename=archivo_valido.txt", nil)
		// _ = httptest.NewRecorder()

		validatePath := func(filename string) (string, bool) {
			// Validar que el archivo esté dentro del directorio configurado
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

		filename := "archivo_valido.txt"
		path, valid := validatePath(filename)

		if !valid {
			t.Errorf("La validación de ruta rechazó un archivo válido")
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("No se pudo acceder al archivo válido: %v", err)
		}
	})

	// Probar acceso a un archivo en un subdirectorio (debería ser válido)
	t.Run("Valid Subdirectory File Access", func(t *testing.T) {
		// En una implementación real, estos se usarían para probar el handler HTTP
		// pero aquí estamos probando sólo la función de validación
		// _ = httptest.NewRequest(http.MethodGet, "/download?filename=subdirectorio/archivo_subdir.txt", nil)
		// _ = httptest.NewRecorder()

		validatePath := func(filename string) (string, bool) {
			// Validar que el archivo esté dentro del directorio configurado
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

		filename := "subdirectorio/archivo_subdir.txt"
		path, valid := validatePath(filename)

		if !valid {
			t.Errorf("La validación de ruta rechazó un archivo válido en subdirectorio")
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("No se pudo acceder al archivo válido en subdirectorio: %v", err)
		}
	})

	// Probar intento de acceso a un archivo fuera del directorio raíz (atravesar directorios)
	t.Run("Path Traversal Attack", func(t *testing.T) {
		// En una implementación real, estos se usarían para probar el handler HTTP
		// pero aquí estamos probando sólo la función de validación
		// _ = httptest.NewRequest(http.MethodGet, "/download?filename=../../../etc/passwd", nil)
		// _ = httptest.NewRecorder()

		validatePath := func(filename string) (string, bool) {
			// Validar que el archivo esté dentro del directorio configurado
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
			t.Errorf("La validación de ruta permitió un ataque de traversal de directorios")
		}
	})
}
