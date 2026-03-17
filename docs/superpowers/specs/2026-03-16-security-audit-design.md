# Security Audit — File Protection & WebSocket Tunnel

**Date:** 2026-03-16
**Scope:** Protect shared files from unauthorized access; harden the WebSocket tunnel against enumeration and hijacking.
**Out of scope:** Admin panel hardening (runs on localhost only).

## Findings Summary

| # | Severity | Finding | Location |
|---|----------|---------|----------|
| 1 | Critical | Symlink traversal in ServeHTTP/apiLs | main.go:143, main.go:160 |
| 2 | Critical | Tunnel WebSocket has no authentication + single client enforcement | worker.js:21, main.go:454, worker.js:49 |
| 3 | Critical | 6-char hash is brutable (~16M combinations) | main.go:42-44 |
| 4 | Critical | `strings.HasPrefix` path check anti-pattern (prefix without separator) | main.go:119, 155, 299, 345 |
| 5 | High | Root path not resolved via EvalSymlinks at startup | main.go:47 |
| 6 | High | responseRecorder has no size limit | main.go:540-543 |
| 7 | High | Missing security headers | main.go:138-140, main.go:143 |
| 8 | High | Dotfile discoverability gap: apiLs hides them but ServeHTTP does not | main.go:168 vs main.go:102 |
| 9 | Medium | Content-Disposition unsanitized | main.go:408 |
| 10 | Medium | ZIP built entirely in memory, no concurrency limit | main.go:375 |
| 11 | Medium | WebSocket reconnection without jitter | main.go:462-464 |

## Approach: Surgical Fixes

Fix each finding directly in the existing files without architectural changes.

---

## Fix Designs

### Fix 1: Symlink traversal protection + root resolution

**Problem:** `http.ServeFile` and `os.ReadDir` follow symlinks. A symlink inside the shared directory pointing outside `h.root` serves arbitrary files. Additionally, `h.root` itself is only resolved via `filepath.Abs` but not `filepath.EvalSymlinks`, which can cause false negatives in prefix checks.

**Fix:**
1. At startup (`main.go:47`), resolve root via `filepath.EvalSymlinks` after `filepath.Abs`:
```go
absDir, err := filepath.Abs(*dir)
// ... then also:
absDir, err = filepath.EvalSymlinks(absDir)
```

2. Add a `resolveAndCheck` helper that uses the safe prefix check (see Fix 4). Apply to:
- `ServeHTTP` — before calling `http.ServeFile`
- `apiLs` — when iterating directory entries, resolve symlinks and skip entries pointing outside root

**Reference:** `collectFiles` already does this correctly (main.go:252-266). Replicate the same logic.

```go
// resolveAndCheck verifies path stays within root after symlink resolution.
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

### Fix 2: Tunnel auth + single client enforcement (merged Fixes 2, 4, 5)

**Problem:** Anyone who knows the hash can connect as tunnel client. Multiple concurrent clients cause undefined behavior. No origin validation.

**Fix:** Single coherent connection-management mechanism:

- **Go client** (`main.go`): Generate a `tunnelSecret` (16 bytes hex) at startup. Connect to `wss://<hash>.shareiscare.dev/__tunnel_connect?secret=<tunnelSecret>`.
- **Worker** (`worker.js`): On `/__tunnel_connect`:
  1. Extract `secret` from URL query parameter.
  2. If no secret provided, reject with 401.
  3. Load stored secret from `state.storage` (durable, survives hibernation).
  4. If no stored secret exists (first connection ever), store the provided secret via `state.storage.put("tunnelSecret", secret)`.
  5. If stored secret exists and does not match, reject with 403.
  6. If stored secret matches (or was just stored): close all existing tunnel-tagged sockets (`state.getWebSockets("tunnel").forEach(ws => ws.close())`), then accept the new connection. This handles reconnections cleanly.
- **Secret lifecycle:** The secret is stored durably in the DO. When the Go client restarts with a new secret, it gets a new hash too (since hash is auto-generated), so the DO is different. If the user specifies `--hash` explicitly, they must restart to get a new secret accepted; the previous DO's stored secret will reject the new one. This is acceptable — the user would need to wait for DO eviction (~30s idle) or use a new hash.

### Fix 3: Increase hash length

**Problem:** 3 bytes = ~16M combinations, brutable in minutes.

**Fix:** Increase to 8 bytes (16 hex chars). This gives ~2^64 combinations, making enumeration infeasible.

```go
b := make([]byte, 8)  // was 3
```

### Fix 4: Safe path prefix check

