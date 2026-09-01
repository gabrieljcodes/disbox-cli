package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL              string `json:"server_url"`
	APIToken               string `json:"api_token"`
	DownloadDir            string `json:"download_dir"`
	MaxConcurrentDownloads int    `json:"max_concurrent_downloads"`
}

func DefaultConfig() Config {
	homeDir, err := os.UserHomeDir()
	downloadDir := ""
	if err == nil {
		downloadDir = filepath.Join(homeDir, "Downloads", "Disbox")
	} else {
		downloadDir = "./downloads"
	}

	return Config{
		ServerURL:              "https://disbox.pousada.space",
		APIToken:               "",
		DownloadDir:            downloadDir,
		MaxConcurrentDownloads: 3,
	}
}

func GetConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".", nil
	}
	configDir := filepath.Join(homeDir, ".config", "disbox")
	return configDir, nil
}

func GetConfigPath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "disbox-config.json", nil
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (Config, error) {
	cfg := DefaultConfig()

	path, err := GetConfigPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			_ = Save(cfg)
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}

	if cfg.ServerURL == "" {
		cfg.ServerURL = "https://disbox.pousada.space"
	}
	if cfg.DownloadDir == "" {
		cfg.DownloadDir = DefaultConfig().DownloadDir
	}
	if cfg.MaxConcurrentDownloads <= 0 {
		cfg.MaxConcurrentDownloads = 3
	}

	return cfg, nil
}

func Save(cfg Config) error {
	dir, err := GetConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := GetConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}
