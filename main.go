package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/a-h/templ"
	"github.com/rodrwan/shareiscare/templates"
	"gopkg.in/yaml.v3"
)

// Version es la versión del programa, que se inyecta durante la compilación
var Version = "dev"

// Config estructura para la configuración del servidor
type Config struct {
	Port    int    `yaml:"port"`
	RootDir string `yaml:"root_dir"`
	Title   string `yaml:"title"`
}

// DefaultConfig retorna una configuración por defecto
func DefaultConfig() *Config {
	return &Config{
		Port:    8080,          // Puerto por defecto
		RootDir: ".",           // Directorio actual por defecto
		Title:   "ShareIsCare", // Título por defecto
	}
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
	yamlContent += string(configData)

	if err := os.WriteFile(filename, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("error al guardar archivo de configuración: %v", err)
	}

	return nil
}

// PrintHelp muestra la ayuda del programa
func PrintHelp() {
	fmt.Printf("ShareIsCare v%s - Servidor de archivos simple\n", Version)
	fmt.Println("\nUso:")
	fmt.Println("  shareiscare                       Inicia el servidor")
	fmt.Println("  shareiscare init [ruta]           Genera archivo de configuración base")
	fmt.Println("  shareiscare help                  Muestra esta ayuda")
	fmt.Println("  shareiscare version               Muestra la versión del programa")
	fmt.Println("\nEjemplos:")
	fmt.Println("  shareiscare                       Inicia el servidor con config.yaml")
	fmt.Println("  shareiscare init                  Genera config.yaml en el directorio actual")
	fmt.Println("  shareiscare init mi-config.yaml   Genera configuración en mi-config.yaml")
}

