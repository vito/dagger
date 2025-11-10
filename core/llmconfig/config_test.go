package llmconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConfigSaveAndLoad(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Override ConfigRoot and ConfigFile for testing
	origConfigRoot := ConfigRoot
	origConfigFile := ConfigFile
	t.Cleanup(func() {
		ConfigRoot = origConfigRoot
		ConfigFile = origConfigFile
	})
	
	ConfigRoot = filepath.Join(tempDir, "dagger", "llm")
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)

	// Create a test config
	cfg := &Config{
		DefaultProvider: "openrouter",
		DefaultModel:    "anthropic/claude-sonnet-4.5",
		Providers: map[string]Provider{
			"openrouter": {
				APIKey:  "sk-or-v1-test-key",
				BaseURL: "https://openrouter.ai/api/v1",
				Enabled: true,
			},
			"anthropic": {
				APIKey:  "sk-ant-test-key",
				Enabled: false,
			},
		},
	}

	// Save the config
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify the file exists
	if !ConfigExists() {
		t.Fatal("ConfigExists() returned false after Save()")
	}

	// Load the config
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Compare the loaded config with the original
	if diff := cmp.Diff(cfg, loaded); diff != "" {
		t.Errorf("Loaded config differs from original (-want +got):\n%s", diff)
	}
}

func TestConfigFilePermissions(t *testing.T) {
	// Skip on Windows as it doesn't use Unix permissions
	if runtime.GOOS == "windows" {
		t.Skip("Skipping file permission test on Windows")
	}

	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Override ConfigRoot and ConfigFile for testing
	origConfigRoot := ConfigRoot
	origConfigFile := ConfigFile
	t.Cleanup(func() {
		ConfigRoot = origConfigRoot
		ConfigFile = origConfigFile
	})
	
	ConfigRoot = filepath.Join(tempDir, "dagger", "llm")
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)

	// Create and save a minimal config
	cfg := &Config{
		DefaultProvider: "openrouter",
		Providers:       make(map[string]Provider),
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Check file permissions
	info, err := os.Stat(ConfigFile)
	if err != nil {
		t.Fatalf("Stat() failed: %v", err)
	}

	perm := info.Mode().Perm()
	expectedPerm := os.FileMode(0600)
	if perm != expectedPerm {
		t.Errorf("Config file has incorrect permissions: got %o, want %o", perm, expectedPerm)
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Override ConfigRoot and ConfigFile for testing
	origConfigRoot := ConfigRoot
	origConfigFile := ConfigFile
	t.Cleanup(func() {
		ConfigRoot = origConfigRoot
		ConfigFile = origConfigFile
	})
	
	ConfigRoot = filepath.Join(tempDir, "dagger", "llm")
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)

	// Load should return nil, nil for non-existent config
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error on non-existent config: %v", err)
	}
	if cfg != nil {
		t.Errorf("Load() should return nil for non-existent config, got %+v", cfg)
	}
}

func TestLoadMalformedConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Override ConfigRoot and ConfigFile for testing
	origConfigRoot := ConfigRoot
	origConfigFile := ConfigFile
	t.Cleanup(func() {
		ConfigRoot = origConfigRoot
		ConfigFile = origConfigFile
	})
	
	ConfigRoot = filepath.Join(tempDir, "dagger", "llm")
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)

	// Create directory
	if err := os.MkdirAll(ConfigRoot, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Write invalid JSON
	if err := os.WriteFile(ConfigFile, []byte("not valid json"), 0600); err != nil {
		t.Fatalf("Failed to write malformed config: %v", err)
	}

	// Load should return an error
	cfg, err := Load()
	if err == nil {
		t.Fatalf("Load() should error on malformed config, got config: %+v", cfg)
	}
}

func TestConfigRemove(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Override ConfigRoot and ConfigFile for testing
	origConfigRoot := ConfigRoot
	origConfigFile := ConfigFile
	t.Cleanup(func() {
		ConfigRoot = origConfigRoot
		ConfigFile = origConfigFile
	})
	
	ConfigRoot = filepath.Join(tempDir, "dagger", "llm")
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)

	// Create and save a config
	cfg := &Config{
		DefaultProvider: "openrouter",
		Providers:       make(map[string]Provider),
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify it exists
	if !ConfigExists() {
		t.Fatal("ConfigExists() returned false after Save()")
	}

	// Remove the config
	if err := Remove(); err != nil {
		t.Fatalf("Remove() failed: %v", err)
	}

	// Verify it's gone
	if ConfigExists() {
		t.Fatal("ConfigExists() returned true after Remove()")
	}

	// Removing again should not error
	if err := Remove(); err != nil {
		t.Fatalf("Remove() should not error when file doesn't exist: %v", err)
	}
}