**Problem:** `strings.HasPrefix(absPath, h.root)` is an anti-pattern. If `h.root` is `/home/user/share` and `absPath` resolves to `/home/user/shareevil`, the check passes incorrectly. This pattern appears in 4 locations: `main.go:119, 155, 299, 345`.

**Fix:** Replace all 4 instances with a safe helper:

```go
func isWithinRoot(path, root string) bool {
    return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}
```

### Fix 5: Dotfile check in ServeHTTP

**Problem:** `apiLs` (line 168) explicitly skips files starting with `.`, but `ServeHTTP` does not. If default patterns are disabled via `--no-defaults`, dotfiles like `.bashrc` or `.profile` are directly accessible by URL even though they don't appear in listings.

**Fix:** Add a dotfile check in `ServeHTTP` after the path validation, checking every path segment:

```go
// Check for dotfiles in any path segment.
for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
    if strings.HasPrefix(part, ".") {
        http.NotFound(w, r)
        return
    }
}
```

### Fix 6: Response recorder size limit

**Problem:** `responseRecorder.body` grows without limit.

**Fix:** Add a max body size constant (e.g., 256MB). Use an `overflow` flag since `http.ServeFile` does not check write errors. After `handler.ServeHTTP(rec, httpReq)` returns, check the flag and send 502 if set.

```go
const maxResponseBody = 256 << 20 // 256 MB

type responseRecorder struct {
    header   http.Header
    body     []byte
    code     int
    overflow bool
}

func (r *responseRecorder) Write(b []byte) (int, error) {
    if r.overflow {
        return len(b), nil // silently discard
    }
    if int64(len(r.body))+int64(len(b)) > maxResponseBody {
        r.overflow = true
        return len(b), nil
    }
    r.body = append(r.body, b...)
    return len(b), nil
}
```

In `handleTunnelReq`, check `rec.overflow` after ServeHTTP and send 502 if true.

### Fix 7: Security headers

**Problem:** No security headers on served content.

**Fix:** Add headers to all `shareHandler.ServeHTTP` responses:
- `X-Content-Type-Options: nosniff` — on all responses
- `X-Frame-Options: DENY` — on all responses
- `Referrer-Policy: no-referrer` — on all responses
- `Content-Security-Policy` — only on index.html directory responses: `default-src 'none'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'unsafe-inline'; img-src data:; connect-src 'self'`

### Fix 8: Sanitize Content-Disposition

**Problem:** `folderName` used directly in header, allowing header injection.

**Fix:** Sanitize the folder name by removing/replacing special characters, or use RFC 6266 encoding:

```go
safeName := strings.Map(func(r rune) rune {
    if r == '"' || r == '\\' || r == '/' || r < 32 {
        return '_'
    }
    return r
}, folderName)
w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, safeName))
```

### Fix 9: ZIP concurrency limit

**Problem:** Multiple concurrent ZIP requests can exhaust memory.

**Fix:** Use a semaphore (buffered channel) to limit concurrent ZIP builds.

```go
type shareHandler struct {
    root    string
    rules   *RulesEngine
    maxZip  int64
    zipSem  chan struct{}
}

// In newShareHandler:
zipSem: make(chan struct{}, 3), // max 3 concurrent ZIPs

// In apiZip:
select {
case h.zipSem <- struct{}{}:
    defer func() { <-h.zipSem }()
default:
    http.Error(w, "too many concurrent downloads", http.StatusServiceUnavailable)
    return
}
```

### Fix 11: Reconnection jitter

**Problem:** Exponential backoff without jitter causes thundering herd.

**Fix:** Add random jitter to the delay.

```go
jitter := time.Duration(rand.Int63n(int64(delay) / 2))
time.Sleep(delay + jitter)
```

---

## Implementation Order

1. **Fix 4** (safe prefix check) — foundational, all other path fixes depend on it
2. **Fix 1** (symlink + root resolution) — uses Fix 4's `isWithinRoot`
3. **Fix 5** (dotfile check in ServeHTTP)
4. **Fix 3** (longer hash) — deploy-independent, Go-only change
5. **Fix 2** (tunnel auth + single client) — requires coordinated Go + Worker change
6. **Fix 6** (response recorder limit)
7. **Fix 7** (security headers)
8. **Fix 8** (Content-Disposition sanitization)
9. **Fix 9** (ZIP concurrency limit)
10. **Fix 10** (ZIP concurrency limit)
11. **Fix 11** (reconnection jitter)

## Breaking Changes

- **Hash length change (Fix 3):** Existing bookmarks/shared URLs with 6-char hashes will stop working. Accepted tradeoff — security over URL brevity. No migration period needed since shares are ephemeral by nature.
