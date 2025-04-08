package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

func main() {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	fmt.Printf("🔍 Detectado: %s-%s\n", goos, goarch)

	switch goos {
	case "linux":
		// If the cloudflared binary is already in the cloudflared directory, skip the download
		if _, err := os.Stat("cloudflared/linux-amd64/cloudflared"); err == nil {
			fmt.Println("🔍 cloudflared ya está en el directorio cloudflared")
			return
		}
	case "darwin":
		if goarch == "arm64" {
			// If the cloudflared binary is already in the cloudflared directory, skip the download
			if _, err := os.Stat("cloudflared/darwin-arm64/cloudflared"); err == nil {
				fmt.Println("🔍 cloudflared ya está en el directorio cloudflared")
				return
			}
		} else {
			// If the cloudflared binary is already in the cloudflared directory, skip the download
			if _, err := os.Stat("cloudflared/darwin-amd64/cloudflared"); err == nil {
				fmt.Println("🔍 cloudflared ya está en el directorio cloudflared")
				return
			}
		}
	case "windows":
		// If the cloudflared binary is already in the cloudflared directory, skip the download
		if _, err := os.Stat("cloudflared/windows-amd64/cloudflared.exe"); err == nil {
			fmt.Println("🔍 cloudflared ya está en el directorio cloudflared")
			return
		}
	default:
		panic("SO no soportado")
	}

	var url, outPath string
	switch goos {
	case "linux":
		url = "https://github.com/cloudflare/cloudflared/releases/download/2025.4.0/cloudflared-linux-amd64"
		outPath = "cloudflared/linux-amd64/cloudflared"
	case "darwin":
		if goarch == "arm64" {
			url = "https://github.com/cloudflare/cloudflared/releases/download/2025.4.0/cloudflared-darwin-arm64.tgz"
			outPath = "cloudflared/darwin-arm64/cloudflared.tgz"
		} else {
			url = "https://github.com/cloudflare/cloudflared/releases/download/2025.4.0/cloudflared-darwin-amd64.tgz"
			outPath = "cloudflared/darwin-amd64/cloudflared.tgz"
		}
	case "windows":
		url = "https://github.com/cloudflare/cloudflared/releases/download/2025.4.0/cloudflared-windows-amd64.exe"
		outPath = "cloudflared/windows-amd64/cloudflared.exe"
	default:
		panic("SO no soportado")
	}

	fmt.Println("⬇️  Descargando cloudflared desde:", url)

	// Crear carpetas necesarias
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		panic(err)
	}

	// Descargar el binario
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	outFile, err := os.Create(outPath)
	if err != nil {
		panic(err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println("🔍 Descarga completa:", outPath)

	// Descomprimir el binario si es necesario
	if goos == "darwin" {
		fmt.Println("🔍 Descomprimiendo cloudflared...")
		// Verificar si el archivo existe y tiene tamaño razonable
		fileInfo, err := os.Stat(outPath)
		if err != nil {
			panic(fmt.Errorf("error al verificar archivo .tgz: %w", err))
		}

		if fileInfo.Size() < 1000 {
			panic(fmt.Errorf("archivo .tgz demasiado pequeño, posiblemente corrupto: %d bytes", fileInfo.Size()))
		}

		if err := extractTarGz(outPath); err != nil {
			fmt.Printf("❌ Error al descomprimir: %v\n", err)
			fmt.Println("⚠️ Intentando descarga directa del binario...")

			// Intentar descargar directamente el binario como alternativa
			directURL := "https://github.com/cloudflare/cloudflared/releases/download/2025.4.0/cloudflared-darwin-arm64"
			directOutPath := filepath.Dir(outPath) + "/cloudflared"

			directResp, err := http.Get(directURL)
			if err != nil {
				panic(fmt.Errorf("error en descarga directa: %w", err))
			}
			defer directResp.Body.Close()

			directOutFile, err := os.Create(directOutPath)
			if err != nil {
				panic(fmt.Errorf("error al crear archivo de salida: %w", err))
			}
			defer directOutFile.Close()

			_, err = io.Copy(directOutFile, directResp.Body)
			if err != nil {
				panic(fmt.Errorf("error al copiar archivo: %w", err))
			}

			// Actualizar outPath al nuevo archivo
			outPath = directOutPath
		} else {
			// Eliminar el archivo .tgz si la extracción fue exitosa
			fmt.Println("🔍 Eliminando archivo .tgz...")
			// Guardar la referencia al archivo tgz antes de modificar outPath
			tgzPath := outPath
			// Asegurarse de que outPath apunte al nuevo archivo extraído
			outPath = filepath.Dir(outPath) + "/cloudflared"
			os.Remove(tgzPath)
		}
	}

	if goos != "windows" {
		fmt.Println("🔍 Estableciendo permisos de ejecución para:", outPath)
		if err := os.Chmod(outPath, 0755); err != nil {
			panic(fmt.Errorf("error al establecer permisos: %w", err))
		}
	}

	fmt.Println("✅ Preparación completa:", outPath)
}

// extractTarGz descomprime un archivo .tgz
func extractTarGz(tarGzPath string) error {
	// Crear una carpeta temporal para extraer
	tempDir := filepath.Dir(tarGzPath) + "/temp"
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	// Abrir el archivo .tgz
	file, err := os.Open(tarGzPath)
	if err != nil {
		return fmt.Errorf("error al abrir archivo .tgz: %w", err)
	}
	defer file.Close()

	// Descomprimir gzip
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("error en gzip.NewReader: %w", err)
	}
	defer gzr.Close()

	// Extraer tar
	tr := tar.NewReader(gzr)
	found := false

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error al leer entrada tar: %w", err)
		}

		// Verificar si es el binario que buscamos
		if filepath.Base(header.Name) == "cloudflared" {
			outPath := filepath.Dir(tarGzPath) + "/cloudflared"
			outFile, err := os.Create(outPath)
			if err != nil {
				return fmt.Errorf("error al crear archivo de salida: %w", err)
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, tr); err != nil {
				return fmt.Errorf("error al copiar archivo: %w", err)
			}

			// Establecer permisos de ejecución inmediatamente
			if err := os.Chmod(outPath, 0755); err != nil {
				return fmt.Errorf("error al establecer permisos: %w", err)
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("no se encontró el binario cloudflared en el archivo .tgz")
	}

	return nil
}
