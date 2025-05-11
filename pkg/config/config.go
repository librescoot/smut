package config

import (
	"flag"
	"fmt"
)

// Config holds the application configuration
type Config struct {
	// Redis configuration
	RedisAddr   string
	UpdateKey   string
	ChecksumKey string
	FailureKey  string
	UpdateType  string // New field for update type
	Component   string // Component name (dbc, mdb)

	// Download configuration
	DownloadDir string
}

// Parse parses command-line arguments and returns a Config
func Parse() (*Config, error) {
	cfg := &Config{}

	// Redis configuration
	flag.StringVar(&cfg.RedisAddr, "redis-addr", "localhost:6379", "Redis server address")
	flag.StringVar(&cfg.UpdateKey, "update-key", "mender/update/url", "Redis key for update URLs")
	flag.StringVar(&cfg.ChecksumKey, "checksum-key", "mender/update/checksum", "Redis key for checksums")
	flag.StringVar(&cfg.FailureKey, "failure-key", "mender/update/last-failure", "Redis key to set on failure")
	flag.StringVar(&cfg.UpdateType, "update-type", "non-blocking", "Type of update ('blocking' or 'non-blocking')") // New flag

	// Download configuration
	flag.StringVar(&cfg.DownloadDir, "download-dir", "/tmp", "Directory to store downloaded update files")

	// Add component flag
	flag.StringVar(&cfg.Component, "component", "", "Component to update (e.g. dbc, mdb)")

	// Parse flags
	flag.Parse()

	// Validate required parameters
	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("redis-addr is required")
	}
	if cfg.UpdateKey == "" {
		return nil, fmt.Errorf("update-key is required")
	}
	if cfg.FailureKey == "" {
		return nil, fmt.Errorf("failure-key is required")
	}
	if cfg.DownloadDir == "" {
		return nil, fmt.Errorf("download-dir is required")
	}
	if cfg.Component == "" {
		return nil, fmt.Errorf("component is required")
	}

	// Validate update-type
	if cfg.UpdateType != "blocking" && cfg.UpdateType != "non-blocking" {
		return nil, fmt.Errorf("invalid update-type '%s', must be 'blocking' or 'non-blocking'", cfg.UpdateType)
	}

	return cfg, nil
}
