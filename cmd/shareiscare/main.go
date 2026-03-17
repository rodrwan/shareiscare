package main

import (
	"archive/zip"
	"bytes"
	crypto_rand "crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// version is injected at build-time via ldflags.
var version = "dev"

//go:embed index.html
var indexHTML []byte

// isWithinRoot checks that path is within root (avoids the HasPrefix anti-pattern without separator).
func isWithinRoot(path, root string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// sanitizeFilename cleans filenames for safe use in HTTP headers.
func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '/' || r < 32 {
			return '_'
		}
		return r
	}, name)
}

// hasDotfileSegment returns true if any path segment starts with a dot.
func hasDotfileSegment(relPath string) bool {
	for _, part := range strings.Split(filepath.ToSlash(relPath), "/") {
		if part != "." && strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// resolveAndCheck resolves symlinks and verifies the resulting path is within root.
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

// resolveIdentity determines which hash and secret to use based on flags and persisted values.
func resolveIdentity(flagHash string, forceNew bool, persistedHash, persistedSecret string) (hash, secret string, changed bool) {
	// --new-hash: always generate both.
	if forceNew {
		return generateHash(), generateToken(), true
	}

	// --hash provided.
	if flagHash != "" {
		if flagHash == persistedHash && persistedSecret != "" {
			// Same hash as persisted, reuse secret.
			return flagHash, persistedSecret, false
		}
		// Different hash or no persisted secret: generate new secret.
		return flagHash, generateToken(), true
	}

	// No flags: use persisted values if they exist.
	if persistedHash != "" && persistedSecret != "" {
		return persistedHash, persistedSecret, false
	}

	// Nothing persisted: generate both.
	return generateHash(), generateToken(), true
}

// generateHash generates a random 16-character hex hash.
func generateHash() string {
	b := make([]byte, 8)
	crypto_rand.Read(b)
	return hex.EncodeToString(b)
}

// validateHash checks that the hash is valid (4-64 chars, only [a-z0-9]).
func validateHash(hash string) error {
	if len(hash) < 4 || len(hash) > 64 {
		return fmt.Errorf("hash must be 4-64 characters, got %d", len(hash))
	}
	for _, r := range hash {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("hash must contain only lowercase letters and digits, got %q", string(r))
		}
	}
	return nil
}

// bandwidthTracker tracks daily bandwidth consumption.
type bandwidthTracker struct {
	mu    sync.Mutex
	date  string // "2006-01-02"
	used  int64
	limit int64
}

// Reserve attempts to reserve n bytes. Returns false if the daily limit is exceeded.
func (bt *bandwidthTracker) Reserve(n int64) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	today := time.Now().Format("2006-01-02")
	if bt.date != today {
		bt.date = today
		bt.used = 0
	}
	if bt.used+n > bt.limit {
		return false
	}
	bt.used += n
	return true
}

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	hash := flag.String("hash", "", "subdomain hash (auto-generated if omitted)")
	dir := flag.String("dir", ".", "directory to share")
	adminPort := flag.String("admin-port", "9898", "admin panel port")
	configPath := flag.String("config", "", "path to .shareiscare.json (default: <dir>/.shareiscare.json)")
	noAdmin := flag.Bool("no-admin", false, "disable admin panel")
	noDefaults := flag.Bool("no-defaults", false, "don't seed default sensitive patterns")
	maxZip := flag.Int64("max-zip", 100*1024*1024, "max total size for ZIP downloads in bytes")
	newHash := flag.Bool("new-hash", false, "force a new hash and URL, ignoring persisted values")
	password := flag.String("password", "", "require password for public access (HTTP Basic Auth)")
	maxBW := flag.Int64("max-bandwidth", 0, "max daily bandwidth in MB (0 = unlimited)")
	flag.Parse()

	if *showVersion {
		fmt.Println("shareiscare", version)
		os.Exit(0)
	}

	absDir, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("invalid directory: %v", err)
	}
	absDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		log.Fatalf("invalid directory (symlink): %v", err)
	}

	// Rules engine.
	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(absDir, ".shareiscare.json")
	}
	rules, err := NewRulesEngine(cfgPath, !*noDefaults)
	if err != nil {
		log.Fatalf("rules engine: %v", err)
	}

	// Resolve tunnel identity (hash + secret).
	persistedHash, persistedSecret := rules.TunnelIdentity()
	resolvedHash, tunnelSecret, changed := resolveIdentity(strings.ToLower(*hash), *newHash, persistedHash, persistedSecret)
	if err := validateHash(resolvedHash); err != nil {
		log.Fatalf("invalid hash: %v", err)
	}
	if changed {
		if err := rules.SetTunnelIdentity(resolvedHash, tunnelSecret); err != nil {
			log.Fatalf("error persisting identity: %v", err)
		}
	}

	// Bandwidth tracker (optional).
	var bw *bandwidthTracker
	if *maxBW > 0 {
		bw = &bandwidthTracker{limit: *maxBW * 1024 * 1024}
	}

	handler := newShareHandler(absDir, rules, *maxZip, *password, bw)
	go runTunnel(resolvedHash, tunnelSecret, handler)

	log.Printf("📁 Sharing: %s", absDir)
	log.Printf("🌍 Public:  https://%s.shareiscare.dev", resolvedHash)
	if *password != "" {
		log.Printf("🔒 Password protection enabled")
	}
	if *maxBW > 0 {
		log.Printf("📊 Daily bandwidth limit: %d MB", *maxBW)
	}

	// Admin server.
	if !*noAdmin {
		token := generateToken()
		admin := newAdminHandler(token, rules, absDir)
		adminAddr := net.JoinHostPort("127.0.0.1", *adminPort)
		log.Printf("🔐 Admin:   http://%s?token=%s", adminAddr, token)
		go func() {
			if err := http.ListenAndServe(adminAddr, admin); err != nil {
				log.Fatalf("admin server: %v", err)
			}
		}()
	}

	select {}
}

