# Variables
BINARY_NAME=shareiscare
GO=go
TEMPL=templ
VERSION=1.0.0
BUILD_DIR=./bin
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

# Colores para la salida
YELLOW=\033[0;33m
GREEN=\033[0;32m
RED=\033[0;31m
NC=\033[0m # No Color

# Detectar sistema operativo
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
    OS = darwin
else ifeq ($(UNAME_S),Linux)
    OS = linux
else
    OS = windows
endif

.PHONY: all build clean run help install generate cross-build build-linux build-windows build-mac

all: help

# Generar código Go a partir de las plantillas templ
generate:
	@echo "$(YELLOW)Generando código Go a partir de las plantillas templ...$(NC)"
	@$(TEMPL) generate
	@echo "$(GREEN)✓ Código generado con éxito$(NC)"

# Compilar el binario principal
build: generate
	@echo "$(YELLOW)Compilando $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@$(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "$(GREEN)✓ Binario compilado en $(BUILD_DIR)/$(BINARY_NAME)$(NC)"

# Limpiar archivos generados y binarios
clean:
	@echo "$(YELLOW)Limpiando archivos generados...$(NC)"
	@rm -rf $(BUILD_DIR)
	@echo "$(GREEN)✓ Limpieza completada$(NC)"

# Ejecutar la aplicación
run: generate
	@echo "$(YELLOW)Ejecutando $(BINARY_NAME)...$(NC)"
	@$(GO) run main.go

# Actualizar dependencias
deps:
	@echo "$(YELLOW)Actualizando dependencias...$(NC)"
	@$(GO) mod tidy
	@echo "$(GREEN)✓ Dependencias actualizadas$(NC)"

# Instalar herramientas necesarias
install:
	@echo "$(YELLOW)Instalando herramientas necesarias...$(NC)"
	@$(GO) install github.com/a-h/templ/cmd/templ@latest
	@echo "$(GREEN)✓ Herramientas instaladas$(NC)"

# Compilación para diferentes plataformas
build-linux:
	@echo "$(YELLOW)Compilando para Linux...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	@echo "$(GREEN)✓ Binario compilado en $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64$(NC)"

build-windows:
	@echo "$(YELLOW)Compilando para Windows...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	@echo "$(GREEN)✓ Binario compilado en $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe$(NC)"

build-mac:
	@echo "$(YELLOW)Compilando para macOS...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	@echo "$(GREEN)✓ Binario compilado en $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64$(NC)"

# Compilar para todas las plataformas
cross-build: build-linux build-windows build-mac
	@echo "$(GREEN)✓ Compilación completada para todas las plataformas$(NC)"

# Iniciar la aplicación en modo desarrollo
dev: generate
	@echo "$(YELLOW)Iniciando en modo desarrollo...$(NC)"
	@$(GO) run main.go

# Generar archivo de configuración
init-config:
	@echo "$(YELLOW)Generando archivo de configuración...$(NC)"
	@$(GO) run main.go init
	@echo "$(GREEN)✓ Archivo de configuración generado$(NC)"

# Mostrar ayuda
help:
	@echo "$(YELLOW)ShareIsCare - Comandos disponibles:$(NC)"
	@echo ""
	@echo "  $(GREEN)make build$(NC)        - Compilar el binario"
	@echo "  $(GREEN)make run$(NC)          - Ejecutar la aplicación"
	@echo "  $(GREEN)make dev$(NC)          - Iniciar en modo desarrollo"
	@echo "  $(GREEN)make generate$(NC)     - Generar código Go a partir de las plantillas templ"
	@echo "  $(GREEN)make clean$(NC)        - Limpiar archivos generados"
	@echo "  $(GREEN)make deps$(NC)         - Actualizar dependencias"
	@echo "  $(GREEN)make install$(NC)      - Instalar herramientas necesarias"
	@echo "  $(GREEN)make init-config$(NC)  - Generar archivo de configuración por defecto"
	@echo "  $(GREEN)make cross-build$(NC)  - Compilar para Linux, Windows y macOS"
	@echo "  $(GREEN)make build-linux$(NC)  - Compilar para Linux"
	@echo "  $(GREEN)make build-windows$(NC)- Compilar para Windows"
	@echo "  $(GREEN)make build-mac$(NC)    - Compilar para macOS"
	@echo ""
	@echo "Ejecuta 'make' o 'make help' para ver esta ayuda"