package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "embed"
)

//go:embed admin.html
var adminHTML []byte

// generateToken creates a random 16-byte hex token.
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type adminHandler struct {
	token string
	rules *RulesEngine
	root  string // shared directory root
}

func newAdminHandler(token string, rules *RulesEngine, root string) *adminHandler {
	return &adminHandler{token: token, rules: rules, root: root}
}

func (a *adminHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !a.authenticate(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	switch {
	case r.URL.Path == "/" || r.URL.Path == "":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(adminHTML)

	case r.URL.Path == "/__admin/api/rules" && r.Method == http.MethodGet:
		a.handleListRules(w, r)

	case r.URL.Path == "/__admin/api/rules" && r.Method == http.MethodPost:
		a.handleAddRule(w, r)

	case r.URL.Path == "/__admin/api/rules" && r.Method == http.MethodDelete:
		a.handleRemoveRule(w, r)

	case r.URL.Path == "/__admin/api/defaults" && r.Method == http.MethodPut:
		a.handleToggleDefaults(w, r)

	case r.URL.Path == "/__admin/api/tree" && r.Method == http.MethodGet:
		a.handleTree(w, r)

	default:
		http.NotFound(w, r)
	}
}

func (a *adminHandler) authenticate(r *http.Request) bool {
	// Check query param.
	if t := r.URL.Query().Get("token"); t == a.token {
		return true
	}
	// Check Authorization header.
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == a.token {
		return true
	}
	return false
}

func (a *adminHandler) handleListRules(w http.ResponseWriter, _ *http.Request) {
	rules := a.rules.ListRules()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"defaultsEnabled": a.rules.DefaultsEnabled(),
		"rules":           rules,
	})
}

func (a *adminHandler) handleAddRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern string `json:"pattern"`
		Action  string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if body.Pattern == "" {
		http.Error(w, "pattern required", 400)
		return
	}
	if body.Action == "" {
		body.Action = "hide"
	}
	if err := a.rules.AddRule(body.Pattern, body.Action); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *adminHandler) handleRemoveRule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern string `json:"pattern"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if body.Pattern == "" {
		http.Error(w, "pattern required", 400)
		return
	}
	if err := a.rules.RemoveRule(body.Pattern); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *adminHandler) handleToggleDefaults(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	if err := a.rules.SetDefaultsEnabled(body.Enabled); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type treeEntry struct {
	Name     string      `json:"name"`
	IsDir    bool        `json:"isDir"`
	Size     int64       `json:"size"`
	Hidden   bool        `json:"hidden"`
	HiddenBy string     `json:"hiddenBy,omitempty"`
	Children []treeEntry `json:"children,omitempty"`
}

func (a *adminHandler) handleTree(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}

	cleanPath := filepath.Clean(reqPath)
	absPath := filepath.Join(a.root, cleanPath)

	if !strings.HasPrefix(absPath, a.root) {
		http.Error(w, "forbidden", 403)
		return
	}

	entries := a.buildTree(absPath, "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"path":    cleanPath,
		"entries": entries,
	})
}

func (a *adminHandler) buildTree(absPath, relPrefix string) []treeEntry {
	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		return nil
	}

	var entries []treeEntry
	for _, e := range dirEntries {
		name := e.Name()
		relPath := filepath.Join(relPrefix, name)
		isDir := e.IsDir()

		hidden, hiddenBy := a.getHiddenInfo(relPath, isDir)

		info, err := e.Info()
		size := int64(-1)
		if err == nil && !isDir {
			size = info.Size()
		}

		entry := treeEntry{
			Name:     name,
			IsDir:    isDir,
			Size:     size,
			Hidden:   hidden,
			HiddenBy: hiddenBy,
		}

		if isDir {
			entry.Children = a.buildTree(filepath.Join(absPath, name), relPath)
		}

		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	return entries
}

func (a *adminHandler) getHiddenInfo(relPath string, isDir bool) (bool, string) {
	base := filepath.Base(relPath)
	if base == ".shareiscare.json" {
		return true, ".shareiscare.json (hardcoded)"
	}

	rules := a.rules.ListRules()
	defaultsEnabled := a.rules.DefaultsEnabled()

	for _, rule := range rules {
		if rule.Action != "hide" {
			continue
		}
		if !defaultsEnabled && rule.Source == "default" {
			continue
		}
		if matchRule(rule.Pattern, relPath, isDir) {
			return true, rule.Pattern
		}
	}

	// Check ancestors.
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for i := 1; i < len(parts); i++ {
		ancestor := strings.Join(parts[:i], "/")
		for _, rule := range rules {
			if rule.Action != "hide" {
				continue
			}
			if !defaultsEnabled && rule.Source == "default" {
				continue
			}
			if matchRule(rule.Pattern, ancestor, true) {
				return true, fmt.Sprintf("%s (via %s)", rule.Pattern, ancestor)
			}
		}
	}

	return false, ""
}
