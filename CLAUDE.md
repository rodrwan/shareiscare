# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make build              # Compile binary to ./bin/shareiscare
make run                # Build + run with auto-generated hash, sharing current dir
make run-hash HASH=abc DIR=./mydir  # Run with specific hash and directory
make vet                # Run go vet
make test               # Run tests (go test ./cmd/shareiscare/ -v)
make clean              # Remove compiled binary
make deploy             # Deploy Cloudflare Worker (requires Node >= 20)
make dev                # Run Worker in local dev mode
```

Direct build: `go build -o ./bin/shareiscare ./cmd/shareiscare/`

## Architecture

ShareIsCare is a local file-sharing service that exposes files via a Cloudflare Worker tunnel. It consists of two components:

### Go Binary (`cmd/shareiscare/`)

A single binary with two HTTP servers:

- **Public shareHandler** — serves files through the WebSocket tunnel. Routes: `/__api/ls` (JSON directory listing) and `/*` (file serving / directory index via embedded `index.html`).
- **Admin adminHandler** — local-only server on `127.0.0.1:9898` with token auth. Manages visibility rules via REST API (`/__admin/api/rules`, `/__admin/api/defaults`, `/__admin/api/tree`). UI embedded via `admin.html`.

Both handlers share a **RulesEngine** (`rules.go`) — thread-safe (sync.RWMutex) pattern matcher that determines file visibility. Config persists to `.shareiscare.json` with atomic writes (tmp + rename).

Key files:
- `main.go` — entry point, flag parsing, shareHandler, tunnel WebSocket client, responseRecorder
- `rules.go` — RulesEngine, gitignore-style pattern matching, default sensitive patterns
- `admin.go` — admin HTTP handler, token auth, tree builder, rule CRUD API
- `index.html` / `admin.html` — embedded UIs (go:embed), vanilla JS, dark theme

### Cloudflare Worker (`worker/`)

- `worker.js` — TunnelDO Durable Object with WebSocket Hibernation API. Routes requests by subdomain hash to the corresponding DO instance, which proxies HTTP over WebSocket to the local Go client.
- `wrangler.toml` — Worker config. Uses `new_sqlite_classes` migration (required for free plan). Routes `*.shareiscare.dev/*`.

### Request Flow

```
Browser → Cloudflare Worker → TunnelDO → WebSocket → Go client (shareHandler) → response back
```

Requests are serialized as JSON with base64 bodies over WebSocket. The Go client uses a `responseRecorder` to capture HTTP responses in memory. A `connWriter` with mutex serializes all WebSocket writes to prevent concurrent write panics.

## Key Patterns

- **Pattern matching** uses gitignore semantics: patterns without `/` match basename, with `/` match full relative path, trailing `/` matches directories only. Implementation uses `filepath.Match`.
- **Hidden file enforcement** returns 404 (not 403) to avoid leaking existence. `IsPathOrAncestorHidden` checks every path segment.
- **Admin auth** uses a random 16-byte hex token, accepted via `?token=` query param or `Authorization: Bearer` header.
- **Both HTML files** are embedded at compile time via `//go:embed`. They use the same visual style: dark theme, IBM Plex Mono, Playfair Display, gold accent (`#c8a96e`).
- **No frameworks** — all frontend code is vanilla HTML/CSS/JS.

## CLI Flags

```
--version       Print version and exit
--hash          Subdomain hash (auto-generated 16-char hex if omitted)
--dir           Directory to share (default: ".")
--admin-port    Admin panel port (default: "9898")
--config        Path to .shareiscare.json (default: <dir>/.shareiscare.json)
--no-admin      Disable admin panel
--no-defaults   Don't seed default sensitive patterns
--max-zip       Max total size for ZIP downloads in bytes (default: 100MB)
--new-hash      Force a new hash and URL, ignoring persisted values
--password      Require password for public access (HTTP Basic Auth)
--max-bandwidth Max daily bandwidth in MB (0 = unlimited, default: 0)
```

## Language

Code comments and log messages are in Spanish. UI text is in English.
