package llmconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/gofrs/flock"
)

const (
	ConfigFileName = "config.json"
	ConfigDirName  = "llm"
)

var (
	ConfigRoot = filepath.Join(xdg.ConfigHome, "dagger", ConfigDirName)
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)
)

// Config represents the LLM configuration file structure
type Config struct {
	DefaultProvider string              `json:"default_provider"`
	DefaultModel    string              `json:"default_model"`
	Providers       map[string]Provider `json:"providers"`
}

// Provider represents a single LLM provider's configuration
type Provider struct {
	APIKey           string `json:"api_key"`
	BaseURL          string `json:"base_url,omitempty"`
	AzureVersion     string `json:"azure_version,omitempty"`
	DisableStreaming bool   `json:"disable_streaming,omitempty"`
	Enabled          bool   `json:"enabled"`
}

// Load reads config from disk, returns nil if not exists
func Load() (*Config, error) {
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config is OK
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Initialize providers map if nil
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]Provider)
	}

	return &cfg, nil
}

// Save writes config to disk with proper permissions (0600)
func (c *Config) Save() error {
	// Create directory if needed
	if err := os.MkdirAll(ConfigRoot, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Lock file for atomic writes
	lockFile := ConfigFile + ".lock"
	lock := flock.New(lockFile)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	// Initialize providers map if nil
	if c.Providers == nil {
		c.Providers = make(map[string]Provider)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with 0600 permissions
	if err := os.WriteFile(ConfigFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ConfigExists checks if config file exists
func ConfigExists() bool {
	_, err := os.Stat(ConfigFile)
	return err == nil
}

// Remove deletes the config file
func Remove() error {
	if err := os.Remove(ConfigFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove config file: %w", err)
	}
	return nil
}
