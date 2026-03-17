# shareiscare

![shareiscare](shareiscare.png)

**Share any folder to the internet. One command. No cloudflared. No ngrok.**
**Your files, your tunnel, your rules.**

```
Browser → *.shareiscare.dev → Cloudflare Worker (Durable Object) ←WebSocket→ tu máquina
```

---

## Instalación

### Homebrew

```bash
brew install rodrwan/tap/shareiscare
```

### Desde source

```bash
git clone https://github.com/rodrwan/shareiscare.git
cd shareiscare
make build    # → ./bin/shareiscare
```

El binario es self-contained — las UIs están embebidas via `go:embed`.

---

## Quick Start

```bash
# Compartir el directorio actual
shareiscare

# Compartir una carpeta específica con hash personalizado
shareiscare --hash miproyecto --dir ./dist

# Con password y límite de ancho de banda
shareiscare --password secreto123 --max-bandwidth 1024
```

```
📁 Sharing: /Users/rod/proyectos/mi-app
🌍 Public:  https://a1b2c3d4e5f67890.shareiscare.dev
🔐 Admin:   http://127.0.0.1:9898?token=f3a1b2c3...
🔒 Password protection enabled
📊 Daily bandwidth limit: 1024 MB
✅ Tunnel connected
```

La URL es **estable entre reinicios** — el hash y el tunnel secret se persisten en `.shareiscare.json`. Para forzar una URL nueva: `shareiscare --new-hash`.

---

## Flags

| Flag | Default | Descripción |
|------|---------|-------------|
| `--hash` | auto (16-char hex) | Hash del subdominio para tu URL |
| `--new-hash` | `false` | Forzar hash y URL nuevos |
| `--dir` | `.` | Directorio a compartir |
| `--password` | — | Password para acceso público (HTTP Basic Auth) |
| `--max-bandwidth` | `0` (ilimitado) | Límite de ancho de banda diario en MB |
| `--admin-port` | `9898` | Puerto del panel de administración |
| `--config` | `<dir>/.shareiscare.json` | Path al archivo de configuración |
| `--no-admin` | `false` | Desactivar panel de administración |
| `--no-defaults` | `false` | No sembrar patrones sensibles por defecto |
| `--max-zip` | `100` MB | Tamaño máximo para descargas ZIP |
| `--version` | — | Imprimir versión y salir |

---

## Panel de administración

shareiscare levanta un servidor admin local en `127.0.0.1:9898` (solo accesible desde tu máquina) protegido con un token auto-generado.

Desde ahí puedes:

- **Ocultar/mostrar archivos** con patrones tipo gitignore (ej. `*.log`, `secrets/`)
- **Activar/desactivar patrones por defecto** (`.env`, `.git/`, `*.key`, `*.pem`, `node_modules/`, etc.)
- **Visualizar el árbol** de archivos con su estado de visibilidad

Las reglas se persisten en `.shareiscare.json`.

