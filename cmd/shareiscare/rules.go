package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Default sensitive patterns to hide.
var defaultPatterns = []string{
	".env", ".env.*", ".git/", ".gitignore", ".DS_Store", "Thumbs.db",
	"node_modules/", "__pycache__/", ".venv/", "*.key", "*.pem", "*.p12",
	"*.pfx", "id_rsa", "id_ed25519", ".ssh/", "credentials", "credentials.json",
	".netrc", ".npmrc", "*.sqlite", "*.sqlite3", "*.db", ".htpasswd",
	"docker-compose.override.yml", "*.log", ".idea/", ".vscode/",
	"*.swp", "*.swo", "*~", ".shareiscare.json",
}

type Rule struct {
	Pattern   string    `json:"pattern"`
	Action    string    `json:"action"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

type RulesConfig struct {
	Version                int    `json:"version"`
	DefaultPatternsEnabled bool   `json:"defaultPatternsEnabled"`
	Rules                  []Rule `json:"rules"`
}

type RulesEngine struct {
	mu       sync.RWMutex
	config   RulesConfig
	filePath string
}

// NewRulesEngine loads config from disk or creates a new one with optional default seeds.
func NewRulesEngine(configPath string, seedDefaults bool) (*RulesEngine, error) {
	re := &RulesEngine{filePath: configPath}

	data, err := os.ReadFile(configPath)
	if err == nil {
		if err := json.Unmarshal(data, &re.config); err != nil {
			return nil, err
		}
		return re, nil
	}

	// File doesn't exist — create fresh config.
	re.config = RulesConfig{
		Version:                1,
		DefaultPatternsEnabled: seedDefaults,
	}

	if seedDefaults {
		now := time.Now().UTC()
		for _, p := range defaultPatterns {
			re.config.Rules = append(re.config.Rules, Rule{
				Pattern:   p,
				Action:    "hide",
				Source:    "default",
				CreatedAt: now,
			})
		}
	}

	if err := re.save(); err != nil {
		return nil, err
	}
	return re, nil
}

// IsHidden returns true if the given relative path should be hidden.
func (re *RulesEngine) IsHidden(relPath string, isDir bool) bool {
	// Always hide config file itself.
	base := filepath.Base(relPath)
	if base == ".shareiscare.json" {
		return true
	}

	re.mu.RLock()
	defer re.mu.RUnlock()

	for _, rule := range re.config.Rules {
		if rule.Action != "hide" {
			continue
		}
		if !re.config.DefaultPatternsEnabled && rule.Source == "default" {
			continue
		}
		if matchRule(rule.Pattern, relPath, isDir) {
			return true
		}
	}
	return false
}

// IsPathOrAncestorHidden checks the path and every ancestor segment.
func (re *RulesEngine) IsPathOrAncestorHidden(relPath string, isDir bool) bool {
	// Check full path.
	if re.IsHidden(relPath, isDir) {
		return true
	}
	// Check each ancestor directory.
	parts := strings.Split(filepath.ToSlash(relPath), "/")
	for i := 1; i < len(parts); i++ {
		ancestor := strings.Join(parts[:i], "/")
		if re.IsHidden(ancestor, true) {
			return true
		}
	}
	return false
}

// AddRule adds a user rule.
func (re *RulesEngine) AddRule(pattern, action string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	// Avoid duplicates.
	for _, r := range re.config.Rules {
		if r.Pattern == pattern {
			return nil
		}
	}

	re.config.Rules = append(re.config.Rules, Rule{
		Pattern:   pattern,
		Action:    action,
		Source:    "user",
		CreatedAt: time.Now().UTC(),
	})
	return re.save()
}

// RemoveRule removes a rule by pattern.
func (re *RulesEngine) RemoveRule(pattern string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	filtered := re.config.Rules[:0]
	for _, r := range re.config.Rules {
		if r.Pattern != pattern {
			filtered = append(filtered, r)
		}
	}
	re.config.Rules = filtered
	return re.save()
}

// ListRules returns a copy of all rules.
func (re *RulesEngine) ListRules() []Rule {
	re.mu.RLock()
	defer re.mu.RUnlock()

	out := make([]Rule, len(re.config.Rules))
	copy(out, re.config.Rules)
	return out
}

// DefaultsEnabled returns whether default patterns are active.
func (re *RulesEngine) DefaultsEnabled() bool {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.config.DefaultPatternsEnabled
}

// SetDefaultsEnabled toggles default pattern enforcement.
func (re *RulesEngine) SetDefaultsEnabled(enabled bool) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	re.config.DefaultPatternsEnabled = enabled
	return re.save()
}

// save writes config atomically (write tmp + rename).
func (re *RulesEngine) save() error {
	data, err := json.MarshalIndent(re.config, "", "  ")
	if err != nil {
		return err
	}

	tmp := re.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, re.filePath)
}

// matchRule implements gitignore-style pattern matching.
func matchRule(pattern, relPath string, isDir bool) bool {
	relPath = filepath.ToSlash(relPath)

	// Trailing slash means directory-only.
	dirOnly := strings.HasSuffix(pattern, "/")
	if dirOnly {
		if !isDir {
			return false
		}
		pattern = strings.TrimSuffix(pattern, "/")
	}

	// Pattern with slash → match against full relative path.
	if strings.Contains(pattern, "/") {
		matched, _ := filepath.Match(pattern, relPath)
		if matched {
			return true
		}
		// Also try matching with prefix for directories (e.g. "secrets" matching "secrets/foo").
		if strings.HasPrefix(relPath, pattern+"/") || relPath == pattern {
			return true
		}
		return false
	}

	// Pattern without slash → match against basename.
	base := filepath.Base(relPath)

	// Handle dotenv-style patterns like ".env.*".
	matched, _ := filepath.Match(pattern, base)
	if matched {
		return true
	}

	// Also try exact match.
	return base == pattern
}
