package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config structure for server configuration
type Config struct {
	Port      int    `yaml:"port"`
	RootDir   string `yaml:"root_dir"`
	Title     string `yaml:"title"`
	Username  string `yaml:"username"`   // Username for authentication
	Password  string `yaml:"password"`   // Password for authentication
	SecretKey string `yaml:"secret_key"` // Secret key for signing sessions
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		Port:      8080,                // Default port
		RootDir:   ".",                 // Current directory by default
		Title:     "ShareIsCare",       // Default title
		Username:  "admin",             // Default username
		Password:  "shareiscare",       // Default password
		SecretKey: generateRandomKey(), // Secret key for sessions
	}
}

// generateRandomKey generates a random key for sessions
func generateRandomKey() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// LoadConfig loads the configuration from the config.yaml file
func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Tries to load configuration from file
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("error reading configuration file: %v", err)
	}

	// Parse YAML file
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error parsing configuration file: %v", err)
	}

	return config, nil
}

// SaveConfig saves the configuration to a YAML file
func SaveConfig(config *Config, filename string) error {
	configData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error serializing configuration: %v", err)
	}

	// Add comments to YAML file
	yamlContent := "# ShareIsCare Configuration\n"
	yamlContent += "# Note: Change default credentials for security\n"
	yamlContent += string(configData)

	if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("error saving configuration file: %v", err)
	}

	return nil
}
