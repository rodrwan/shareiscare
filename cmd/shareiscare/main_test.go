package main

import (
	"fmt"
	"net/http"
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
