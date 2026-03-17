# Security Audit Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 11 security vulnerabilities in ShareIsCare focused on file protection and WebSocket tunnel hardening.

**Architecture:** Surgical fixes applied directly to existing files. No new files except tests. Go-side changes in `cmd/shareiscare/main.go`, Worker-side changes in `worker/src/worker.js`. A shared `isWithinRoot` helper replaces all unsafe `strings.HasPrefix` path checks. Tunnel authentication via shared secret stored durably in the Durable Object.

**Tech Stack:** Go 1.25, Cloudflare Workers (JS), gorilla/websocket, Durable Objects with Hibernation API.

**Spec:** `docs/superpowers/specs/2026-03-16-security-audit-design.md`

---

## Chunk 1: Path Safety (Fixes 4, 1, 5, 8)

### Task 1: Safe path prefix check + root resolution

**Files:**
- Modify: `cmd/shareiscare/main.go:42-50` (root resolution at startup)
- Modify: `cmd/shareiscare/main.go:119` (ServeHTTP prefix check)
- Modify: `cmd/shareiscare/main.go:155` (apiLs prefix check)
- Modify: `cmd/shareiscare/main.go:259` (collectFiles prefix check)
- Modify: `cmd/shareiscare/main.go:299` (apiZipInfo prefix check)
- Modify: `cmd/shareiscare/main.go:342` (apiZip prefix check)
- Test: `cmd/shareiscare/main_test.go`

- [ ] **Step 1: Write test for `isWithinRoot` helper**

Create `cmd/shareiscare/main_test.go`:

```go
package main

import (
	"testing"
)

func TestIsWithinRoot(t *testing.T) {
	tests := []struct {
		path   string
		root   string
		expect bool
	}{
		{"/home/user/share/file.txt", "/home/user/share", true},
		{"/home/user/share", "/home/user/share", true},
		{"/home/user/share/sub/file.txt", "/home/user/share", true},
		{"/home/user/shareevil/file.txt", "/home/user/share", false},
		{"/home/user/shar", "/home/user/share", false},
		{"/other/path", "/home/user/share", false},
	}
	for _, tt := range tests {
		got := isWithinRoot(tt.path, tt.root)
		if got != tt.expect {
			t.Errorf("isWithinRoot(%q, %q) = %v, want %v", tt.path, tt.root, got, tt.expect)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cmd/shareiscare && go test -run TestIsWithinRoot -v`
Expected: FAIL — `isWithinRoot` undefined

- [ ] **Step 3: Add `isWithinRoot` and `resolveAndCheck` helpers to `main.go`**

Add after the imports, before `func main()`:

```go
// isWithinRoot verifica que path esté dentro de root (evita el anti-patrón de HasPrefix sin separador).
func isWithinRoot(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// resolveAndCheck resuelve symlinks y verifica que el path resultante esté dentro de root.
func resolveAndCheck(path, root string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	if !isWithinRoot(resolved, root) {
		return "", fmt.Errorf("path escapes root")
	}
	return resolved, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd cmd/shareiscare && go test -run TestIsWithinRoot -v`
Expected: PASS

- [ ] **Step 5: Resolve root via EvalSymlinks at startup**

In `main()`, after `filepath.Abs(*dir)` (line 47-50), add EvalSymlinks:

Replace:
```go
absDir, err := filepath.Abs(*dir)
if err != nil {
    log.Fatalf("directorio inválido: %v", err)
}
```

With:
```go
absDir, err := filepath.Abs(*dir)
if err != nil {
    log.Fatalf("directorio inválido: %v", err)
}
absDir, err = filepath.EvalSymlinks(absDir)
if err != nil {
    log.Fatalf("directorio inválido (symlink): %v", err)
}
```

- [ ] **Step 6: Replace all 5 `strings.HasPrefix` path checks with `isWithinRoot`**

In `ServeHTTP` (line 119):
```go
// Before:
if !strings.HasPrefix(absPath, h.root) {
// After:
if !isWithinRoot(absPath, h.root) {
```

In `apiLs` (line 155):
```go
// Before:
if !strings.HasPrefix(absPath, h.root) {
// After:
if !isWithinRoot(absPath, h.root) {
```

