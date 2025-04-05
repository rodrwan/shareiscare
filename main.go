package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/rodrwan/shareiscare/config"
	"github.com/rodrwan/shareiscare/handlers"
)

// Version es la versión del programa, que se inyecta durante la compilación
var Version = "dev"

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
func RunServer(config *config.Config) {
	// Ruta para el handler principal (listado de archivos)
	http.HandleFunc("GET /", handlers.Index(config))
	// Ruta para descargar archivos
	http.HandleFunc("GET /download", handlers.Download(config))
	// Ruta de login (GET)
	http.HandleFunc("GET /login", handlers.Login(config))
	// Ruta de login (POST)
	http.HandleFunc("POST /login", handlers.LoginPost(config))
	// Ruta de logout
	http.HandleFunc("GET /logout", handlers.Logout(config))
	// Ruta para mostrar el formulario de subida de archivos (GET) - protegida
	http.HandleFunc("GET /upload", handlers.RequireAuth(handlers.Upload(config), config))
	// Ruta para procesar la subida de archivos (POST) - protegida
	http.HandleFunc("POST /upload", handlers.RequireAuth(handlers.UploadPost(config), config))

	// Iniciar el servidor
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("ShareIsCare v%s iniciado en http://localhost%s", Version, addr)
	log.Printf("Usuario por defecto: %s / Contraseña: %s", config.Username, config.Password)
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
			cfg := config.DefaultConfig()
			if err := config.SaveConfig(cfg, configFile); err != nil {
				log.Fatalf("Error al generar la configuración: %v", err)
			}

			fmt.Printf("Archivo de configuración generado: %s\n", configFile)
			fmt.Printf("Usuario por defecto: %s / Contraseña: %s\n", cfg.Username, cfg.Password)
			fmt.Println("IMPORTANTE: Se recomienda cambiar las credenciales por defecto.")
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
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error al cargar configuración: %v", err)
	}

	// Si no existe el archivo de configuración, crearlo
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		fmt.Println("No se encontró archivo de configuración. Creando config.yaml con valores por defecto...")
		if err := config.SaveConfig(cfg, "config.yaml"); err != nil {
			log.Printf("Advertencia: No se pudo guardar el archivo de configuración: %v", err)
		} else {
			fmt.Println("Archivo de configuración generado: config.yaml")
			fmt.Printf("Usuario por defecto: %s / Contraseña: %s\n", cfg.Username, cfg.Password)
			fmt.Println("IMPORTANTE: Se recomienda cambiar las credenciales por defecto.")
		}
	}

	// Iniciar el servidor
	RunServer(cfg)
}
