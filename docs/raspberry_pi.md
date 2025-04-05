# Using ShareIsCare on Raspberry Pi

This document provides instructions for installing and running ShareIsCare on Raspberry Pi devices.

## Compatible models

ShareIsCare offers compiled binaries for different ARM architectures:

- **ARMv7 (shareiscare-linux-armv7)**:
  - Raspberry Pi 2
  - Raspberry Pi 3
  - Raspberry Pi 4
  - Raspberry Pi 400

- **ARMv6 (shareiscare-linux-armv6)**:
  - Raspberry Pi 1 (all versions)
  - Raspberry Pi Zero
  - Raspberry Pi Zero W/WH
  - Raspberry Pi Zero 2 W

## Requirements

- Raspberry Pi (any model with Raspberry Pi OS / Raspbian)
- Internet connection (for initial download)
- Administrator permissions (for installation)

## Installation

### Method 1: Direct download from GitHub

1. Download the latest version for your model from the [releases page](https://github.com/rodrwan/shareiscare/releases)
   
   For Raspberry Pi 2, 3, 4, 400:
   ```bash
   wget https://github.com/rodrwan/shareiscare/releases/latest/download/shareiscare-linux-armv7.zip
   ```
   
   For Raspberry Pi 1, Zero, Zero W:
   ```bash
   wget https://github.com/rodrwan/shareiscare/releases/latest/download/shareiscare-linux-armv6.zip
   ```

2. Unzip the file:
   ```bash
   unzip shareiscare-linux-armv*.zip
   ```

3. Make the binary executable:
   ```bash
   chmod +x shareiscare-linux-armv*
   ```

4. Optional - Move the binary to a directory in the PATH:
   ```bash
   sudo mv shareiscare-linux-armv* /usr/local/bin/shareiscare
   ```

### Method 2: Build from source code

If you prefer to build from source code:

1. Install Go (if you don't have it already):
   ```bash
   sudo apt update
   sudo apt install golang
   ```

2. Clone the repository:
   ```bash
   git clone https://github.com/rodrwan/shareiscare.git
   cd shareiscare
   ```

3. Install templ (required tool for generating template code):
   ```bash
   go install github.com/a-h/templ/cmd/templ@latest
   ```

4. Build the project:
   
   For Raspberry Pi 2, 3, 4, 400:
   ```bash
   make build-raspberrypi
   ```
   
   For Raspberry Pi 1, Zero, Zero W:
   ```bash
   make build-raspberrypi-zero
   ```

   The binary will be available in `./bin/shareiscare-linux-armv7` or `./bin/shareiscare-linux-armv6` respectively.

## Configuration

1. Create a configuration file (if you're not using the one that comes with the download):
   ```bash
   ./shareiscare init
   ```

2. Edit the configuration according to your needs:
   ```bash
   nano config.yaml
   ```

   Example configuration:
   ```yaml
   # ShareIsCare Configuration
   port: 8080        # Port on which the server will run
   root_dir: "/home/pi/shared_files"     # Directory to share
   title: "My Raspberry Pi - File Sharing"
   ```

## Usage

1. Start the server:
   ```bash
   ./shareiscare
   ```

2. Access the server from a browser:
   - From the same Raspberry Pi: `http://localhost:8080`
   - From another device on the network: `http://YOUR_RASPBERRY_IP:8080`

   To find your Raspberry Pi's IP, you can use:
   ```bash
   hostname -I
   ```

## Run as a service

To make ShareIsCare start automatically when the Raspberry Pi boots:

1. Create a systemd service file:
   ```bash
   sudo nano /etc/systemd/system/shareiscare.service
   ```

2. Add the following content (adjust paths according to your configuration):
   ```
   [Unit]
   Description=ShareIsCare File Sharing Server
   After=network.target

   [Service]
   ExecStart=/usr/local/bin/shareiscare
   WorkingDirectory=/home/pi
   StandardOutput=inherit
   StandardError=inherit
   Restart=always
   User=pi

   [Install]
   WantedBy=multi-user.target
   ```

3. Enable and start the service:
   ```bash
   sudo systemctl enable shareiscare
   sudo systemctl start shareiscare
   ```

4. Check the status:
   ```bash
   sudo systemctl status shareiscare
   ```

## Troubleshooting

- **Blocked port**: Make sure the firewall allows connections to the configured port:
  ```bash
  sudo ufw allow 8080/tcp
  ```

- **File permissions**: Make sure the user running ShareIsCare has permissions on the configured directory:
  ```bash
  sudo chown -R pi:pi /path/to/directory
  ```

- **Service logs**: If you're running as a service, you can view logs with:
  ```bash
  sudo journalctl -u shareiscare
  ```

- **"Exec format error"**: This means you're using the wrong binary for your Raspberry Pi model. Check which model you have and use the appropriate binary (ARMv6 or ARMv7). 