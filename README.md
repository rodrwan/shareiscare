# ShareIsCare

ShareIsCare is a small application that works as an HTTP server to share files from a specific folder.

## Preview

![ShareIsCare Interface](frontend.jpeg)

## Features

- Simple and responsive web interface
- Configuration through YAML file
- Displays list of files with sizes
- Text file content visualization
- Configuration generation through command
- Implementation with templ templates
- Everything packaged in a single binary
- **Authentication system** to protect files
- Improved interface with dark mode

## Installation

```bash
# Clone the repository
git clone https://github.com/rodrwan/shareiscare.git
cd shareiscare

# Method 1: Using Makefile (recommended)
make install       # Install dependencies
make build         # Compile the project
make init-config   # Generate default configuration

# Method 2: Manual commands
go mod tidy
go install github.com/a-h/templ/cmd/templ@latest
templ generate
go build -o shareiscare main.go
```

## Building and Development

The project includes a Makefile with several useful commands:

```bash
# Show help
make help

# Build the application
make build

# Run the application in development mode
make run
make dev

# Generate code from templ templates
make generate

# Build for different platforms
make build-linux
make build-windows
make build-mac
make cross-build    # Build for all platforms

# Other useful tasks
make clean         # Clean generated files
make deps          # Update dependencies
make init-config   # Generate default configuration file
```

## Configuration

Configuration is done through the `config.yaml` file. If it doesn't exist, it will be automatically created with default values when starting the application.

### Generate configuration with command

```bash
# Using make
make init-config

# Or directly with the executable
./shareiscare init

# Generate in a specific location
./shareiscare init my-config.yaml
```

### Configuration file structure

```yaml
# Example config.yaml
port: 8080           # Port on which the server will run
root_dir: "."        # Root directory to serve files
title: "ShareIsCare" # Title for the web interface
username: "admin"    # Username for authentication (change for security)
password: "shareiscare" # Password for authentication (change for security)
secret_key: "random_key" # Key for signing sessions (automatically generated)
```

## Authentication

The application now includes an authentication system to protect files:

- Login is required to access files
- Default credentials: admin/shareiscare (change in config.yaml)
- The session is maintained via cookies signed with the secret key

## Usage

```bash
# View help
./shareiscare help

# Start the server
./shareiscare
```

Then open your browser at http://localhost:8080 to access the web interface.
Use the credentials configured in `config.yaml` to log in.

## Distribution

To distribute the application, simply build the binary and distribute it:

```bash
# Build for all platforms
make cross-build

# The binaries will be available in the ./build/ folder
```

## Creating Releases

The project is configured to generate automatic releases on GitHub when version tags are created. To create a new release:

```bash
# Create a new tag and release
make release v=1.0.0  # Replace 1.0.0 with the desired version number
```

This command:
1. Creates a Git tag with the format `v1.0.0`
2. Pushes the tag to GitHub
3. Triggers the GitHub Actions workflow
4. Automatically builds binaries for all platforms
5. Creates a GitHub release with the attached binaries

The releases will be available on the repository's [releases page](https://github.com/rodrwan/shareiscare/releases).

## Architecture

The project uses:
- Go as the base language
- Templ for HTML templates
- YAML for configuration
- Tailwind CSS for styles
- Native authentication system with sessions

The templates are compiled to Go code, allowing everything to be packaged in a single binary without external files.

## License

MIT

# Buy me a Coffee

<a href="https://buymeacoffee.com/roddotcom" target="_blank"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" style="height: 41px !important;width: 174px !important;box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;-webkit-box-shadow: 0px 3px 2px 0px rgba(190, 190, 190, 0.5) !important;" ></a>