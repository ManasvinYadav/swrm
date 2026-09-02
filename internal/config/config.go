package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Interface       string `yaml:"interface"`
	DHT             bool   `yaml:"dht"`
	ListenPort      int    `yaml:"listen_port"`
	DownloadDir     string `yaml:"download_dir"`
	PostDownloadCmd string `yaml:"post_download_cmd"`
	DownloadLimit   int    `yaml:"download_limit"`
	UploadLimit     int    `yaml:"upload_limit"`
}

func Load() (*Config, error) {
	ifaceFlag := flag.String("interface", "", "Network interface to bind to (e.g. tun0, wg0)")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(home, ".config", "swrm")
	configPath := filepath.Join(configDir, "config.yaml")

	cfg := &Config{
		DHT:         true,
		ListenPort:  6881,
		DownloadDir: filepath.Join(home, "Downloads", "swrm"),
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read %s: %w", configPath, err)
		}
		// No config file yet: fall back to built-in defaults so a first run
		// doesn't crash. The user can still override via -interface, or by
		// creating configPath later.
	} else if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}

	if *ifaceFlag != "" {
		cfg.Interface = *ifaceFlag
	}
	// Interface binding is optional: an empty value means standard system
	// routing instead of raw-socket VPN binding.
	if cfg.ListenPort < 0 || cfg.ListenPort > 65535 {
		return nil, fmt.Errorf("listen_port must be between 0 and 65535")
	}
	if cfg.DownloadLimit < 0 || cfg.UploadLimit < 0 {
		return nil, fmt.Errorf("rate limits must be zero (unlimited) or positive")
	}
	if cfg.DownloadDir == "~" {
		cfg.DownloadDir = home
	} else if strings.HasPrefix(cfg.DownloadDir, "~/") {
		cfg.DownloadDir = filepath.Join(home, strings.TrimPrefix(cfg.DownloadDir, "~/"))
	}

	return cfg, nil
}
