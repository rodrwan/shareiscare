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

// Version is the program version, injected during compilation
var Version = "dev"

// PrintHelp displays program help
func PrintHelp() {
	fmt.Printf("ShareIsCare v%s - Simple file server\n", Version)
	fmt.Println("\nUsage:")
	fmt.Println("  shareiscare                       Start the server")
	fmt.Println("  shareiscare init [path]           Generate base configuration file")
	fmt.Println("  shareiscare help                  Show this help")
	fmt.Println("  shareiscare version               Show program version")
	fmt.Println("\nExamples:")
	fmt.Println("  shareiscare                       Start server with config.yaml")
	fmt.Println("  shareiscare init                  Generate config.yaml in current directory")
	fmt.Println("  shareiscare init my-config.yaml   Generate configuration in my-config.yaml")
}

// RunServer starts the HTTP server
func RunServer(config *config.Config) {
	// Main handler route (file listing)
	http.HandleFunc("GET /", handlers.Index(config))
	// Route for browsing directories
	http.HandleFunc("GET /browse/", handlers.Browse(config))
	// Route for downloading files
	http.HandleFunc("GET /download", handlers.Download(config))
	// Route for previewing files
	http.HandleFunc("GET /preview", handlers.Preview(config))
	// Login route (GET)
	http.HandleFunc("GET /login", handlers.Login(config))
	// Login route (POST)
	http.HandleFunc("POST /login", handlers.LoginPost(config))
	// Logout route
	http.HandleFunc("GET /logout", handlers.Logout(config))
	// Route to display the file upload form (GET) - protected
	http.HandleFunc("GET /upload", handlers.RequireAuth(handlers.Upload(config), config))
	// Route to process file uploads (POST) - protected
	http.HandleFunc("POST /upload", handlers.RequireAuth(handlers.UploadPost(config), config))
	// Route to delete files (POST) - protected and admin only
	http.HandleFunc("POST /delete", handlers.RequireAuth(handlers.RequireAdmin(handlers.Delete(config), config), config))

	// Start the server
	addr := fmt.Sprintf(":%d", config.Port)
	log.Printf("ShareIsCare v%s started at http://localhost%s", Version, addr)
	log.Printf("Default user: %s / Password: %s", config.Username, config.Password)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func main() {
	// Process command line arguments
	if len(os.Args) > 1 {
		cmd := strings.ToLower(os.Args[1])

		switch cmd {
		case "init":
			// Command to generate base configuration
			configFile := "config.yaml"
			if len(os.Args) > 2 {
				configFile = os.Args[2]
			}

			// Check if the file already exists
			if _, err := os.Stat(configFile); err == nil {
				fmt.Printf("The file %s already exists. Do you want to overwrite it? (y/n): ", configFile)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("Operation cancelled.")
					return
				}
			}

			// Generate default configuration
			cfg := config.DefaultConfig()
			if err := config.SaveConfig(cfg, configFile); err != nil {
				log.Fatalf("Error generating configuration: %v", err)
			}

			fmt.Printf("Configuration file generated: %s\n", configFile)
			fmt.Printf("Default user: %s / Password: %s\n", cfg.Username, cfg.Password)
			fmt.Println("IMPORTANT: It is recommended to change the default credentials.")
			return

		case "help", "-h", "--help":
			// Help command
			PrintHelp()
			return

		case "version", "-v", "--version":
			// Command to display program version
			fmt.Printf("ShareIsCare v%s\n", Version)
			return

		default:
			fmt.Printf("Unknown command: %s\n\n", cmd)
			PrintHelp()
			return
		}
	}

	// Normal mode: start server
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// If the configuration file doesn't exist, create it
	if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
		fmt.Println("Configuration file not found. Creating config.yaml with default values...")
		if err := config.SaveConfig(cfg, "config.yaml"); err != nil {
			log.Printf("Warning: Could not save configuration file: %v", err)
		} else {
			fmt.Println("Configuration file generated: config.yaml")
			fmt.Printf("Default user: %s / Password: %s\n", cfg.Username, cfg.Password)
			fmt.Println("IMPORTANT: It is recommended to change the default credentials.")
		}
	}

	// Start the server
	RunServer(cfg)
}