### API

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/__admin/api/rules` | `GET` | Listar reglas |
| `/__admin/api/rules` | `POST` | Agregar regla |
| `/__admin/api/rules` | `DELETE` | Eliminar regla |
| `/__admin/api/defaults` | `GET` / `PUT` | Consultar/togglear patrones por defecto |
| `/__admin/api/tree` | `GET` | Árbol completo con visibilidad |

Auth via `?token=<token>` o `Authorization: Bearer <token>`.

---

## Seguridad y protecciones

### Protección de archivos (cliente Go)

- **Patrones sensibles por defecto**: `.env`, `.git/`, `*.key`, `*.pem`, `*.sqlite`, `credentials`, etc. — ocultos automáticamente al arrancar
- **Dotfiles**: nunca se sirven
- **Symlinks**: se valida que no escapen del directorio raíz
- **404 en vez de 403**: los archivos ocultos retornan 404 para no filtrar su existencia
- **Headers de seguridad**: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, CSP

### Protección de la cuenta Cloudflare (Worker)

| Protección | Variable (`wrangler.toml`) | Default |
|-----------|---------------------------|---------|
| Rate limit por IP | `RATE_LIMIT_RPM` | 60 req/min |
| Validación de hash | — | `[a-z0-9]{4,64}` |
| Límite de request body | `MAX_REQUEST_BODY_MB` | 100 MB |
| Límite de tunnels activos | `MAX_TUNNELS` | 50 |
| Limpieza de DOs huérfanos | `CLEANUP_HOURS` | 24 h |

El límite de tunnels usa un **registry DO** interno que mantiene un contador atómico. Cuando un DO se limpia por inactividad, decrementa el contador automáticamente.

Todos los valores son configurables en `worker/wrangler.toml`:

```toml
[vars]
MAX_TUNNELS = "50"
MAX_REQUEST_BODY_MB = "100"
RATE_LIMIT_RPM = "60"
CLEANUP_HOURS = "24"
```

### Protecciones opcionales (flags)

| Protección | Flag |
|-----------|------|
| Password (HTTP Basic Auth) | `--password` |
| Límite de ancho de banda diario | `--max-bandwidth` |

---

## Arquitectura

```
┌─────────────┐        ┌──────────────────────────────┐
│   Browser   │  HTTPS  │  Cloudflare Edge             │
│  (visitante)│ ──────▶ │  Worker → TunnelDO (por hash)│
└─────────────┘        └──────────────┬───────────────┘
                                      │ WebSocket (wss://)
                       ┌──────────────▼───────────────┐
                       │  shareiscare (binario Go)    │
                       │                              │
                       │  shareHandler  → archivos    │
                       │  adminHandler  → reglas      │
                       │  RulesEngine   → visibilidad │
                       └──────────────────────────────┘
```

### Flujo de una request

1. Browser pide `https://miHash.shareiscare.dev/docs/`
2. Worker valida hash + rate limit, serializa la request a JSON (body en base64)
3. Durable Object reenvía por WebSocket al cliente Go
4. `shareHandler` verifica password, reglas y bandwidth, sirve el archivo **en memoria**
5. Respuesta viaja de vuelta por WebSocket → Worker → Browser

### Por qué Durable Objects

Un Worker normal es stateless — no puede mantener un WebSocket entre requests. Los Durable Objects son instancias con estado en el edge: cada hash mapea a un DO único que mantiene el WebSocket y el mapa de requests pendientes. Usa la **WebSocket Hibernation API** para que el DO duerma entre mensajes y solo consuma CPU durante actividad real.

---

## Self-hosting

Para usar tu propio dominio en vez de `shareiscare.dev`:

### 1. Setup del Worker

```bash
npm install -g wrangler
wrangler login
```

Edita `worker/wrangler.toml`:

```toml
[[routes]]
pattern = "*.tudominio.com/*"
zone_name = "tudominio.com"
```

### 2. DNS wildcard

En Cloudflare Dashboard → tu zona → DNS:

| Type | Name | Address | Proxy |
|------|------|---------|-------|
| A | * | 192.0.2.1 | Proxied |

(La IP es un placeholder — el Worker intercepta antes)

### 3. Deploy

```bash
make deploy
```

### 4. Actualizar dominio en el cliente

En `cmd/shareiscare/main.go`, reemplaza `shareiscare.dev` por tu dominio y recompila con `make build`.

---

## Desarrollo

```bash
make build              # Compilar → ./bin/shareiscare
make run                # Build + run (directorio actual)
make run-hash HASH=abc  # Run con hash específico
make vet                # go vet
make test               # Tests
make deploy             # Deploy Worker a Cloudflare
make dev                # Worker en modo dev local
```

### Releases

Se publican automáticamente via [GoReleaser](https://goreleaser.com/) al pushear un tag `v*`. El CI corre tests, genera binarios para darwin/linux (amd64/arm64), y publica la formula a [rodrwan/homebrew-tap](https://github.com/rodrwan/homebrew-tap).

---

## Troubleshooting

| Error | Causa | Solución |
|-------|-------|----------|
| `tunnel: connect failed: no such host` | Worker no desplegado o DNS wildcard faltante | `dig +short @1.1.1.1 test.tudominio.com` |
| `503 No hay cliente conectado` | Binario Go no está corriendo | Iniciar `shareiscare` |
| `429 Rate limit exceeded` | Demasiadas requests desde tu IP | Esperar 60s o ajustar `RATE_LIMIT_RPM` |
| `429 tunnel limit reached` | Límite de tunnels activos alcanzado | Ajustar `MAX_TUNNELS` |
| `403 invalid secret` | Secret del DO no coincide | Usar `--new-hash` o esperar cleanup |
| `hash inválido` | Hash no cumple `[a-z0-9]{4,64}` | Usar solo minúsculas y dígitos |
| `Durable Objects not supported` | Plan gratuito de Workers | Activar Workers Paid (~$5/mes) |
| Wildcard DNS no funciona | Registro DNS en modo DNS-only | Cambiar a **Proxied** (nube naranja) |

---

## Licencia

MIT