// RunServer inicia el servidor HTTP
func RunServer(config *Config) {
	// Ruta para el handler principal (listado de archivos)
	http.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		files, err := os.ReadDir(config.RootDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Lista de archivos a excluir
		excludeFiles := map[string]bool{
			"config.yaml":     true,
			"shareiscare":     true,
			"shareiscare.exe": true,
		}

		var fileInfos []templates.FileInfo
		for _, file := range files {
			// Filtrar archivos de sistema de ShareIsCare
			if excludeFiles[file.Name()] {
				continue
			}

			filePath := filepath.Join(config.RootDir, file.Name())

			// Obtener información del archivo
			info, err := os.Stat(filePath)
			if err != nil {
				continue
			}

			// Formatear tamaño
			size := ""
			if !info.IsDir() {
				bytes := info.Size()
				if bytes < 1024 {
					size = fmt.Sprintf("%d B", bytes)
				} else if bytes < 1024*1024 {
					size = fmt.Sprintf("%.1f KB", float64(bytes)/1024)
				} else {
					size = fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
				}
			} else {
				size = "directorio"
			}

			fileInfos = append(fileInfos, templates.FileInfo{
				Name:  file.Name(),
				Path:  file.Name(),
				Size:  size,
				IsDir: info.IsDir(),
			})
		}

		data := templates.IndexData{
			Title:     config.Title,
			Directory: config.RootDir,
			Files:     fileInfos,
		}

		// Renderizar la plantilla
		templ.Handler(templates.Index(data)).ServeHTTP(w, r)
	})

	// Ruta para descargar archivos
	http.HandleFunc("GET /download", func(w http.ResponseWriter, r *http.Request) {
		filename := r.URL.Query().Get("filename")
		if filename == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		// Validar que el archivo esté dentro del directorio configurado
		fullPath := filepath.Join(config.RootDir, filename)
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			http.Error(w, "Error de configuración", http.StatusInternalServerError)
			return
		}
		absPath, err := filepath.Abs(fullPath)
		if err != nil {
			http.Error(w, "Ruta inválida", http.StatusBadRequest)
			return
		}

		rel, err := filepath.Rel(absRoot, absPath)
		if err != nil || strings.HasPrefix(rel, "..") || strings.Contains(rel, "/../") {
			http.Error(w, "Acceso denegado", http.StatusForbidden)
			return
		}

		// Verificar si el archivo existe y no es un directorio
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

		// Configurar las cabeceras para forzar la descarga
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filename)))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// Enviar el archivo
		_, err = io.Copy(w, file)
		if err != nil {
			log.Printf("Error al enviar archivo: %v", err)
		}
	})

	// Ruta para mostrar el formulario de subida de archivos (GET)
	http.HandleFunc("GET /upload", func(w http.ResponseWriter, r *http.Request) {
		data := templates.UploadData{
			Title:     config.Title,
			Directory: config.RootDir,
			Success:   false,
			Message:   "",
		}

		// Renderizar la plantilla
		templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
	})

	// Ruta para procesar la subida de archivos (POST)
	http.HandleFunc("POST /upload", func(w http.ResponseWriter, r *http.Request) {
		// Limitar el tamaño máximo del formulario a 32MB
		r.ParseMultipartForm(32 << 20)

		// Obtener los archivos subidos
		files := r.MultipartForm.File["files"]
		if len(files) == 0 {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "No se han seleccionado archivos",
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		// Validación: asegurarse de que el directorio de destino exista y tenga permisos
		absRoot, err := filepath.Abs(config.RootDir)
		if err != nil {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "Error de configuración: " + err.Error(),
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		// Verificar permisos de escritura
		if _, err := os.Stat(absRoot); err != nil {
			data := templates.UploadData{
				Title:     config.Title,
				Directory: config.RootDir,
				Success:   false,
				Message:   "Error al acceder al directorio de destino: " + err.Error(),
			}
			templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
			return
		}

		uploadedFiles := []string{}
		var errorMessage string

		// Procesar cada archivo
		for _, fileHeader := range files {
			// Obtener el archivo
			file, err := fileHeader.Open()
			if err != nil {
				log.Printf("Error al abrir archivo: %v", err)
				continue
			}
			defer file.Close()

			// Crear el destino
			dst, err := os.Create(filepath.Join(absRoot, fileHeader.Filename))
			if err != nil {
				errorMessage = "Error al crear archivo de destino: " + err.Error()
				log.Printf("%s", errorMessage)
				continue
			}
			defer dst.Close()

			// Copiar contenido
			if _, err = io.Copy(dst, file); err != nil {
				errorMessage = "Error al guardar archivo: " + err.Error()
				log.Printf("%s", errorMessage)
				continue
			}

			uploadedFiles = append(uploadedFiles, fileHeader.Filename)
		}

		// Preparar respuesta
		data := templates.UploadData{
			Title:     config.Title,
			Directory: config.RootDir,
			Success:   len(uploadedFiles) > 0,
			Message:   "",
		}

		if len(uploadedFiles) > 0 {
			if len(uploadedFiles) == 1 {
				data.Message = "Archivo subido con éxito: " + uploadedFiles[0]
			} else {
				data.Message = fmt.Sprintf("%d archivos subidos con éxito", len(uploadedFiles))
			}
		} else if errorMessage != "" {
			data.Message = errorMessage
		} else {
			data.Message = "No se pudo procesar ningún archivo"
		}

		// Renderizar la plantilla con el resultado
		templ.Handler(templates.Upload(data)).ServeHTTP(w, r)
	})

	// Iniciar el servidor
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("ShareIsCare v%s iniciado en http://localhost%s", Version, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func main() {
	// Procesar argumentos de línea de comandos
	if len(os.Args) > 1 {
		cmd := strings.ToLower(os.Args[1])

		switch cmd {
		case "init":
			// Comando para generar configuración base
			configFile := "config.yaml"
			if len(os.Args) > 2 {
				configFile = os.Args[2]
			}

			// Verificar si el archivo ya existe
			if _, err := os.Stat(configFile); err == nil {
				fmt.Printf("El archivo %s ya existe. ¿Deseas sobrescribirlo? (s/n): ", configFile)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "s" {
					fmt.Println("Operación cancelada.")
					return
				}
			}

			// Generar configuración por defecto
			config := DefaultConfig()
			if err := SaveConfig(config, configFile); err != nil {
				log.Fatalf("Error al generar la configuración: %v", err)
			}

			fmt.Printf("Archivo de configuración generado: %s\n", configFile)
			return

		case "help", "-h", "--help":
			// Comando de ayuda
			PrintHelp()
			return

		case "version", "-v", "--version":
			// Comando para mostrar la versión del programa
			fmt.Printf("ShareIsCare v%s\n", Version)
			return

		default:
			fmt.Printf("Comando desconocido: %s\n\n", cmd)
			PrintHelp()
			return
		}
	}

	// Modo normal: iniciar servidor
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Error al cargar configuración: %v", err)
	}

	// Si no existe el archivo de configuración, crearlo
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		fmt.Println("No se encontró archivo de configuración. Creando config.yaml con valores por defecto...")
		if err := SaveConfig(config, "config.yaml"); err != nil {
			log.Printf("Advertencia: No se pudo guardar el archivo de configuración: %v", err)
		} else {
			fmt.Println("Archivo de configuración generado: config.yaml")
		}
	}

	// Iniciar el servidor
	RunServer(config)
}