func TestConfigJSONSerialization(t *testing.T) {
	// Test that JSON serialization matches the expected schema
	cfg := &Config{
		DefaultProvider: "openrouter",
		DefaultModel:    "anthropic/claude-sonnet-4.5",
		Providers: map[string]Provider{
			"openrouter": {
				APIKey:  "sk-or-v1-test-key",
				BaseURL: "https://openrouter.ai/api/v1",
				Enabled: true,
			},
			"openai": {
				APIKey:           "sk-test-key",
				BaseURL:          "https://api.openai.com/v1",
				AzureVersion:     "2024-02-15",
				DisableStreaming: true,
				Enabled:          true,
			},
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() failed: %v", err)
	}

	// Unmarshal back
	var cfg2 Config
	if err := json.Unmarshal(data, &cfg2); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}

	// Compare
	if diff := cmp.Diff(cfg, &cfg2); diff != "" {
		t.Errorf("JSON round-trip differs (-want +got):\n%s", diff)
	}

	// Verify omitempty fields work
	minimalCfg := &Config{
		DefaultProvider: "anthropic",
		Providers: map[string]Provider{
			"anthropic": {
				APIKey:  "sk-ant-test",
				Enabled: true,
			},
		},
	}

	data, err = json.Marshal(minimalCfg)
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	// Check that omitted fields are not present
	providers := m["providers"].(map[string]interface{})
	anthropic := providers["anthropic"].(map[string]interface{})
	
	if _, exists := anthropic["base_url"]; exists {
		t.Error("base_url should be omitted when empty")
	}
	if _, exists := anthropic["azure_version"]; exists {
		t.Error("azure_version should be omitted when empty")
	}
	if _, exists := anthropic["disable_streaming"]; exists {
		t.Error("disable_streaming should be omitted when false")
	}
}

func TestConfigConcurrentWrites(t *testing.T) {
	// Test that file locking prevents concurrent write corruption
	tempDir := t.TempDir()
	
	// Override ConfigRoot and ConfigFile for testing
	origConfigRoot := ConfigRoot
	origConfigFile := ConfigFile
	t.Cleanup(func() {
		ConfigRoot = origConfigRoot
		ConfigFile = origConfigFile
	})
	
	ConfigRoot = filepath.Join(tempDir, "dagger", "llm")
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)

	// Create initial config
	cfg := &Config{
		DefaultProvider: "openrouter",
		Providers:       make(map[string]Provider),
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Initial Save() failed: %v", err)
	}

	// Perform concurrent writes
	done := make(chan error, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			cfg := &Config{
				DefaultProvider: "openrouter",
				Providers: map[string]Provider{
					"provider": {
						APIKey:  "test-key",
						Enabled: true,
					},
				},
			}
			done <- cfg.Save()
		}(i)
	}

	// Collect results
	for i := 0; i < 10; i++ {
		if err := <-done; err != nil {
			t.Errorf("Concurrent write %d failed: %v", i, err)
		}
	}

	// Verify we can still load the config
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after concurrent writes failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil after concurrent writes")
	}
}

func TestConfigEmptyProviders(t *testing.T) {
	// Test that empty providers map is initialized properly
	cfg := &Config{
		DefaultProvider: "openrouter",
		// Providers is nil
	}

	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Override ConfigRoot and ConfigFile for testing
	origConfigRoot := ConfigRoot
	origConfigFile := ConfigFile
	t.Cleanup(func() {
		ConfigRoot = origConfigRoot
		ConfigFile = origConfigFile
	})
	
	ConfigRoot = filepath.Join(tempDir, "dagger", "llm")
	ConfigFile = filepath.Join(ConfigRoot, ConfigFileName)

	// Save should initialize the map
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load should have initialized map
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.Providers == nil {
		t.Error("Providers map should be initialized after load")
	}
	
	if len(loaded.Providers) != 0 {
		t.Errorf("Providers map should be empty, got %d providers", len(loaded.Providers))
	}
}
