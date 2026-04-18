// Package config reads and writes the on-disk agiler CLI configuration.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// DefaultAPIBase is used when neither config nor env sets api_base.
	DefaultAPIBase = "https://api.agiler.io"
)

// Config is the on-disk structure. Env vars and CLI flags override these
// fields after Load.
type Config struct {
	APIKey  string `toml:"api_key"`
	APIBase string `toml:"api_base"`
}

// Options controls how a Config is located on disk.
type Options struct {
	// FlagConfig, if non-empty, overrides the path-lookup order.
	FlagConfig string
}

// Dir returns the directory where configs are read/written.
// Honors AGILER_CONFIG_DIR, else uses $HOME/.config/agiler.
func Dir() string {
	if dir := os.Getenv("AGILER_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "agiler")
}

// Path resolves the config file using this lookup order:
//  1. opts.FlagConfig (explicit path; returned as-is)
//  2. ./agiler.toml
//  3. Dir()/config.toml
//  4. /etc/agiler/config.toml
//
// Returns the first existing path, or Dir()/config.toml if nothing exists
// (for config set/path commands).
func Path(opts Options) string {
	if opts.FlagConfig != "" {
		return opts.FlagConfig
	}

	candidates := []string{
		filepath.Join(".", "agiler.toml"),
		filepath.Join(Dir(), "config.toml"),
		filepath.Join("/etc", "agiler", "config.toml"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return filepath.Join(Dir(), "config.toml")
}

// Load reads the config file at Path(opts), then applies env-var overrides.
// Missing files are not an error.
func Load(opts Options) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(Path(opts))
	if err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	if v := os.Getenv("AGILER_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("AGILER_API_BASE"); v != "" {
		cfg.APIBase = v
	}

	if cfg.APIBase == "" {
		cfg.APIBase = DefaultAPIBase
	}

	return cfg, nil
}

// Save writes cfg to Path(opts), creating the parent directory if needed.
func Save(opts Options, cfg *Config) error {
	p := Path(opts)
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(p, buf.Bytes(), 0o600)
}

// Set updates one config field and saves.
func Set(opts Options, key, value string) error {
	cfg, err := Load(opts)
	if err != nil {
		cfg = &Config{}
	}

	switch strings.ReplaceAll(key, "-", "_") {
	case "api_key":
		cfg.APIKey = value
	case "api_base":
		cfg.APIBase = value
	default:
		return fmt.Errorf("unknown config key: %s (valid: api-key, api-base)", key)
	}

	return Save(opts, cfg)
}

// Get returns the value for key.
func Get(opts Options, key string) (string, error) {
	cfg, err := Load(opts)
	if err != nil {
		return "", err
	}

	switch strings.ReplaceAll(key, "-", "_") {
	case "api_key":
		return cfg.APIKey, nil
	case "api_base":
		return cfg.APIBase, nil
	default:
		return "", fmt.Errorf("unknown config key: %s (valid: api-key, api-base)", key)
	}
}
