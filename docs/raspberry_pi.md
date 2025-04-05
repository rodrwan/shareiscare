# Uso de ShareIsCare en Raspberry Pi

Este documento proporciona instrucciones para instalar y ejecutar ShareIsCare en dispositivos Raspberry Pi.

## Requisitos

- Raspberry Pi (cualquier modelo con Raspberry Pi OS / Raspbian)
- Conexión a Internet (para la descarga inicial)
- Permisos de administrador (para instalación)

## Instalación

### Método 1: Descarga directa desde GitHub

1. Descarga la última versión para ARM desde la [página de releases](https://github.com/rodrwan/shareiscare/releases)
   ```bash
   wget https://github.com/rodrwan/shareiscare/releases/latest/download/shareiscare-linux-arm.zip
   ```

2. Descomprime el archivo:
   ```bash
   unzip shareiscare-linux-arm.zip
   ```

3. Haz que el binario sea ejecutable:
   ```bash
   chmod +x shareiscare-linux-arm
   ```

4. Opcional - Mueve el binario a un directorio en el PATH:
   ```bash
   sudo mv shareiscare-linux-arm /usr/local/bin/shareiscare
   ```

### Método 2: Compilar desde el código fuente

Si prefieres compilar desde el código fuente:

1. Instala Go (si aún no lo tienes):
   ```bash
   sudo apt update
   sudo apt install golang
   ```

2. Clona el repositorio:
   ```bash
   git clone https://github.com/rodrwan/shareiscare.git
   cd shareiscare
   ```

3. Instala templ (herramienta necesaria para generar código de plantillas):
   ```bash
   go install github.com/a-h/templ/cmd/templ@latest
   ```

4. Compila el proyecto:
   ```bash
   make build-raspberrypi
   ```

   El binario estará disponible en `./bin/shareiscare-linux-arm`

## Configuración

1. Crea un archivo de configuración (si no usas el que viene con la descarga):
   ```bash
   ./shareiscare init
   ```

2. Edita la configuración según tus necesidades:
   ```bash
   nano config.yaml
   ```

   Ejemplo de configuración:
   ```yaml
   # Configuración de ShareIsCare
   port: 8080        # Puerto en el que se ejecutará el servidor
   root_dir: "/home/pi/shared_files"     # Directorio a compartir
   title: "Mi Raspberry Pi - Compartición de archivos"
   ```

## Uso

1. Inicia el servidor:
   ```bash
   ./shareiscare
   ```

2. Accede al servidor desde un navegador:
   - Desde la misma Raspberry Pi: `http://localhost:8080`
   - Desde otro dispositivo en la red: `http://IP_DE_TU_RASPBERRY:8080`

   Para encontrar la IP de tu Raspberry Pi, puedes usar:
   ```bash
   hostname -I
   ```

## Ejecutar como servicio

Para que ShareIsCare se inicie automáticamente al arrancar la Raspberry Pi:

1. Crea un archivo de servicio systemd:
   ```bash
   sudo nano /etc/systemd/system/shareiscare.service
   ```

2. Añade el siguiente contenido (ajusta las rutas según tu configuración):
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

3. Habilita e inicia el servicio:
   ```bash
   sudo systemctl enable shareiscare
   sudo systemctl start shareiscare
   ```

4. Verifica el estado:
   ```bash
   sudo systemctl status shareiscare
   ```

## Resolución de problemas

- **Puerto bloqueado**: Asegúrate de que el firewall permita conexiones al puerto configurado:
  ```bash
  sudo ufw allow 8080/tcp
  ```

- **Permisos de archivos**: Asegúrate de que el usuario que ejecuta ShareIsCare tenga permisos en el directorio configurado:
  ```bash
  sudo chown -R pi:pi /ruta/al/directorio
  ```

- **Logs del servicio**: Si estás ejecutando como servicio, puedes ver los logs con:
  ```bash
  sudo journalctl -u shareiscare
  ``` 