type shareHandler struct {
	root      string
	rules     *RulesEngine
	maxZip    int64
	zipSem    chan struct{}
	password  string
	bandwidth *bandwidthTracker
}

func newShareHandler(root string, rules *RulesEngine, maxZip int64, password string, bw *bandwidthTracker) *shareHandler {
	return &shareHandler{
		root: root, rules: rules, maxZip: maxZip,
		zipSem: make(chan struct{}, 3), password: password, bandwidth: bw,
	}
}

type fileEntry struct {
	Name    string `json:"name"`
	IsDir   bool   `json:"isDir"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	Ext     string `json:"ext"`
}

// setSecurityHeaders adds security headers to all public responses.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (h *shareHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	// Password authentication (HTTP Basic Auth).
	if h.password != "" {
		_, pass, ok := r.BasicAuth()
		if !ok || pass != h.password {
			w.Header().Set("WWW-Authenticate", `Basic realm="shareiscare"`)
			http.Error(w, "unauthorized", 401)
			return
		}
	}

	if r.URL.Path == "/__api/zip-info" {
		h.apiZipInfo(w, r)
		return
	}
	if r.URL.Path == "/__api/zip" {
		h.apiZip(w, r)
		return
	}
	if r.URL.Path == "/__api/ls" {
		h.apiLs(w, r)
		return
	}

	cleanPath := filepath.Clean(r.URL.Path)
	absPath   := filepath.Join(h.root, cleanPath)

	if !isWithinRoot(absPath, h.root) {
		http.Error(w, "forbidden", 403)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Resolve symlinks and verify they don't escape root.
	resolved, err := resolveAndCheck(absPath, h.root)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	absPath = resolved

	// Check rules: block hidden files/dirs (return 404 to not leak existence).
	relPath, _ := filepath.Rel(h.root, absPath)
	if hasDotfileSegment(relPath) {
		http.NotFound(w, r)
		return
	}
	if h.rules.IsPathOrAncestorHidden(relPath, info.IsDir()) {
		http.NotFound(w, r)
		return
	}

	if info.IsDir() {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline' https://fonts.googleapis.com; font-src https://fonts.gstatic.com; script-src 'unsafe-inline'; img-src data:; connect-src 'self'")
		w.Write(indexHTML)
		return
	}

	// Daily bandwidth control.
	if h.bandwidth != nil && !h.bandwidth.Reserve(info.Size()) {
		http.Error(w, "bandwidth limit exceeded", http.StatusTooManyRequests)
		return
	}

	http.ServeFile(w, r, absPath)
}

func (h *shareHandler) apiLs(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}

	cleanPath := filepath.Clean(reqPath)
	absPath   := filepath.Join(h.root, cleanPath)

	if !isWithinRoot(absPath, h.root) {
		http.Error(w, "forbidden", 403)
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		http.Error(w, "cannot read dir", 500)
		return
	}

	var files []fileEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		// Verify symlinks don't escape root.
		entryPath := filepath.Join(absPath, e.Name())
		if _, err := resolveAndCheck(entryPath, h.root); err != nil {
			continue
		}
		// Check rules engine.
		relPath, _ := filepath.Rel(h.root, filepath.Join(absPath, e.Name()))
		if h.rules.IsHidden(relPath, e.IsDir()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		ext := ""
		if !e.IsDir() {
			ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(e.Name())), ".")
		}
		files = append(files, fileEntry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    sizeOf(info),
			ModTime: info.ModTime().Format("Jan 02, 2006"),
			Ext:     ext,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"path":    cleanPath,
		"entries": files,
	})
}

func sizeOf(info fs.FileInfo) int64 {
	if info.IsDir() {
		return -1
	}
	return info.Size()
}

// walkEntry represents a file collected for ZIP.
type walkEntry struct {
	absPath string
	relPath string
	size    int64
}

// collectFiles recursively walks a directory, respecting visibility rules.
func (h *shareHandler) collectFiles(absDir string) ([]walkEntry, int64, error) {
	var entries []walkEntry
	var totalSize int64

	err := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}

		name := info.Name()

		// Skip dotfiles.
		if strings.HasPrefix(name, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(h.root, path)

		// Check visibility rules.
		if h.rules.IsHidden(relPath, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Resolve file symlinks and verify they don't escape root.
		// Note: filepath.Walk does not follow directory symlinks (it skips them).
		if info.Mode()&os.ModeSymlink != 0 {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				return nil
			}
			resolved, err = filepath.Abs(resolved)
			if err != nil || !isWithinRoot(resolved, h.root) {
				return nil
			}
			info, err = os.Stat(resolved)
			if err != nil || info.IsDir() {
				return nil
			}
		}

		if info.IsDir() {
			return nil
		}

		// Relative path inside ZIP: use directory name as root.
		zipRel, _ := filepath.Rel(absDir, path)
		folderName := filepath.Base(absDir)
		zipPath := filepath.Join(folderName, zipRel)

		entries = append(entries, walkEntry{
			absPath: path,
			relPath: zipPath,
			size:    info.Size(),
		})
		totalSize += info.Size()

		return nil
	})

	return entries, totalSize, err
}

func (h *shareHandler) apiZipInfo(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}

	cleanPath := filepath.Clean(reqPath)
	absPath := filepath.Join(h.root, cleanPath)

	if !isWithinRoot(absPath, h.root) {
		http.Error(w, "forbidden", 403)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		http.Error(w, "not a directory", 400)
		return
	}

	// Verify the directory is not hidden.
	relPath, _ := filepath.Rel(h.root, absPath)
	if h.rules.IsPathOrAncestorHidden(relPath, true) {
		http.NotFound(w, r)
		return
	}

	files, totalSize, err := h.collectFiles(absPath)
	if err != nil {
		http.Error(w, "error scanning directory", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"folder":    filepath.Base(absPath),
		"fileCount": len(files),
		"totalSize": totalSize,
		"maxSize":   h.maxZip,
		"ok":        len(files) > 0 && totalSize <= h.maxZip,
	})
}

func (h *shareHandler) apiZip(w http.ResponseWriter, r *http.Request) {
	// Limit concurrent ZIP downloads to protect memory.
	select {
	case h.zipSem <- struct{}{}:
		defer func() { <-h.zipSem }()
	default:
		http.Error(w, "too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}

	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}

	cleanPath := filepath.Clean(reqPath)
	absPath := filepath.Join(h.root, cleanPath)

	if !isWithinRoot(absPath, h.root) {
		http.Error(w, "forbidden", 403)
		return
	}

	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		http.Error(w, "not a directory", 400)
		return
	}

	relPath, _ := filepath.Rel(h.root, absPath)
	if h.rules.IsPathOrAncestorHidden(relPath, true) {
		http.NotFound(w, r)
		return
	}

	files, totalSize, err := h.collectFiles(absPath)
	if err != nil {
		http.Error(w, "error scanning directory", 500)
		return
	}

	if len(files) == 0 {
		http.Error(w, "empty directory", 400)
		return
	}

	if totalSize > h.maxZip {
		http.Error(w, fmt.Sprintf("total size %d exceeds max %d", totalSize, h.maxZip), 413)
		return
	}

	// Daily bandwidth control for ZIP.
	if h.bandwidth != nil && !h.bandwidth.Reserve(totalSize) {
		http.Error(w, "bandwidth limit exceeded", http.StatusTooManyRequests)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for _, f := range files {
		fh := &zip.FileHeader{
			Name:     filepath.ToSlash(f.relPath),
			Method:   zip.Deflate,
			Modified: time.Now(),
		}

		// Try to preserve actual mod time.
		if fi, err := os.Stat(f.absPath); err == nil {
			fh.Modified = fi.ModTime()
		}

		writer, err := zw.CreateHeader(fh)
		if err != nil {
			http.Error(w, "zip creation error", 500)
			return
		}

		if err := copyFileToZip(writer, f.absPath, f.size); err != nil {
			continue
		}
	}

	if err := zw.Close(); err != nil {
		http.Error(w, "zip finalization error", 500)
		return
	}

	folderName := filepath.Base(absPath)
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, sanitizeFilename(folderName)))
	w.Write(buf.Bytes())
}

// copyFileToZip copies a file to the ZIP writer, limiting to the expected size.
func copyFileToZip(w io.Writer, path string, maxBytes int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, io.LimitReader(f, maxBytes))
	return err
}

// Tunnel

type tunnelReq struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type tunnelResp struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// connWriter serializes all writes to a websocket connection.
type connWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (cw *connWriter) write(data []byte) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.conn.WriteMessage(websocket.TextMessage, data)
}

func runTunnel(hash, secret string, handler http.Handler) {
	wsURL := fmt.Sprintf("wss://%s.shareiscare.dev/__tunnel_connect?secret=%s", hash, secret)
	delay := 2 * time.Second

	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			jitter := time.Duration(rand.Int64N(int64(delay) / 2))
			log.Printf("tunnel: connect failed: %v — retrying in %v", err, delay+jitter)
			time.Sleep(delay + jitter)
			if delay < 60*time.Second {
				delay *= 2
			}
			continue
		}

		delay = 2 * time.Second
		log.Printf("✅ Tunnel connected")

		cw := &connWriter{conn: conn}

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("tunnel: disconnected: %v", err)
				break
			}

			var req tunnelReq
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			go handleTunnelReq(cw, req, handler)
		}

		conn.Close()
	}
}

func handleTunnelReq(cw *connWriter, req tunnelReq, handler http.Handler) {
	var bodyReader io.Reader = http.NoBody
	if req.Body != "" {
		decoded, _ := base64.StdEncoding.DecodeString(req.Body)
		bodyReader = strings.NewReader(string(decoded))
	}

	httpReq, err := http.NewRequest(req.Method, req.Path, bodyReader)
	if err != nil {
		sendTunnelErr(cw, req.ID, 500)
		return
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	rec := &responseRecorder{header: make(http.Header), code: 200}
	handler.ServeHTTP(rec, httpReq)

	if rec.overflow {
		sendTunnelErr(cw, req.ID, 502)
		return
	}

	headers := map[string]string{}
	for k, v := range rec.header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	out, _ := json.Marshal(tunnelResp{
		ID:      req.ID,
		Type:    "response",
		Status:  rec.code,
		Headers: headers,
		Body:    base64.StdEncoding.EncodeToString(rec.body),
	})
	cw.write(out)
}

func sendTunnelErr(cw *connWriter, id string, status int) {
	out, _ := json.Marshal(tunnelResp{ID: id, Type: "response", Status: status})
	cw.write(out)
}

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
