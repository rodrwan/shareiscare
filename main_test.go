package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rodrwan/shareiscare/config"
)

func TestDefaultConfig(t *testing.T) {
	config := config.DefaultConfig()

	if config.Port != 8080 {
		t.Errorf("Puerto incorrecto, esperado: 8080, obtenido: %d", config.Port)
	}

	if config.RootDir != "." {
		t.Errorf("Directorio raíz incorrecto, esperado: '.', obtenido: %s", config.RootDir)
	}

	if config.Title != "ShareIsCare" {
		t.Errorf("Título incorrecto, esperado: 'ShareIsCare', obtenido: %s", config.Title)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Crear directorio temporal para pruebas
	tempDir, err := os.MkdirTemp("", "shareiscare-test")
	if err != nil {
		t.Fatalf("Error al crear directorio temporal: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testConfigPath := filepath.Join(tempDir, "test-config.yaml")

	// Crear configuración personalizada para guardar
	testConfig := &config.Config{
		Port:    9090,
		RootDir: "/test/dir",
		Title:   "Test Title",
	}

	// Probar SaveConfig
	err = config.SaveConfig(testConfig, testConfigPath)
	if err != nil {
		t.Fatalf("Error al guardar configuración: %v", err)
	}

	// Verificar que el archivo existe
	if _, err := os.Stat(testConfigPath); os.IsNotExist(err) {
		t.Fatalf("El archivo de configuración no fue creado")
	}

	// Probar LoadConfig
	// Como LoadConfig solo lee de "config.yaml", creamos un respaldo temporal
	originalConfigPath := "config.yaml"
	backupPath := ""

	// Si existe un config.yaml, hacer backup
	if _, err := os.Stat(originalConfigPath); err == nil {
		backupPath = originalConfigPath + ".bak"
		if err := os.Rename(originalConfigPath, backupPath); err != nil {
			t.Fatalf("Error al hacer backup de config.yaml: %v", err)
		}
		defer func() {
			os.Remove(originalConfigPath)
			os.Rename(backupPath, originalConfigPath)
		}()
	}

	// Copiar nuestra configuración de prueba a config.yaml
	testConfigData, err := os.ReadFile(testConfigPath)
	if err != nil {
		t.Fatalf("Error al leer archivo de prueba: %v", err)
	}

	if err := os.WriteFile(originalConfigPath, testConfigData, 0644); err != nil {
		t.Fatalf("Error al escribir en config.yaml: %v", err)
	}

	// Ahora podemos probar LoadConfig
	loadedConfig, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Error al cargar configuración: %v", err)
	}

	// Verificar que la configuración cargada coincide con la guardada
	if loadedConfig.Port != testConfig.Port {
		t.Errorf("Puerto incorrecto, esperado: %d, obtenido: %d", testConfig.Port, loadedConfig.Port)
	}

	if loadedConfig.RootDir != testConfig.RootDir {
		t.Errorf("Directorio raíz incorrecto, esperado: %s, obtenido: %s", testConfig.RootDir, loadedConfig.RootDir)
	}

	if loadedConfig.Title != testConfig.Title {
		t.Errorf("Título incorrecto, esperado: %s, obtenido: %s", testConfig.Title, loadedConfig.Title)
	}
}
