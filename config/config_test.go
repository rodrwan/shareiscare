package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != 8080 {
		t.Errorf("Puerto por defecto erróneo, esperado: 8080, obtenido: %d", cfg.Port)
	}

	if cfg.RootDir != "." {
		t.Errorf("Directorio raíz por defecto erróneo, esperado: '.', obtenido: %s", cfg.RootDir)
	}

	if cfg.Title != "ShareIsCare" {
		t.Errorf("Título por defecto erróneo, esperado: 'ShareIsCare', obtenido: %s", cfg.Title)
	}

	if cfg.Username != "admin" {
		t.Errorf("Nombre de usuario por defecto erróneo, esperado: 'admin', obtenido: %s", cfg.Username)
	}

	if cfg.Password != "shareiscare" {
		t.Errorf("Contraseña por defecto errónea, esperada: 'shareiscare', obtenida: %s", cfg.Password)
	}

	if cfg.SecretKey == "" {
		t.Error("SecretKey por defecto no debería estar vacía")
	}
}

func TestGenerateRandomKey(t *testing.T) {
	// Prueba indirecta de generateRandomKey a través de DefaultConfig
	cfg1 := DefaultConfig()

	// Esperamos un poco para asegurarnos de que el timestamp cambie
	time.Sleep(2 * time.Millisecond)

	cfg2 := DefaultConfig()

	if cfg1.SecretKey == cfg2.SecretKey {
		t.Error("Las claves secretas deberían ser diferentes para diferentes instancias")
	}
}

func TestLoadConfig(t *testing.T) {
	// Caso 1: Archivo no existe, debe devolver la configuración por defecto
	os.Remove("config.yaml") // Asegurarse de que no existe

	cfg, err := LoadConfig()
	if err != nil {
		t.Errorf("No se esperaba error cuando el archivo no existe: %v", err)
	}

	if cfg.Port != 8080 || cfg.Title != "ShareIsCare" {
		t.Error("Debería devolver valores por defecto cuando no existe el archivo")
	}

	// Caso 2: Crear un archivo de configuración para testear
	testConfig := &Config{
		Port:      9090,
		RootDir:   "/tmp",
		Title:     "TestConfig",
		Username:  "testuser",
		Password:  "testpass",
		SecretKey: "test-key",
	}

	err = SaveConfig(testConfig, "config.yaml")
	if err != nil {
		t.Fatalf("Error al guardar la configuración de prueba: %v", err)
	}
	defer os.Remove("config.yaml")

	// Cargar la configuración y verificar
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Errorf("Error al cargar la configuración: %v", err)
	}

	if loadedCfg.Port != 9090 || loadedCfg.Title != "TestConfig" {
		t.Errorf("La configuración cargada no coincide con la guardada")
	}
}

func TestSaveConfig(t *testing.T) {
	// Crear configuración para guardar
	cfg := &Config{
		Port:      7070,
		RootDir:   "/var/test",
		Title:     "SaveTest",
		Username:  "saveuser",
		Password:  "savepass",
		SecretKey: "save-key",
	}

	// Guardar en un archivo temporal
	tempFile := "test_config.yaml"
	err := SaveConfig(cfg, tempFile)
	if err != nil {
		t.Fatalf("Error al guardar la configuración: %v", err)
	}
	defer os.Remove(tempFile)

	// Leer el archivo y verificar que contiene los datos esperados
	data, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Error al leer el archivo guardado: %v", err)
	}

	content := string(data)

	// Verificar que contiene los comentarios
	if !strings.Contains(content, "# ShareIsCare Configuration") {
		t.Error("El archivo no contiene el comentario de encabezado esperado")
	}

	// Verificar que contiene los valores de configuración
	if !strings.Contains(content, "port: 7070") {
		t.Error("El archivo no contiene el valor de puerto esperado")
	}

	if !strings.Contains(content, "title: SaveTest") {
		t.Error("El archivo no contiene el valor de título esperado")
	}
}
