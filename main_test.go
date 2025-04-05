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
		t.Errorf("Incorrect port, expected: 8080, got: %d", config.Port)
	}

	if config.RootDir != "." {
		t.Errorf("Incorrect root directory, expected: '.', got: %s", config.RootDir)
	}

	if config.Title != "ShareIsCare" {
		t.Errorf("Incorrect title, expected: 'ShareIsCare', got: %s", config.Title)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create temporary directory for tests
	tempDir, err := os.MkdirTemp("", "shareiscare-test")
	if err != nil {
		t.Fatalf("Error creating temporary directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testConfigPath := filepath.Join(tempDir, "test-config.yaml")

	// Create custom configuration to save
	testConfig := &config.Config{
		Port:    9090,
		RootDir: "/test/dir",
		Title:   "Test Title",
	}

	// Test SaveConfig
	err = config.SaveConfig(testConfig, testConfigPath)
	if err != nil {
		t.Fatalf("Error saving configuration: %v", err)
	}

	// Verify that the file exists
	if _, err := os.Stat(testConfigPath); os.IsNotExist(err) {
		t.Fatalf("Configuration file was not created")
	}

	// Test LoadConfig
	// Since LoadConfig only reads from "config.yaml", we create a temporary backup
	originalConfigPath := "config.yaml"
	backupPath := ""

	// If config.yaml exists, make a backup
	if _, err := os.Stat(originalConfigPath); err == nil {
		backupPath = originalConfigPath + ".bak"
		if err := os.Rename(originalConfigPath, backupPath); err != nil {
			t.Fatalf("Error backing up config.yaml: %v", err)
		}
		defer func() {
			os.Remove(originalConfigPath)
			os.Rename(backupPath, originalConfigPath)
		}()
	}

	// Copy our test configuration to config.yaml
	testConfigData, err := os.ReadFile(testConfigPath)
	if err != nil {
		t.Fatalf("Error reading test file: %v", err)
	}

	if err := os.WriteFile(originalConfigPath, testConfigData, 0644); err != nil {
		t.Fatalf("Error writing to config.yaml: %v", err)
	}

	// Now we can test LoadConfig
	loadedConfig, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Error loading configuration: %v", err)
	}

	// Verify that the loaded configuration matches the saved one
	if loadedConfig.Port != testConfig.Port {
		t.Errorf("Incorrect port, expected: %d, got: %d", testConfig.Port, loadedConfig.Port)
	}

	if loadedConfig.RootDir != testConfig.RootDir {
		t.Errorf("Incorrect root directory, expected: %s, got: %s", testConfig.RootDir, loadedConfig.RootDir)
	}

	if loadedConfig.Title != testConfig.Title {
		t.Errorf("Incorrect title, expected: %s, got: %s", testConfig.Title, loadedConfig.Title)
	}
}
