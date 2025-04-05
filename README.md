# ShareIsCare

ShareIsCare es una pequeña aplicación que funciona como servidor HTTP para compartir archivos desde una carpeta específica.

## Vista previa

![Interfaz de ShareIsCare](frontend.jpeg)

## Características

- Interfaz web simple y responsive
- Configuración mediante archivo YAML
- Muestra lista de archivos con tamaños
- Visualización de contenido de archivos de texto
- Generación de configuración mediante comando
- Implementación con plantillas templ 
- Todo empaquetado en un único binario

## Instalación

```bash
# Clonar el repositorio
git clone https://github.com/rodrwan/shareiscare.git
cd shareiscare

# Método 1: Usando Makefile (recomendado)
make install       # Instalar dependencias
make build         # Compilar el proyecto
make init-config   # Generar configuración por defecto

# Método 2: Comandos manuales
go mod tidy
go install github.com/a-h/templ/cmd/templ@latest
templ generate
go build -o shareiscare main.go
```

## Compilación y Desarrollo

El proyecto incluye un Makefile con varios comandos útiles:

```bash
# Mostrar ayuda
make help

# Compilar la aplicación
make build

# Ejecutar la aplicación en modo desarrollo
make run
make dev

# Generar código de las plantillas templ
make generate

# Compilar para distintas plataformas
make build-linux
make build-windows
make build-mac
make cross-build    # Compilar para todas las plataformas

# Otras tareas útiles
make clean         # Limpiar archivos generados
make deps          # Actualizar dependencias
make init-config   # Generar archivo de configuración por defecto
```

## Configuración

La configuración se realiza a través del archivo `config.yaml`. Si no existe, se creará automáticamente con valores predeterminados al iniciar la aplicación.

### Generar configuración con comando

```bash
# Usando make
make init-config

# O directamente con el ejecutable
./shareiscare init

# Generar en una ubicación específica
./shareiscare init mi-config.yaml
```

### Estructura del archivo de configuración

```yaml
# Ejemplo de config.yaml
port: 8080        # Puerto en el que se ejecutará el servidor
root_dir: "."     # Directorio raíz para servir archivos
title: "ShareIsCare"  # Título para la interfaz web
```

## Uso

```bash
# Ver ayuda
./shareiscare help

# Iniciar el servidor
./shareiscare
```

Luego abre tu navegador en http://localhost:8080 para acceder a la interfaz web.

## Distribución

Para distribuir la aplicación, simplemente compila el binario y distribúyelo:

```bash
# Compilar para todas las plataformas
make cross-build

# Los binarios estarán disponibles en la carpeta ./build/
```

## Creación de Releases

El proyecto está configurado para generar releases automáticos en GitHub cuando se crean tags de versión. Para crear un nuevo release:

```bash
# Crear un nuevo tag y release
make release v=1.0.0  # Reemplaza 1.0.0 con el número de versión deseado
```

Este comando:
1. Crea un tag Git con el formato `v1.0.0`
2. Empuja el tag a GitHub
3. Activa el flujo de trabajo de GitHub Actions
4. Compila automáticamente los binarios para todas las plataformas
5. Crea un release en GitHub con los binarios adjuntos

Los releases quedarán disponibles en la [página de releases](https://github.com/rodrwan/shareiscare/releases) del repositorio.

## Arquitectura

El proyecto utiliza:
- Go como lenguaje base
- Templ para las plantillas HTML
- YAML para la configuración

Las plantillas se compilan a código Go, lo que permite empaquetar todo en un único binario sin archivos externos.

## Licencia

MIT

