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
	"time"
)

// --- Configuración que debés personalizar ---
const (
	apiToken   = "-RszDE1ApkMoNyHje2rwJo0zq2jT5zCk7dIGv7tG"
	zoneID     = "9a044912a1c24db5b90c24d8769f1236"
	domain     = "shareiscare.com"
	tunnelName = "shareiscare"
	tunnelURL  = "d4b6ec9f-b19e-454f-8e6b-c12ede1d6b32.cfargotunnel.com" // el que te da cloudflared
)

// --- Modelo para el request a la API de Cloudflare ---
type DNSRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// --- Función para crear el registro DNS ---
func CreateDNSRecord() (string, error) {
	subdomain := generateSubdomain(8)
	hostname := fmt.Sprintf("%s.%s", subdomain, domain)

	record := DNSRecord{
		Type:    "CNAME",
		Name:    fmt.Sprintf("%s.%s", subdomain, domain),
		Content: tunnelURL,
		TTL:     120,
		Proxied: false,
	}

	body, _ := json.Marshal(record)
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Println("✅ Registro DNS creado con éxito")
		return hostname, nil
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
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

	data, err := embeddedBinaries.ReadFile(binPath)
	if err != nil {
		return "", fmt.Errorf("error leyendo cloudflared embebido: %w", err)
	}

	tmpDir, _ := os.MkdirTemp("", "cloudflared-*")
	outputPath := filepath.Join(tmpDir, filepath.Base(binPath))
	err = os.WriteFile(outputPath, data, 0755)
	if err != nil {
		return "", fmt.Errorf("error escribiendo binario: %w", err)
	}

	return outputPath, nil
}

// --- Ejecuta cloudflared con los argumentos adecuados ---
func RunCloudflared(binPath, hostname string, localPort int) error {
	cmd := exec.Command(binPath,
		"tunnel",
		"--url", fmt.Sprintf("http://localhost:%d", localPort),
		"--hostname", hostname,
		"--name", tunnelName,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
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