In `apiZipInfo` (line 299):
```go
// Before:
if !strings.HasPrefix(absPath, h.root) {
// After:
if !isWithinRoot(absPath, h.root) {
```

In `apiZip` (line 342):
```go
// Before:
if !strings.HasPrefix(absPath, h.root) {
// After:
if !isWithinRoot(absPath, h.root) {
```

Also update `collectFiles` (line 259):
```go
// Before:
if err != nil || !strings.HasPrefix(resolved, h.root) {
// After:
if err != nil || !isWithinRoot(resolved, h.root) {
```

- [ ] **Step 7: Add symlink check in `ServeHTTP` before `http.ServeFile`**

After the rules check (line 135) and before the `info.IsDir()` check (line 137), add symlink resolution:

```go
// Resolver symlinks y verificar que no escapen de root.
resolved, err := resolveAndCheck(absPath, h.root)
if err != nil {
    http.NotFound(w, r)
    return
}
absPath = resolved
```

- [ ] **Step 8: Add symlink check in `apiLs` for each entry**

In the `apiLs` loop, after the dotfile check (line 170) and before the rules check, add:

```go
// Verificar symlinks no escapen de root.
entryPath := filepath.Join(absPath, e.Name())
if _, err := resolveAndCheck(entryPath, h.root); err != nil {
    continue
}
```

- [ ] **Step 9: Run `go vet` and `go build`**

Run: `cd cmd/shareiscare && go vet ./... && go build -o /dev/null .`
Expected: No errors

- [ ] **Step 10: Commit**

```bash
git add cmd/shareiscare/main.go cmd/shareiscare/main_test.go
git commit -m "security: fix path traversal via HasPrefix anti-pattern and symlink resolution

Add isWithinRoot helper that checks for separator after prefix.
Resolve root via EvalSymlinks at startup.
Add symlink checks in ServeHTTP and apiLs.
Replace all 5 unsafe HasPrefix path checks."
```

### Task 2: Dotfile check in ServeHTTP

**Files:**
- Modify: `cmd/shareiscare/main.go:102-144` (ServeHTTP method)
- Test: `cmd/shareiscare/main_test.go`

- [ ] **Step 1: Write test for `hasDotfileSegment`**

Add to `cmd/shareiscare/main_test.go`:

```go
func TestHasDotfileSegment(t *testing.T) {
	tests := []struct {
		relPath string
		expect  bool
	}{
		{"file.txt", false},
		{".env", true},
		{"subdir/.hidden", true},
		{".git/config", true},
		{"normal/path/file.go", false},
		{".", false},
	}
	for _, tt := range tests {
		got := hasDotfileSegment(tt.relPath)
		if got != tt.expect {
			t.Errorf("hasDotfileSegment(%q) = %v, want %v", tt.relPath, got, tt.expect)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cmd/shareiscare && go test -run TestHasDotfileSegment -v`
Expected: FAIL — `hasDotfileSegment` undefined

- [ ] **Step 3: Add `hasDotfileSegment` helper**

Add to `main.go` near the other helpers:

```go
// hasDotfileSegment retorna true si algún segmento del path comienza con punto.
func hasDotfileSegment(relPath string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
		if part != "." && strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd cmd/shareiscare && go test -run TestHasDotfileSegment -v`
Expected: PASS

- [ ] **Step 5: Add dotfile check in `ServeHTTP`**

In `ServeHTTP`, after computing `relPath` (line 131) and before the rules check (line 132), add:

```go
if hasDotfileSegment(relPath) {
    http.NotFound(w, r)
    return
}
```

- [ ] **Step 6: Run `go vet`**

Run: `cd cmd/shareiscare && go vet ./...`
Expected: No errors

- [ ] **Step 7: Commit**

```bash
git add cmd/shareiscare/main.go cmd/shareiscare/main_test.go
git commit -m "security: block dotfiles in ServeHTTP to match apiLs behavior

Previously dotfiles were hidden from directory listings (apiLs)
but still directly accessible by URL in ServeHTTP."
```

### Task 3: Sanitize Content-Disposition header

**Files:**
- Modify: `cmd/shareiscare/main.go:406-408` (apiZip Content-Disposition)

