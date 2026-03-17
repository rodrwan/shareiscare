# shareiscare

![shareiscare](shareiscare.png)

**Share any folder to the internet. One command. No cloudflared. No ngrok.**
**Your files, your tunnel, your rules.**

```
Browser → *.shareiscare.dev → Cloudflare Worker (Durable Object) ←WebSocket→ your machine
```

---

## Installation

### Homebrew

```bash
brew install rodrwan/tap/shareiscare
```

### From source

```bash
git clone https://github.com/rodrwan/shareiscare.git
cd shareiscare
make build    # → ./bin/shareiscare
```

The binary is self-contained — UIs are embedded via `go:embed`.

---

## Quick Start

```bash
# Share the current directory
shareiscare

# Share a specific folder with a custom hash
shareiscare --hash myproject --dir ./dist

# With password and bandwidth limit
shareiscare --password secret123 --max-bandwidth 1024
```

```
📁 Sharing: /Users/rod/projects/my-app
🌍 Public:  https://a1b2c3d4e5f67890.shareiscare.dev
🔐 Admin:   http://127.0.0.1:9898?token=f3a1b2c3...
🔒 Password protection enabled
📊 Daily bandwidth limit: 1024 MB
✅ Tunnel connected
```

The URL is **stable across restarts** — the hash and tunnel secret are persisted in `.shareiscare.json`. To force a new URL: `shareiscare --new-hash`.

---

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--hash` | auto (16-char hex) | Subdomain hash for your URL |
| `--new-hash` | `false` | Force a new hash and URL |
| `--dir` | `.` | Directory to share |
| `--password` | — | Password for public access (HTTP Basic Auth) |
| `--max-bandwidth` | `0` (unlimited) | Daily bandwidth limit in MB |
| `--admin-port` | `9898` | Admin panel port |
| `--config` | `<dir>/.shareiscare.json` | Path to config file |
| `--no-admin` | `false` | Disable admin panel |
| `--no-defaults` | `false` | Don't seed default sensitive patterns |
| `--max-zip` | `100` MB | Max size for ZIP downloads |
| `--version` | — | Print version and exit |

---

## Admin Panel

shareiscare starts a local admin server on `127.0.0.1:9898` (only accessible from your machine) protected with an auto-generated token.

From there you can:

- **Hide/show files** with gitignore-style patterns (e.g. `*.log`, `secrets/`)
- **Enable/disable default patterns** (`.env`, `.git/`, `*.key`, `*.pem`, `node_modules/`, etc.)
- **View the file tree** with visibility status

Rules are persisted in `.shareiscare.json`.

### API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/__admin/api/rules` | `GET` | List rules |
| `/__admin/api/rules` | `POST` | Add rule |
| `/__admin/api/rules` | `DELETE` | Remove rule |
| `/__admin/api/defaults` | `GET` / `PUT` | Query/toggle default patterns |
| `/__admin/api/tree` | `GET` | Full tree with visibility |

Auth via `?token=<token>` or `Authorization: Bearer <token>`.

---

## Security & Protections

### File Protection (Go client)

