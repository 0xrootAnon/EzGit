package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	DataDir     string `json:"data_dir"`
	EnableAudit bool   `json:"enable_audit"`
}

func LoadOrCreate() (*Config, error) {
	home := os.Getenv("HOME")
	if home == "" {
		home = "."
	}
	dir := filepath.Join(home, ".ezgit")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	cfgFile := filepath.Join(dir, "config.json")
	cfg := &Config{
		DataDir:     dir,
		EnableAudit: true,
	}
	if _, err := os.Stat(cfgFile); err == nil {
		b, err := os.ReadFile(cfgFile)
		if err == nil {
			_ = json.Unmarshal(b, cfg)
		}
	} else {
		b, _ := json.MarshalIndent(cfg, "", "  ")
		_ = os.WriteFile(cfgFile, b, 0o600)
	}
	return cfg, nil
}
