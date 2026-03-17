package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestResponseRecorderOverflow(t *testing.T) {
	rec := &responseRecorder{header: make(http.Header), code: 200}

	// Writing just under the limit should work.
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

	// Force overflow by writing a huge block.
	huge := make([]byte, maxResponseBody+1)
	rec2 := &responseRecorder{header: make(http.Header), code: 200}
	rec2.Write(huge)
	if !rec2.overflow {
		t.Error("should overflow with body > maxResponseBody")
	}
}

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

func TestResolveIdentity(t *testing.T) {
	tests := []struct {
		name            string
		flagHash        string
		forceNew        bool
		persistedHash   string
		persistedSecret string
		wantHash        string // empty = verify it's not empty
		wantSecret      string
		wantChanged     bool
	}{
		{
			name:        "no flags and no persisted generates both",
			wantChanged: true,
		},
		{
			name:            "reuses persisted values",
			persistedHash:   "abc123",
			persistedSecret: "secret456",
			wantHash:        "abc123",
			wantSecret:      "secret456",
			wantChanged:     false,
		},
		{
			name:            "flag hash equal to persisted reuses secret",
			flagHash:        "abc123",
			persistedHash:   "abc123",
			persistedSecret: "secret456",
			wantHash:        "abc123",
			wantSecret:      "secret456",
			wantChanged:     false,
		},
		{
			name:            "different flag hash generates new secret",
			flagHash:        "newhash",
			persistedHash:   "abc123",
			persistedSecret: "secret456",
			wantHash:        "newhash",
			wantChanged:     true,
		},
		{
			name:            "force new ignores persisted",
			forceNew:        true,
			persistedHash:   "abc123",
			persistedSecret: "secret456",
			wantChanged:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, secret, changed := resolveIdentity(tt.flagHash, tt.forceNew, tt.persistedHash, tt.persistedSecret)

			if hash == "" {
				t.Error("hash should not be empty")
			}
			if secret == "" {
				t.Error("secret should not be empty")
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if tt.wantHash != "" && hash != tt.wantHash {
				t.Errorf("hash = %q, want %q", hash, tt.wantHash)
			}
			if tt.wantSecret != "" && secret != tt.wantSecret {
				t.Errorf("secret = %q, want %q", secret, tt.wantSecret)
			}
			// force new and different flag hash must generate a secret different from persisted.
			if tt.wantChanged && tt.persistedSecret != "" && tt.wantSecret == "" && secret == tt.persistedSecret {
				t.Error("expected a new secret, got the persisted one")
			}
		})
	}
}

func TestTunnelIdentityPersistence(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".shareiscare.json")

	// Create engine, set identity.
	re, err := NewRulesEngine(cfgPath, false)
	if err != nil {
		t.Fatalf("NewRulesEngine: %v", err)
	}
	if err := re.SetTunnelIdentity("myhash", "mysecret"); err != nil {
		t.Fatalf("SetTunnelIdentity: %v", err)
	}

	// Recreate from the same file.
	re2, err := NewRulesEngine(cfgPath, false)
	if err != nil {
		t.Fatalf("NewRulesEngine (reload): %v", err)
	}
	hash, secret := re2.TunnelIdentity()
	if hash != "myhash" || secret != "mysecret" {
		t.Errorf("got (%q, %q), want (\"myhash\", \"mysecret\")", hash, secret)
	}
}

func TestTunnelIdentityBackwardCompat(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".shareiscare.json")

	// Write JSON without hash/tunnelSecret fields (old format).
	old := map[string]any{
		"version":                1,
		"defaultPatternsEnabled": true,
		"rules":                  []any{},
	}
	data, _ := json.MarshalIndent(old, "", "  ")
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	re, err := NewRulesEngine(cfgPath, true)
	if err != nil {
		t.Fatalf("NewRulesEngine: %v", err)
	}
	hash, secret := re.TunnelIdentity()
	if hash != "" || secret != "" {
		t.Errorf("expected empty identity from old config, got (%q, %q)", hash, secret)
	}
}

func TestValidateHash(t *testing.T) {
	tests := []struct {
		hash    string
		wantErr bool
	}{
		{"abcd1234", false},
		{"abcdefghijklmnop", false},
		{"abc", true},                      // too short
		{"ABCD1234", true},                 // uppercase
		{"abcd-1234", true},                // hyphen
		{"abcd_1234", true},                // underscore
		{strings.Repeat("a", 65), true},    // too long
		{strings.Repeat("a", 64), false},   // just at the limit
		{"1234", false},                    // mínimo 4 chars
	}
	for _, tt := range tests {
		err := validateHash(tt.hash)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateHash(%q) error = %v, wantErr %v", tt.hash, err, tt.wantErr)
		}
	}
}

func TestBandwidthTracker(t *testing.T) {
	bt := &bandwidthTracker{limit: 100}

	if !bt.Reserve(50) {
		t.Error("should allow 50 bytes")
	}
	if !bt.Reserve(50) {
		t.Error("should allow another 50 bytes (total=100)")
	}
	if bt.Reserve(1) {
		t.Error("should reject when at limit")
	}
}

func testDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dir, _ = filepath.EvalSymlinks(dir)
	return dir
}

func TestBandwidthTrackerUnlimited(t *testing.T) {
	// When bandwidth is nil, nothing is blocked (verify the handler doesn't crash).
	dir := testDir(t)
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	cfgPath := filepath.Join(dir, ".shareiscare.json")
	rules, _ := NewRulesEngine(cfgPath, false)

	h := newShareHandler(dir, rules, 100*1024*1024, "", nil)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200 without bandwidth limit, got %d", rec.Code)
	}
}

func TestPasswordAuth(t *testing.T) {
	dir := testDir(t)
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	cfgPath := filepath.Join(dir, ".shareiscare.json")
	rules, _ := NewRulesEngine(cfgPath, false)

	h := newShareHandler(dir, rules, 100*1024*1024, "secret123", nil)

	// No auth → 401.
	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("expected 401 without auth, got %d", rec.Code)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header")
	}

	// Wrong password → 401.
	req = httptest.NewRequest("GET", "/test.txt", nil)
	req.SetBasicAuth("user", "wrongpass")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("expected 401 with wrong password, got %d", rec.Code)
	}

	// Correct password → 200.
	req = httptest.NewRequest("GET", "/test.txt", nil)
	req.SetBasicAuth("user", "secret123")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200 with correct password, got %d", rec.Code)
	}
}

func TestPasswordAuthDisabled(t *testing.T) {
	dir := testDir(t)
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	cfgPath := filepath.Join(dir, ".shareiscare.json")
	rules, _ := NewRulesEngine(cfgPath, false)

	// No password → open access.
	h := newShareHandler(dir, rules, 100*1024*1024, "", nil)

	req := httptest.NewRequest("GET", "/test.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Errorf("expected 200 without password, got %d", rec.Code)
	}
}