- **Default sensitive patterns**: `.env`, `.git/`, `*.key`, `*.pem`, `*.sqlite`, `credentials`, etc. — automatically hidden on startup
- **Dotfiles**: never served
- **Symlinks**: validated to not escape the root directory
- **404 instead of 403**: hidden files return 404 to avoid leaking their existence
- **Security headers**: `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, CSP

### Cloudflare Account Protection (Worker)

| Protection | Variable (`wrangler.toml`) | Default |
|-----------|---------------------------|---------|
| Rate limit per IP | `RATE_LIMIT_RPM` | 60 req/min |
| Hash validation | — | `[a-z0-9]{4,64}` |
| Request body limit | `MAX_REQUEST_BODY_MB` | 100 MB |
| Active tunnel limit | `MAX_TUNNELS` | 50 |
| Orphan DO cleanup | `CLEANUP_HOURS` | 24 h |

The tunnel limit uses an internal **registry DO** that maintains an atomic counter. When a DO is cleaned up due to inactivity, the counter is decremented automatically.

All values are configurable in `worker/wrangler.toml`:

```toml
[vars]
MAX_TUNNELS = "50"
MAX_REQUEST_BODY_MB = "100"
RATE_LIMIT_RPM = "60"
CLEANUP_HOURS = "24"
```

### Optional Protections (flags)

| Protection | Flag |
|-----------|------|
| Password (HTTP Basic Auth) | `--password` |
| Daily bandwidth limit | `--max-bandwidth` |

---

## Architecture

```
┌─────────────┐        ┌──────────────────────────────┐
│   Browser   │  HTTPS  │  Cloudflare Edge             │
│  (visitor)  │ ──────▶ │  Worker → TunnelDO (by hash) │
└─────────────┘        └──────────────┬───────────────┘
                                      │ WebSocket (wss://)
                       ┌──────────────▼───────────────┐
                       │  shareiscare (Go binary)     │
                       │                              │
                       │  shareHandler  → files       │
                       │  adminHandler  → rules       │
                       │  RulesEngine   → visibility  │
                       └──────────────────────────────┘
```

### Request Flow

1. Browser requests `https://myHash.shareiscare.dev/docs/`
2. Worker validates hash + rate limit, serializes the request to JSON (body in base64)
3. Durable Object forwards via WebSocket to the Go client
4. `shareHandler` checks password, rules and bandwidth, serves the file **in memory**
5. Response travels back via WebSocket → Worker → Browser

### Why Durable Objects

A regular Worker is stateless — it can't maintain a WebSocket across requests. Durable Objects are stateful instances at the edge: each hash maps to a unique DO that maintains the WebSocket and the pending request map. It uses the **WebSocket Hibernation API** so the DO sleeps between messages and only consumes CPU during actual activity.

---

## Self-hosting

To use your own domain instead of `shareiscare.dev`:

### 1. Worker Setup

```bash
npm install -g wrangler
wrangler login
```

Edit `worker/wrangler.toml`:

```toml
[[routes]]
pattern = "*.yourdomain.com/*"
zone_name = "yourdomain.com"
```

### 2. Wildcard DNS

In Cloudflare Dashboard → your zone → DNS:

| Type | Name | Address | Proxy |
|------|------|---------|-------|
| A | * | 192.0.2.1 | Proxied |

(The IP is a placeholder — the Worker intercepts first)

### 3. Deploy

```bash
make deploy
```

### 4. Update domain in the client

In `cmd/shareiscare/main.go`, replace `shareiscare.dev` with your domain and recompile with `make build`.

---

## Development

```bash
make build              # Compile → ./bin/shareiscare
make run                # Build + run (current directory)
make run-hash HASH=abc  # Run with specific hash
make vet                # go vet
make test               # Tests
make deploy             # Deploy Worker to Cloudflare
make dev                # Worker in local dev mode
```

### Releases

Published automatically via [GoReleaser](https://goreleaser.com/) when pushing a `v*` tag. CI runs tests, generates binaries for darwin/linux (amd64/arm64), and publishes the formula to [rodrwan/homebrew-tap](https://github.com/rodrwan/homebrew-tap).

---

## Troubleshooting

| Error | Cause | Solution |
|-------|-------|----------|
| `tunnel: connect failed: no such host` | Worker not deployed or wildcard DNS missing | `dig +short @1.1.1.1 test.yourdomain.com` |
| `503 No client connected` | Go binary is not running | Start `shareiscare` |
| `429 Rate limit exceeded` | Too many requests from your IP | Wait 60s or adjust `RATE_LIMIT_RPM` |
| `429 tunnel limit reached` | Active tunnel limit reached | Adjust `MAX_TUNNELS` |
| `403 invalid secret` | DO secret mismatch | Use `--new-hash` or wait for cleanup |
| `invalid hash` | Hash doesn't match `[a-z0-9]{4,64}` | Use only lowercase letters and digits |
| `Durable Objects not supported` | Free Workers plan | Enable Workers Paid (~$5/month) |
| Wildcard DNS not working | DNS record in DNS-only mode | Switch to **Proxied** (orange cloud) |

---

## License

MIT
