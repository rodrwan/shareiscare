#!/bin/bash

# Script para ejecutar tests unitarios de ShareIsCare

set -e  # Salir si hay errores

echo "=== Ejecutando tests unitarios de ShareIsCare ==="

# Limpieza de archivos temporales que puedan existir de ejecuciones anteriores
echo "Limpiando archivos temporales..."
if [ -f config.yaml.bak ]; then
    mv config.yaml.bak config.yaml
fi

# Ejecutar los tests
echo "Ejecutando tests..."
go test -v ./...

# Verificar que el código compile para ARM (Raspberry Pi)
echo "Verificando compilación para Raspberry Pi..."
if GOOS=linux GOARCH=arm GOARM=7 go build -o /tmp/shareiscare-arm-test main.go; then
    echo "✓ La compilación para Raspberry Pi es correcta"
    rm /tmp/shareiscare-arm-test
else
    echo "✗ Error en la compilación para Raspberry Pi"
    exit 1
fi

echo "=== Tests completados ==="