- [ ] **Step 1: Write test for `sanitizeFilename`**

Add to `cmd/shareiscare/main_test.go`:

```go
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"normal-folder", "normal-folder"},
		{`has"quotes`, "has_quotes"},
		{`has\backslash`, "has_backslash"},
		{"has/slash", "has_slash"},
		{"has\x00null", "has_null"},
		{"café", "café"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expect {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cmd/shareiscare && go test -run TestSanitizeFilename -v`
Expected: FAIL

- [ ] **Step 3: Implement `sanitizeFilename`**

Add to `main.go`:

```go
// sanitizeFilename limpia nombres de archivo para uso seguro en headers HTTP.
func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '/' || r < 32 {
			return '_'
		}
		return r
	}, name)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd cmd/shareiscare && go test -run TestSanitizeFilename -v`
Expected: PASS

- [ ] **Step 5: Apply sanitization in `apiZip`**

Replace line 408:
```go
// Before:
w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, folderName))
// After:
w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, sanitizeFilename(folderName)))
```

- [ ] **Step 6: Commit**

```bash
git add cmd/shareiscare/main.go cmd/shareiscare/main_test.go
git commit -m "security: sanitize Content-Disposition filename to prevent header injection"
```

---

## Chunk 2: Tunnel Hardening (Fixes 3, 2, 11)

### Task 4: Increase hash length to 16 hex chars

**Files:**
- Modify: `cmd/shareiscare/main.go:42` (hash generation)

- [ ] **Step 1: Change hash byte length from 3 to 8**

Replace line 42:
```go
// Before:
b := make([]byte, 3)
// After:
b := make([]byte, 8)
```

- [ ] **Step 2: Run `go build`**

Run: `cd cmd/shareiscare && go build -o /dev/null .`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/shareiscare/main.go
git commit -m "security: increase hash from 6 to 16 hex chars (~2^64 combinations)

Prevents brute-force enumeration of active shares.
Breaking: existing 6-char hash URLs will stop working."
```

### Task 5: Tunnel authentication with shared secret

**Files:**
- Modify: `cmd/shareiscare/main.go:453-489` (runTunnel function)
- Modify: `worker/src/worker.js:9-35` (TunnelDO connection handling)

- [ ] **Step 1: Write test for tunnel secret URL construction**

Add to `cmd/shareiscare/main_test.go`:

```go
func TestTunnelSecretURLFormat(t *testing.T) {
	secret := generateToken()
	if len(secret) != 32 {
		t.Errorf("generateToken() returned %d chars, want 32", len(secret))
	}
	url := fmt.Sprintf("wss://%s.shareiscare.dev/__tunnel_connect?secret=%s", "abc123", secret)
	if !strings.Contains(url, "?secret=") {
		t.Error("URL should contain ?secret= parameter")
	}
}
```

Run: `cd cmd/shareiscare && go test -run TestTunnelSecretURLFormat -v`
Expected: PASS (uses existing `generateToken` from admin.go)

- [ ] **Step 2: Generate tunnel secret in Go client and pass it in WebSocket URL**

In `main()`, before `go runTunnel(...)` (line 63), generate the secret:

```go
tunnelSecret := generateToken()
handler := newShareHandler(absDir, rules, *maxZip)
go runTunnel(*hash, tunnelSecret, handler)
```

Update `runTunnel` signature and WebSocket URL:

```go
func runTunnel(hash, secret string, handler http.Handler) {
	wsURL := fmt.Sprintf("wss://%s.shareiscare.dev/__tunnel_connect?secret=%s", hash, secret)
	delay := 2 * time.Second
```

- [ ] **Step 3: Update Worker `#handleClientConnect` to validate secret and enforce single client**

Replace the `#handleClientConnect` method in `worker/src/worker.js`:

