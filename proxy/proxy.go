package proxy

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed config.yml
var configFS embed.FS

type Tunnel struct {
	ID string `json:"id"`
}

// --- Modelo para el request a la API de Cloudflare ---
type DNSRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// Config representa la configuración del túnel
type TunnelConfig struct {
	Tunnel          string `yaml:"tunnel"`
	CredentialsFile string `yaml:"credentials-file"`
	Ingress         []struct {
		Hostname string `yaml:"hostname,omitempty"`
		Service  string `yaml:"service"`
	} `yaml:"ingress"`
}

// --- Función para crear el registro DNS ---
func CreateDNSRecord(domain, tunnelURL, zoneID, apiToken string) (string, error) {
	subdomain := generateSubdomain(8)
	hostname := fmt.Sprintf("%s.%s", subdomain, domain)
	// return hostname, nil
	fmt.Println("hostname", hostname)
	record := DNSRecord{
		Type:    "CNAME",
		Name:    hostname,
		Content: tunnelURL,
		TTL:     120,
		Proxied: true,
	}

	body, _ := json.Marshal(record)
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error al crear el registro DNS:", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println("✅ Registro DNS creado con éxito")
		return hostname, nil
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error leyendo respuesta: %w", err)
	}
	return "", fmt.Errorf("falló la creación del DNS (%d): %s", resp.StatusCode, string(bodyBytes))
}

// --- Selecciona el binario correcto y lo extrae ---
func ExtractCloudflaredBinary(embeddedBinaries embed.FS) (string, error) {
	var binPath string
	switch runtime.GOOS {
	case "linux":
		binPath = "cloudflared/linux-amd64/cloudflared"
	case "darwin":
		if runtime.GOARCH == "arm64" {
			binPath = "cloudflared/darwin-arm64/cloudflared"
		} else {
			binPath = "cloudflared/darwin-amd64/cloudflared"
		}
	case "windows":
		binPath = "cloudflared/windows-amd64/cloudflared.exe"
	default:
		return "", fmt.Errorf("sistema operativo no soportado: %s", runtime.GOOS)
	}

	// Verificar si el binario existe en el FS embebido
	_, err := embeddedBinaries.ReadFile(binPath)
	if err != nil {
		return "", fmt.Errorf("binario de cloudflared no encontrado para %s/%s: %w", runtime.GOOS, runtime.GOARCH, err)
	}

	// Crear directorio temporal
	tmpDir, err := os.MkdirTemp("", "cloudflared-*")
	if err != nil {
		return "", fmt.Errorf("error creando directorio temporal: %w", err)
	}

	// Leer el binario embebido
	data, err := embeddedBinaries.ReadFile(binPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("error leyendo cloudflared embebido: %w", err)
	}

	// Construir la ruta de salida
	outputPath := filepath.Join(tmpDir, filepath.Base(binPath))

	// Escribir el binario con permisos de ejecución
	err = os.WriteFile(outputPath, data, 0755)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("error escribiendo binario: %w", err)
	}

	// Verificar que el binario es ejecutable
	if runtime.GOOS != "windows" {
		cmd := exec.Command(outputPath, "version")
		if err := cmd.Run(); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("error verificando binario: %w", err)
		}
	}

	return outputPath, nil
}

func WriteTunnelConfig(binPath, hostname string, localPort int, tunnelName string) (*os.File, error) {
	// Obtener el directorio home del usuario
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo directorio home: %w", err)
	}

	// Crear directorio .cloudflared si no existe
	cloudflaredDir := filepath.Join(homeDir, ".cloudflared")
	if err := os.MkdirAll(cloudflaredDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio .cloudflared: %w", err)
	}

	// Obtener el ID del túnel
	getTunnelFileContent := exec.Command(binPath, "tunnel", "list", "--name", tunnelName, "--output", "json")
	output, err := getTunnelFileContent.Output()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo ID del túnel: %w", err)
	}

	var tunnels []Tunnel
	err = json.Unmarshal(output, &tunnels)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	if len(tunnels) == 0 {
		return nil, fmt.Errorf("no se encontró el túnel con nombre %s", tunnelName)
	}

	tunnelID := tunnels[0].ID

	// Leer la configuración base embebida
	configData, err := configFS.ReadFile("config.yml")
	if err != nil {
		return nil, fmt.Errorf("error leyendo configuración base: %w", err)
	}

	// Deserializar la configuración
	var config TunnelConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("error deserializando configuración: %w", err)
	}

	// Actualizar la configuración
	config.Tunnel = tunnelName
	config.CredentialsFile = filepath.Join(cloudflaredDir, fmt.Sprintf("%s.json", tunnelID))

	// Actualizar las reglas de ingress
	config.Ingress[0].Hostname = hostname
	config.Ingress[0].Service = fmt.Sprintf("http://localhost:%d", localPort)
	config.Ingress[1].Service = "http_status:404"

	// Serializar la configuración actualizada
	updatedConfig, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("error serializando configuración: %w", err)
	}

	// Construir la ruta al archivo de credenciales del túnel
	credentialsPath := filepath.Join(cloudflaredDir, fmt.Sprintf("%s.json", tunnelID))

	// Verificar si el archivo existe
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		// Generar el archivo de credenciales
		genCreds := exec.Command(binPath, "tunnel", "token", tunnelID)
		token, err := genCreds.Output()
		if err != nil {
			return nil, fmt.Errorf("error generando token del túnel: %w", err)
		}

		// Crear un JSON válido con el token
		credentials := map[string]string{
			"AccountTag":   "122a0fa673f6a5705fd1fa948e061322",
			"TunnelSecret": strings.TrimSpace(string(token)),
			"TunnelID":     tunnelID,
			"TunnelName":   tunnelName,
		}

		// Convertir el mapa a JSON
		jsonData, err := json.MarshalIndent(credentials, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("error creando JSON: %w", err)
		}

		// Crear el archivo de credenciales
		if err := os.WriteFile(credentialsPath, jsonData, 0600); err != nil {
			return nil, fmt.Errorf("error creando archivo de credenciales: %w", err)
		}
	}

	// Verificar que el archivo de credenciales es válido
	credsData, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("error leyendo archivo de credenciales: %w", err)
	}

	var creds map[string]interface{}
	if err := json.Unmarshal(credsData, &creds); err != nil {
		return nil, fmt.Errorf("error validando JSON de credenciales: %w", err)
	}

	// Crear un archivo temporal en memoria
	tmpFile, err := os.CreateTemp("", "cloudflared-config-*.yml")
	if err != nil {
		return nil, fmt.Errorf("error creando archivo temporal: %w", err)
	}

	// Escribir la configuración en el archivo temporal
	if _, err := tmpFile.Write(updatedConfig); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("error escribiendo configuración temporal: %w", err)
	}

	// Cerrar el archivo para que cloudflared pueda leerlo
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("error cerrando archivo temporal: %w", err)
	}

	return tmpFile, nil
}

// --- Ejecuta cloudflared con los argumentos adecuados ---
func RunCloudflared(binPath, hostname string, localPort int, tunnelName string) (string, error) {
	// Validar que el puerto sea válido
	if localPort <= 0 || localPort > 65535 {
		return "", fmt.Errorf("puerto inválido: %d", localPort)
	}

	tmpFile, err := WriteTunnelConfig(binPath, hostname, localPort, tunnelName)
	if err != nil {
		return "", fmt.Errorf("error escribiendo configuración temporal: %w", err)
	}

	return tmpFile.Name(), nil
}

// --- Generador simple de subdominios ---
func generateSubdomain(n int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = charset[r.Intn(len(charset))]
	}
	return string(b)
}
