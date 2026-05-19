package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)
	t.Setenv("AGILER_API_KEY", "")
	t.Setenv("AGILER_API_BASE", "")
	t.Setenv("AGILER_WORKSPACE_ID", "")

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIBase != DefaultAPIBase {
		t.Errorf("APIBase = %q, want %q", cfg.APIBase, DefaultAPIBase)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", cfg.APIKey)
	}
	if cfg.WorkspaceID != "" {
		t.Errorf("WorkspaceID = %q, want empty", cfg.WorkspaceID)
	}
}

func TestLoadFromTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)
	t.Setenv("AGILER_API_KEY", "")
	t.Setenv("AGILER_API_BASE", "")
	t.Setenv("AGILER_WORKSPACE_ID", "")

	content := `api_key = "K1"
api_base = "https://custom.example.com"
workspace_id = "00000000-0000-0000-0000-000000000001"`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "K1" {
		t.Errorf("APIKey = %q", cfg.APIKey)
	}
	if cfg.APIBase != "https://custom.example.com" {
		t.Errorf("APIBase = %q", cfg.APIBase)
	}
	if cfg.WorkspaceID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("WorkspaceID = %q", cfg.WorkspaceID)
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)
	t.Setenv("AGILER_API_KEY", "FROM_ENV")
	t.Setenv("AGILER_API_BASE", "https://env.example.com")
	t.Setenv("AGILER_WORKSPACE_ID", "00000000-0000-0000-0000-000000000002")

	content := `api_key = "FROM_FILE"`
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "FROM_ENV" {
		t.Errorf("env should override file: got %q", cfg.APIKey)
	}
	if cfg.APIBase != "https://env.example.com" {
		t.Errorf("env APIBase: got %q", cfg.APIBase)
	}
	if cfg.WorkspaceID != "00000000-0000-0000-0000-000000000002" {
		t.Errorf("env WorkspaceID: got %q", cfg.WorkspaceID)
	}
}

func TestFlagConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)
	t.Setenv("AGILER_API_KEY", "")
	t.Setenv("AGILER_API_BASE", "")
	t.Setenv("AGILER_WORKSPACE_ID", "")

	flagPath := filepath.Join(dir, "flag.toml")
	if err := os.WriteFile(flagPath, []byte(`api_key = "FLAG"`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Options{FlagConfig: flagPath})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey != "FLAG" {
		t.Errorf("--config path not honored: %q", cfg.APIKey)
	}
}

func TestSetAndGet(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)
	t.Setenv("AGILER_API_KEY", "")
	t.Setenv("AGILER_API_BASE", "")
	t.Setenv("AGILER_WORKSPACE_ID", "")

	if err := Set(Options{}, "api-key", "abc"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	v, err := Get(Options{}, "api-key")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if v != "abc" {
		t.Errorf("Get api-key = %q, want abc", v)
	}

	if err := Set(Options{}, "workspace-id", "00000000-0000-0000-0000-000000000003"); err != nil {
		t.Fatalf("Set workspace-id: %v", err)
	}
	v, err = Get(Options{}, "workspace-id")
	if err != nil {
		t.Fatalf("Get workspace-id: %v", err)
	}
	if v != "00000000-0000-0000-0000-000000000003" {
		t.Errorf("Get workspace-id = %q", v)
	}

	if _, err := Get(Options{}, "unknown-key"); err == nil {
		t.Error("expected error for unknown key")
	}
}

func TestOSLoader(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)
	t.Setenv("AGILER_API_KEY", "")
	t.Setenv("AGILER_API_BASE", "")
	t.Setenv("AGILER_WORKSPACE_ID", "")

	l := NewOSLoader(Options{})
	if err := l.Set("api-base", "https://x.example.com"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	cfg, err := l.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIBase != "https://x.example.com" {
		t.Errorf("APIBase = %q", cfg.APIBase)
	}
	if l.Path() == "" {
		t.Error("Path is empty")
	}
}