```javascript
async #handleClientConnect(request) {
    const url = new URL(request.url);
    const secret = url.searchParams.get("secret");

    if (!secret) {
      return new Response("secret required", { status: 401 });
    }

    // Cargar o almacenar secreto de forma durable.
    const stored = await this.state.storage.get("tunnelSecret");
    if (stored && stored !== secret) {
      return new Response("invalid secret", { status: 403 });
    }
    if (!stored) {
      await this.state.storage.put("tunnelSecret", secret);
    }

    // Cerrar conexiones previas (permite reconexión limpia del mismo cliente).
    const existing = this.state.getWebSockets("tunnel");
    for (const ws of existing) {
      try { ws.close(1000, "replaced"); } catch (_) {}
    }

    // Limpiar pending de conexiones anteriores.
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new Error("tunnel replaced"));
    }
    this.pending.clear();

    const [client, server] = Object.values(new WebSocketPair());
    this.state.acceptWebSocket(server, ["tunnel"]);
    return new Response(null, { status: 101, webSocket: client });
  }
```

- [ ] **Step 4: Update the `fetch` method to pass `request` to `#handleClientConnect`**

In the `fetch` method of `TunnelDO`, update the call:

```javascript
if (isConnect && isUpgrade) {
    return this.#handleClientConnect(request);
}
```

- [ ] **Step 5: Run `go build` for Go side**

Run: `cd cmd/shareiscare && go build -o /dev/null .`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add cmd/shareiscare/main.go cmd/shareiscare/main_test.go worker/src/worker.js
git commit -m "security: add tunnel authentication via shared secret

Go client generates a 16-byte hex secret and sends it on WebSocket connect.
Worker validates the secret, stores it durably in the DO, and enforces
single-client by closing existing connections on reconnect."
```

### Task 6: WebSocket reconnection jitter

**Files:**
- Modify: `cmd/shareiscare/main.go` (runTunnel reconnection loop)

- [ ] **Step 1: Add `math/rand` import and jitter to the delay**

Add `math/rand` to imports (check if `math/rand/v2` is available for Go 1.25, otherwise `math/rand`).

In `runTunnel`, replace the sleep logic:

```go
// Before:
log.Printf("tunnel: connect failed: %v — retry en %v", err, delay)
time.Sleep(delay)
if delay < 60*time.Second {
    delay *= 2
}

// After:
jitter := time.Duration(rand.Int64N(int64(delay) / 2))
log.Printf("tunnel: connect failed: %v — retry en %v", err, delay+jitter)
time.Sleep(delay + jitter)
if delay < 60*time.Second {
    delay *= 2
}
```

Note: Go 1.25 uses `math/rand/v2` with `rand.Int64N`. Add import: `math/rand/v2`.

- [ ] **Step 2: Run `go build`**

Run: `cd cmd/shareiscare && go build -o /dev/null .`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add cmd/shareiscare/main.go
git commit -m "security: add jitter to WebSocket reconnection backoff"
```

---

## Chunk 3: Response Safety (Fixes 6, 7, 9, 10)

### Task 7: Response recorder size limit

**Files:**
- Modify: `cmd/shareiscare/main.go:532-543` (responseRecorder)
- Modify: `cmd/shareiscare/main.go:491-525` (handleTunnelReq)
- Test: `cmd/shareiscare/main_test.go`

- [ ] **Step 1: Write test for responseRecorder overflow**

Add to `cmd/shareiscare/main_test.go`:

```go
func TestResponseRecorderOverflow(t *testing.T) {
	rec := &responseRecorder{header: make(http.Header), code: 200}

	// Escribir justo bajo el límite debería funcionar.
	chunk := make([]byte, 1024)
	for i := 0; i < 100; i++ {
		n, err := rec.Write(chunk)
		if err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
		if n != len(chunk) {
			t.Fatalf("short write at chunk %d: %d", i, n)
		}
	}
	if rec.overflow {
		t.Error("should not overflow at 100KB")
	}

	// Forzar overflow escribiendo un bloque enorme.
	huge := make([]byte, maxResponseBody+1)
	rec2 := &responseRecorder{header: make(http.Header), code: 200}
	rec2.Write(huge)
	if !rec2.overflow {
		t.Error("should overflow with body > maxResponseBody")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd cmd/shareiscare && go test -run TestResponseRecorderOverflow -v`
Expected: FAIL — compilation errors (`overflow` field and `maxResponseBody` constant undefined)

- [ ] **Step 3: Update `responseRecorder` with overflow protection**

Replace the `responseRecorder` struct and `Write` method:

