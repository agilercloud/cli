package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	APIKey  string `toml:"api_key"`
	APIBase string `toml:"api_base"`
}

const defaultAPIBase = "https://api.agiler.io"

func configDir() string {
	if dir := os.Getenv("AGILER_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "agiler")
}

// configPath resolves the config file using this lookup order:
//  1. --config flag (explicit path, must exist)
//  2. ./agiler.toml
//  3. ~/.config/agiler/config.toml (or $AGILER_CONFIG_DIR)
//  4. /etc/agiler/config.toml
//
// Returns the first path found, or the default ~/.config/agiler/config.toml
// if nothing exists (for config set/path commands).
func configPath() string {
	if flagConfig != "" {
		return flagConfig
	}

	candidates := []string{
		filepath.Join(".", "agiler.toml"),
		filepath.Join(configDir(), "config.toml"),
		filepath.Join("/etc", "agiler", "config.toml"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// default for writes when no config exists yet
	return filepath.Join(configDir(), "config.toml")
}

func loadConfig() (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(configPath())
	if err == nil {
		if err := toml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parse config: %w", err)
		}
	}

	// env vars override config file
	if v := os.Getenv("AGILER_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("AGILER_API_BASE"); v != "" {
		cfg.APIBase = v
	}

	if cfg.APIBase == "" {
		cfg.APIBase = defaultAPIBase
	}

	return cfg, nil
}

func saveConfig(cfg *Config) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(configPath(), buf.Bytes(), 0600)
}

func configSet(key, value string) error {
	cfg, err := loadConfig()
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

	return saveConfig(cfg)
}

func configGet(key string) (string, error) {
	cfg, err := loadConfig()
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
