package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config estructura para la configuración del servidor
type Config struct {
	Port      int    `yaml:"port"`
	RootDir   string `yaml:"root_dir"`
	Title     string `yaml:"title"`
	Username  string `yaml:"username"`   // Usuario para autenticación
	Password  string `yaml:"password"`   // Contraseña para autenticación
	SecretKey string `yaml:"secret_key"` // Clave secreta para firmar sesiones
}

// DefaultConfig retorna una configuración por defecto
func DefaultConfig() *Config {
	return &Config{
		Port:      8080,                // Puerto por defecto
		RootDir:   ".",                 // Directorio actual por defecto
		Title:     "ShareIsCare",       // Título por defecto
		Username:  "admin",             // Usuario por defecto
		Password:  "shareiscare",       // Contraseña por defecto
		SecretKey: generateRandomKey(), // Clave secreta para sesiones
	}
}

// generateRandomKey genera una clave aleatoria para sesiones
func generateRandomKey() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// LoadConfig carga la configuración desde el archivo config.yaml
func LoadConfig() (*Config, error) {
	config := DefaultConfig()

	// Intenta cargar la configuración desde el archivo
	data, err := os.ReadFile("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("error al leer archivo de configuración: %v", err)
	}

	// Parsear el archivo YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("error al parsear archivo de configuración: %v", err)
	}

	return config, nil
}

// SaveConfig guarda la configuración en un archivo YAML
func SaveConfig(config *Config, filename string) error {
	configData, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("error al serializar configuración: %v", err)
	}

	// Agregamos comentarios al archivo YAML
	yamlContent := "# Configuración de ShareIsCare\n"
	yamlContent += "# Nota: Cambiar las credenciales por defecto por seguridad\n"
	yamlContent += string(configData)

	if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("error al guardar archivo de configuración: %v", err)
	}

	return nil
}