```go
const maxResponseBody = 256 << 20 // 256 MB

type responseRecorder struct {
	header   http.Header
	body     []byte
	code     int
	overflow bool
}

func (r *responseRecorder) Header() http.Header  { return r.header }
func (r *responseRecorder) WriteHeader(code int)  { r.code = code }
func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.overflow {
		return len(b), nil
	}
	if int64(len(r.body))+int64(len(b)) > maxResponseBody {
		r.overflow = true
		return len(b), nil
	}
	r.body = append(r.body, b...)
	return len(b), nil
}
```

- [ ] **Step 4: Check overflow in `handleTunnelReq`**

In `handleTunnelReq`, after `handler.ServeHTTP(rec, httpReq)`, add:

```go
handler.ServeHTTP(rec, httpReq)

if rec.overflow {
    sendTunnelErr(cw, req.ID, 502)
    return
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd cmd/shareiscare && go test -run TestResponseRecorderOverflow -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/shareiscare/main.go cmd/shareiscare/main_test.go
git commit -m "security: add 256MB size limit to responseRecorder

Prevents OOM from large file responses tunneled over WebSocket.
Silently discards writes past the limit and returns 502."
```

### Task 8: Security headers

**Files:**
- Modify: `cmd/shareiscare/main.go:102-144` (ServeHTTP method)

- [ ] **Step 1: Add a `setSecurityHeaders` helper**

Add to `main.go`:

```go
// setSecurityHeaders agrega headers de seguridad a todas las respuestas públicas.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
}
```

- [ ] **Step 2: Call it at the top of `ServeHTTP`**

At the very beginning of `ServeHTTP`, before any return path:

```go
func (h *shareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	if r.URL.Path == "/__api/zip-info" {
```

- [ ] **Step 3: Add CSP header for directory (index.html) responses**

In the directory branch of `ServeHTTP` (where `indexHTML` is served), add CSP before writing:

```go
if info.IsDir() {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'unsafe-inline'; img-src data:; connect-src 'self'")
    w.Write(indexHTML)
    return
}
```

- [ ] **Step 4: Run `go build`**

Run: `cd cmd/shareiscare && go build -o /dev/null .`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add cmd/shareiscare/main.go
git commit -m "security: add X-Content-Type-Options, X-Frame-Options, Referrer-Policy, and CSP headers"
```

### Task 9: ZIP concurrency limit

**Files:**
- Modify: `cmd/shareiscare/main.go:84-92` (shareHandler struct + constructor)
- Modify: `cmd/shareiscare/main.go:333-410` (apiZip method)

- [ ] **Step 1: Add `zipSem` field to `shareHandler`**

Update the struct:

```go
type shareHandler struct {
	root   string
	rules  *RulesEngine
	maxZip int64
	zipSem chan struct{}
}
```

Update the constructor:

```go
func newShareHandler(root string, rules *RulesEngine, maxZip int64) *shareHandler {
	return &shareHandler{root: root, rules: rules, maxZip: maxZip, zipSem: make(chan struct{}, 3)}
}
```

- [ ] **Step 2: Add semaphore acquire at the top of `apiZip`**

At the start of `apiZip`, before any work:

```go
func (h *shareHandler) apiZip(w http.ResponseWriter, r *http.Request) {
	// Limitar descargas ZIP concurrentes para proteger memoria.
	select {
	case h.zipSem <- struct{}{}:
		defer func() { <-h.zipSem }()
	default:
		http.Error(w, "too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}

	reqPath := r.URL.Query().Get("path")
```

- [ ] **Step 3: Run `go build`**

Run: `cd cmd/shareiscare && go build -o /dev/null .`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add cmd/shareiscare/main.go
git commit -m "security: limit concurrent ZIP builds to 3 to prevent memory exhaustion"
```

### Task 10: Final verification

- [ ] **Step 1: Run all tests**

Run: `cd cmd/shareiscare && go test -v ./...`
Expected: All tests PASS

- [ ] **Step 2: Run vet**

Run: `cd cmd/shareiscare && go vet ./...`
Expected: No issues

- [ ] **Step 3: Build binary**

Run: `make build`
Expected: Binary compiles successfully

- [ ] **Step 4: Commit any remaining fixes**

If any issues were found, fix and commit.
