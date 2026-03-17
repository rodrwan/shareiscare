# shareiscare

![shareiscare](shareiscare.png)

Comparte cualquier carpeta de tu máquina local en internet con una URL pública
tipo `https://<hash>.shareiscare.dev`, sin instalar cloudflared ni ningún binario
externo. Todo el stack es tuyo: un Worker de Cloudflare y un pequeño binario Go.

```
Browser → *.shareiscare.dev → Worker (Durable Object) ←WebSocket→ tu máquina
```

---

## Tabla de contenidos

1. [Arquitectura](#1-arquitectura)
2. [Requisitos](#2-requisitos)
3. [Estructura del proyecto](#3-estructura-del-proyecto)
4. [Instalación](#4-instalación)
5. [Uso](#5-uso)
6. [Panel de administración](#6-panel-de-administración)
7. [Protecciones anti-abuso](#7-protecciones-anti-abuso)
8. [Cómo funciona por dentro](#8-cómo-funciona-por-dentro)
9. [Self-hosting (tu propio dominio)](#9-self-hosting-tu-propio-dominio)
10. [Solución de problemas](#10-solución-de-problemas)

---

## 1. Arquitectura

```
┌─────────────┐   HTTPS    ┌──────────────────────────────┐
│   Browser   │ ─────────▶ │  Cloudflare Edge             │
│  (visitante)│            │  Worker: shareiscare         │
└─────────────┘            │  Durable Object: TunnelDO    │
                           │                              │
                           │  Una instancia DO por hash   │
                           │  WebSocket Hibernation API   │
                           └──────────────┬───────────────┘
                                          │ WebSocket (wss://)
                                          │
                           ┌──────────────▼───────────────┐
                           │  shareiscare (binario Go)    │
                           │  corriendo en tu máquina     │
                           │                              │
                           │  ┌──────────────────────┐    │
                           │  │  shareHandler (pub)   │    │
                           │  │  adminHandler (local) │    │
                           │  │  RulesEngine          │    │
                           │  └──────────────────────┘    │
                           └──────────────────────────────┘
```

### Flujo de una request

1. El browser pide `https://miHash.shareiscare.dev/docs/`
2. El Worker valida el hash y el rate limit, luego serializa la request a JSON (método, path, headers, body en base64)
3. El Worker envía ese JSON por el WebSocket al Durable Object del hash, que lo reenvía al cliente Go
4. El cliente Go reconstruye un `http.Request` real y lo pasa al `shareHandler` **en memoria** (sin tocar ningún puerto TCP local)
5. El `shareHandler` verifica password, reglas de visibilidad, y límite de ancho de banda antes de servir
6. Captura la respuesta con un `responseRecorder`, la serializa a JSON y la devuelve por el WebSocket
7. El Worker deserializa y responde al browser con el `Response` original

---

## 2. Requisitos

| Componente | Versión mínima | Para qué |
|---|---|---|
| Go | 1.25+ | compilar el cliente |
| Node.js | 20+ | desplegar el Worker con wrangler |
| Cuenta Cloudflare | Plan **Workers Paid** (~$5/mes) | Durable Objects requieren plan de pago |

> **Nota sobre el plan:** El plan gratuito de Workers no incluye Durable Objects.
> Los primeros 1 millón de requests/mes y 1 GB·s de DO están incluidos en el plan Paid.
> Para uso personal/compartir archivos ocasionalmente no superarás esos límites.

---

## 3. Estructura del proyecto

```
shareiscare/
├── cmd/shareiscare/         # Binario Go — corre en tu máquina
│   ├── main.go              # Entry point, shareHandler, tunnel WebSocket client
│   ├── rules.go             # RulesEngine: visibilidad de archivos, persistencia
│   ├── admin.go             # Admin handler: token auth, rule CRUD, tree API
│   ├── admin.html           # UI del panel admin (go:embed)
│   ├── index.html           # UI pública del file browser (go:embed)
│   └── main_test.go         # Tests
├── worker/                  # Cloudflare Worker — se despliega una sola vez
│   ├── wrangler.toml        # Configuración, rutas, y vars de protección
│   └── src/
│       └── worker.js        # Router, rate limiter, TunnelDO + registry
├── web/
│   └── index.html           # Landing page del proyecto
├── Makefile                 # Build, test, deploy shortcuts
├── .goreleaser.yaml         # Config de GoReleaser + Homebrew tap
└── .github/workflows/
    └── release.yml          # CI: tests + release on tag push
```

---

## 4. Instalación

### Homebrew (macOS/Linux)

```bash
brew install rodrwan/tap/shareiscare
```

### Desde source

```bash
git clone https://github.com/rodrwan/shareiscare.git
cd shareiscare
make build
# Binario en ./bin/shareiscare
```

### Cross-compile

```bash
GOOS=linux GOARCH=amd64 go build -o shareiscare-linux ./cmd/shareiscare/
GOOS=darwin GOARCH=arm64 go build -o shareiscare-mac ./cmd/shareiscare/
GOOS=windows GOARCH=amd64 go build -o shareiscare.exe ./cmd/shareiscare/
```

El binario incluye `index.html` y `admin.html` embebidos (`//go:embed`). No necesita archivos externos.

---

## 5. Uso

### Compartir una carpeta

```bash
# Compartir el directorio actual (hash auto-generado, URL estable entre reinicios)
shareiscare

# Compartir una carpeta específica
shareiscare --dir ~/Pictures/vacaciones

# Con hash personalizado
shareiscare --hash miproyecto --dir ./dist

# Con password para acceso público
shareiscare --password secreto123

# Con límite de ancho de banda (1 GB/día)
shareiscare --max-bandwidth 1024
```

### Salida al correr

```
📁 Sharing: /Users/rod/proyectos/mi-app
🌍 Public:  https://a1b2c3d4e5f67890.shareiscare.dev
🔐 Admin:   http://127.0.0.1:9898?token=abc123...
🔒 Password protection enabled
📊 Daily bandwidth limit: 1024 MB
✅ Tunnel connected
```

### URL estable

El hash y el tunnel secret se persisten en `.shareiscare.json`. Al reiniciar, se reutiliza la misma URL automáticamente. Para forzar una URL nueva:

```bash
shareiscare --new-hash
```

### Detener

`Ctrl+C` — el Worker detecta que el WebSocket se cerró y responderá `503` hasta que vuelvas a conectar.

### Reconexión automática

Si se cae la conexión, el cliente reconecta automáticamente con backoff exponencial: 2s → 4s → 8s → ... → 60s máximo.

### Flags completos

| Flag | Default | Descripción |
|------|---------|-------------|
| `--hash` | auto (16-char hex) | Subdomain hash para tu URL |
| `--new-hash` | `false` | Forzar hash nuevo, ignorando el persistido |
| `--dir` | `.` | Directorio a compartir |
| `--password` | — | Password para acceso público (HTTP Basic Auth) |
| `--max-bandwidth` | `0` (ilimitado) | Ancho de banda diario máximo en MB |
| `--admin-port` | `9898` | Puerto del panel admin |
| `--config` | `<dir>/.shareiscare.json` | Path al archivo de configuración |
| `--no-admin` | `false` | Desactivar panel admin |
| `--no-defaults` | `false` | No sembrar patrones sensibles por defecto |
| `--max-zip` | 100 MB | Tamaño máximo para descargas ZIP |
| `--version` | — | Imprimir versión y salir |

---

## 6. Panel de administración

Al iniciar, shareiscare levanta un servidor admin local en `127.0.0.1:9898` (solo accesible desde tu máquina) con autenticación por token.

### Funcionalidades

- **Reglas de visibilidad**: ocultar/mostrar archivos y carpetas con patrones tipo gitignore
- **Patrones por defecto**: `.env`, `.git/`, `*.key`, `*.pem`, `node_modules/`, etc. — activables/desactivables
- **Vista de árbol**: visualizar qué archivos son visibles y cuáles están ocultos
- **Persistencia**: las reglas se guardan en `.shareiscare.json`

### API REST

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/__admin/api/rules` | GET | Listar todas las reglas |
| `/__admin/api/rules` | POST | Agregar regla (`{"pattern": "*.log", "action": "hide"}`) |
| `/__admin/api/rules` | DELETE | Eliminar regla (`{"pattern": "*.log"}`) |
| `/__admin/api/defaults` | GET | Estado de patrones por defecto |
| `/__admin/api/defaults` | PUT | Activar/desactivar (`{"enabled": true}`) |
| `/__admin/api/tree` | GET | Árbol completo con estado de visibilidad |

Auth: `?token=<token>` o header `Authorization: Bearer <token>`.

---

## 7. Protecciones anti-abuso

Protecciones integradas para evitar la explotación de la cuenta de Cloudflare:

### En el Worker

| Protección | Config (wrangler.toml) | Default |
|-----------|----------------------|---------|
| **Rate limit por IP** | `RATE_LIMIT_RPM` | 60 req/min |
| **Validación de hash** | — | Solo `[a-z0-9]{4,64}` |
| **Límite de body** | `MAX_REQUEST_BODY_MB` | 10 MB |
| **Límite de tunnels activos** | `MAX_TUNNELS` | 50 |
| **Cleanup de DOs huérfanos** | `CLEANUP_HOURS` | 24 horas |

El límite de tunnels se maneja via un **registry DO** interno (`__registry__`) que mantiene un contador atómico de tunnels activos. Cuando un DO se limpia por inactividad, decrementa el contador.

Todos los valores son configurables en `worker/wrangler.toml` bajo `[vars]`:

```toml
[vars]
MAX_TUNNELS = "50"
MAX_REQUEST_BODY_MB = "10"
RATE_LIMIT_RPM = "60"
CLEANUP_HOURS = "24"
```

### En el cliente Go

| Protección | Flag | Default |
|-----------|------|---------|
| **Password (HTTP Basic Auth)** | `--password` | desactivado |
| **Bandwidth diario** | `--max-bandwidth` | ilimitado |
| **Patrones sensibles** | (automático) | `.env`, `*.key`, `.git/`, etc. |
| **Protección de symlinks** | (automático) | Bloquea symlinks que escapan del root |
| **Archivos ocultos** | (automático) | Dotfiles no se sirven |

### Seguridad por defecto

- Los archivos ocultos por reglas retornan **404** (no 403) para no filtrar existencia
- El admin panel solo escucha en **127.0.0.1** (localhost)
- Headers de seguridad: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Content-Security-Policy`
- El tunnel secret se valida en el DO — un secret incorrecto recibe **403**

---

## 8. Cómo funciona por dentro

### El cliente Go

**`shareHandler`** implementa `http.Handler` con estas rutas:

- `GET /__api/ls?path=<ruta>` — JSON con listado del directorio (filtrado por reglas)
- `GET /__api/zip?path=<ruta>` — Descarga ZIP de un directorio completo
- `GET /__api/zip-info?path=<ruta>` — Info previa al ZIP (tamaño, cantidad de archivos)
- `GET /*` — Directorio → sirve `index.html` embebido. Archivo → `http.ServeFile`

**`RulesEngine`** maneja visibilidad con patrones tipo gitignore, thread-safe via `sync.RWMutex`. Persiste a `.shareiscare.json` con escritura atómica (tmp + rename). También almacena el hash y tunnel secret para URLs estables.

**`adminHandler`** sirve la UI admin y expone la API REST de reglas. Auth por token random de 16 bytes hex.

**`responseRecorder`** es un `http.ResponseWriter` falso que captura status code, headers y body en memoria (límite 256 MB) sin abrir puertos TCP locales.

**`runTunnel`** abre un WebSocket hacia `wss://<hash>.shareiscare.dev/__tunnel_connect?secret=<secret>` con reconexión automática y backoff exponencial.

### El Worker

**`TunnelDO`** (Durable Object) usa la **WebSocket Hibernation API** — el DO duerme entre mensajes y solo se cobra CPU durante actividad real.

Mantiene:
- Un WebSocket hacia el cliente local (tag `tunnel`)
- `this.pending` — `Map<requestId, {resolve, reject, timer}>` para requests en vuelo (timeout 30s)
- `tunnelSecret` en storage durable — validado en cada conexión

**Registry DO** — una instancia especial del mismo `TunnelDO` (hash `__registry__`) que mantiene un contador de tunnels activos para enforcer el límite.

**Router** — valida hash (regex), aplica rate limit por IP, y rutea al DO correspondiente.

### La UI

Single-page app en HTML/JS vanilla (sin frameworks) embebida en el binario Go.
Usa `fetch('/__api/ls?path=...')` para listar directorios y `history.pushState`
para navegar sin recargar. Soporta descarga ZIP de carpetas completas. Dark theme
con IBM Plex Mono, Playfair Display, y acento dorado (`#c8a96e`).

---

## 9. Self-hosting (tu propio dominio)

### 9.1 Setup del Worker (una sola vez)

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

El dominio debe estar **activo en Cloudflare** (nameservers apuntando a CF).

### 9.2 Configurar wildcard DNS

En Cloudflare Dashboard → tu zona → DNS → Add record:

| Type | Name | IPv4 address | Proxy status |
|------|------|-------------|--------------|
| A    | *    | 192.0.2.1   | Proxied      |

La IP `192.0.2.1` es un placeholder — el Worker intercepta antes de que llegue a cualquier servidor.

### 9.3 Desplegar

```bash
make deploy
# o: cd worker && npx wrangler deploy
```

### 9.4 Ajustar límites

Edita `[vars]` en `worker/wrangler.toml` según tu uso esperado y redespliega.

### 9.5 Actualizar el dominio en el cliente

En `cmd/shareiscare/main.go`, busca `shareiscare.dev` y reemplaza por tu dominio. Recompila:

```bash
make build
```

---

## 10. Solución de problemas

### `tunnel: connect failed: dial tcp: no such host`
El Worker no está desplegado o el DNS wildcard no está configurado.
Verifica con: `dig +short @1.1.1.1 test.tudominio.com`

### `503 No hay cliente conectado`
El binario Go no está corriendo o se desconectó. Inicia `shareiscare --dir ...`

### `429 Rate limit exceeded`
Demasiadas requests desde tu IP. Espera 60 segundos o ajusta `RATE_LIMIT_RPM` en `wrangler.toml`.

### `429 tunnel limit reached`
Se alcanzó el límite de tunnels activos. Ajusta `MAX_TUNNELS` en `wrangler.toml`.

### `hash inválido: hash must be 4-64 characters`
El hash proporcionado con `--hash` no cumple los requisitos: solo letras minúsculas y dígitos, entre 4 y 64 caracteres.

### `403 invalid secret`
El tunnel secret guardado en el DO no coincide con el del cliente. Usa `--new-hash` para generar identidad nueva, o espera a que el DO se limpie (por defecto 24h).

### Los archivos se ven pero las subcarpetas dan 404
Verifica que el `--dir` apunte a la raíz correcta y que tengas permisos de lectura.

### `Error: Durable Objects are not supported in your plan`
Necesitas activar el plan Workers Paid en el dashboard de Cloudflare.

### El Worker despliega pero no captura el wildcard
Asegúrate de que el registro DNS `*` esté en modo **Proxied** (nube naranja), no DNS-only (nube gris).

---

## Desarrollo

```bash
make build              # Compilar binario a ./bin/shareiscare
make run                # Build + run compartiendo directorio actual
make run-hash HASH=abc  # Run con hash específico
make vet                # go vet
make test               # Tests
make deploy             # Deploy Worker a Cloudflare
make dev                # Worker en modo dev local
```

### Releases

Se publican automáticamente via GoReleaser cuando se pushea un tag `v*`. El CI corre tests, genera binarios para darwin/linux (amd64/arm64), y publica a Homebrew via `rodrwan/homebrew-tap`.

---

## Licencia

MIT — haz lo que quieras con esto.
