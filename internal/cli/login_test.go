package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func readConfigFile(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	parsed := map[string]any{}
	if err := toml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	return parsed
}

func TestConfigSetFromStdin(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)

	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("ak_from_stdin\n")
	code := Run(a, context.Background(), []string{"config", "set", "api-key", "-"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	cfg := readConfigFile(t, dir)
	if got := cfg["api_key"]; got != "ak_from_stdin" {
		t.Errorf("api_key = %v, want ak_from_stdin", got)
	}
}

func TestConfigSetStdinTrimsTrailingWhitespace(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)

	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("ak_padded \r\n\n")
	code := Run(a, context.Background(), []string{"config", "set", "api-key", "-"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	cfg := readConfigFile(t, dir)
	if got := cfg["api_key"]; got != "ak_padded" {
		t.Errorf("api_key = %q, want ak_padded", got)
	}
}

func TestConfigSetLiteralValueUnchanged(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)

	a, _, _ := newTestApp(t)
	code := Run(a, context.Background(), []string{"config", "set", "api-key", "ak_literal"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	cfg := readConfigFile(t, dir)
	if got := cfg["api_key"]; got != "ak_literal" {
		t.Errorf("api_key = %v, want ak_literal", got)
	}
}

func TestLoginNonInteractiveReadsStdin(t *testing.T) {
	withInteractiveStub(t, false)
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)

	a, _, errBuf := newTestApp(t)
	a.In = strings.NewReader("ak_pipe_input\n")
	code := Run(a, context.Background(), []string{"login"})
	if code != 0 {
		t.Fatalf("exit %d, stderr=%q", code, errBuf.String())
	}

	cfg := readConfigFile(t, dir)
	if got := cfg["api_key"]; got != "ak_pipe_input" {
		t.Errorf("api_key = %v, want ak_pipe_input", got)
	}
	if !strings.Contains(errBuf.String(), "Logged in.") {
		t.Errorf("expected 'Logged in.' on stderr, got %q", errBuf.String())
	}
}

func TestLoginRejectsEmptyInput(t *testing.T) {
	withInteractiveStub(t, false)
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)

	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("\n")
	code := Run(a, context.Background(), []string{"login"})
	if code == 0 {
		t.Fatal("expected nonzero exit for empty key")
	}

	if _, err := os.Stat(filepath.Join(dir, "config.toml")); err == nil {
		t.Error("config file should not be written for empty key")
	}
}

func TestLoginTrimsWhitespace(t *testing.T) {
	withInteractiveStub(t, false)
	dir := t.TempDir()
	t.Setenv("AGILER_CONFIG_DIR", dir)

	a, _, _ := newTestApp(t)
	a.In = strings.NewReader("  ak_trimmed  \n")
	code := Run(a, context.Background(), []string{"login"})
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	cfg := readConfigFile(t, dir)
	if got := cfg["api_key"]; got != "ak_trimmed" {
		t.Errorf("api_key = %v, want ak_trimmed", got)
	}
}
