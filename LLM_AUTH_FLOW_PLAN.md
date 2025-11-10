# LLM Authentication Flow Redesign Plan

## Problem Statement

The current LLM authentication system has several issues:
1. **Noisy**: Blindly tries to fetch all provider env vars (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`, etc.)
2. **Slow**: Fetches from secret providers even when keys don't exist
3. **User-hostile**: No way to discover which env vars are needed
4. **Tedious**: Requires mucking with `.env` or `.envrc` files to switch models
5. **Fragmented**: Each provider requires separate API keys and configuration

## Design Goals

1. **Interactive setup**: Prompt users to configure their LLM provider on first use
2. **Secure storage**: Save credentials in `~/.config/dagger/llm` with proper file permissions (0600)
3. **Easy model switching**: Support changing providers/models without editing config files
4. **OpenRouter-first**: Leverage OpenRouter as a unified gateway to reduce key management
5. **Backward compatible**: Continue supporting direct provider keys for advanced users
6. **CLI management**: Add `dagger llm` commands for auth management

## Architecture Overview

### Storage Structure

```
~/.config/dagger/llm/
├── config.json          # Main config (0600 perms)
└── config.json.lock     # File lock for atomic writes
```

**Config Schema:**
```json
{
  "default_provider": "openrouter",
  "default_model": "anthropic/claude-sonnet-4.5",
  "providers": {
    "openrouter": {
      "api_key": "sk-or-v1-...",
      "base_url": "https://openrouter.ai/api/v1",
      "enabled": true
    },
    "anthropic": {
      "api_key": "sk-ant-...",
      "base_url": "",
      "enabled": false
    },
    "openai": {
      "api_key": "sk-...",
      "base_url": "",
      "azure_version": "",
      "disable_streaming": false,
      "enabled": false
    },
    "google": {
      "api_key": "AIza...",
      "base_url": "",
      "enabled": false
    }
  }
}
```

## Implementation Plan

### Phase 1: Configuration Storage Layer

**New file: `internal/llm/config/config.go`**

```go
package config

import (
    "encoding/json"
    "os"
    "path/filepath"
    
    "github.com/adrg/xdg"
    "github.com/gofrs/flock"
)

type Config struct {
    DefaultProvider string              `json:"default_provider"`
    DefaultModel    string              `json:"default_model"`
    Providers       map[string]Provider `json:"providers"`
}

type Provider struct {
    APIKey          string `json:"api_key"`
    BaseURL         string `json:"base_url,omitempty"`
    AzureVersion    string `json:"azure_version,omitempty"`
    DisableStreaming bool  `json:"disable_streaming,omitempty"`
    Enabled         bool   `json:"enabled"`
}

var (
    configRoot = filepath.Join(xdg.ConfigHome, "dagger", "llm")
    configFile = filepath.Join(configRoot, "config.json")
)

// Load reads config from disk, returns empty config if not exists
func Load() (*Config, error)

// Save writes config to disk with proper permissions (0600)
func (c *Config) Save() error

// Provider returns the provider config by name
func (c *Config) Provider(name string) (*Provider, bool)

// SetProvider updates or creates a provider config
func (c *Config) SetProvider(name string, p Provider) error

// DeleteProvider removes a provider from config
func (c *Config) DeleteProvider(name string) error
```

**Key Features:**
- File locking using `github.com/gofrs/flock` (same as Cloud auth)
- Atomic writes with 0600 permissions
- Directory creation with 0755 permissions
- JSON serialization for readability

### Phase 2: Interactive Setup Flow

**New file: `core/llmconfig/setup.go`**

Implements an interactive TUI for first-time setup:

```go
package llmconfig

import (
    "context"
    "fmt"
    "io"
    "strings"
    
    "github.com/charmbracelet/huh"
)

// InteractiveSetup guides user through LLM configuration
// Returns (configured bool, error) - configured is true if setup completed
func InteractiveSetup(ctx context.Context, frontend dagui.Frontend) (bool, error) {
    // 1. Check if config already exists
    if ConfigExists() {
        // Ask if they want to reconfigure
        var reconfigure bool
        form := huh.NewForm(
            huh.NewGroup(
                huh.NewConfirm().
                    Title("LLM configuration already exists").
                    Description("Do you want to reconfigure?").
                    Value(&reconfigure),
            ),
        )
        if err := frontend.HandleForm(ctx, form); err != nil {
            return false, err
        }
        if !reconfigure {
            return false, nil // User chose not to reconfigure
        }
    }
    
    // 2. Present provider choices
    var providerChoice string
    providerForm := huh.NewForm(
        huh.NewGroup(
            huh.NewSelect[string]().
                Title("Choose an LLM provider").
                Description("OpenRouter provides unified access to 100+ models with a single API key").
                Options(
                    huh.NewOption("OpenRouter (recommended)", "openrouter"),
                    huh.NewOption("Anthropic (Claude models)", "anthropic"),
                    huh.NewOption("OpenAI (GPT models)", "openai"),
                    huh.NewOption("Google (Gemini models)", "google"),
                ).
                Value(&providerChoice),
        ),
    )
    
    if err := frontend.HandleForm(ctx, providerForm); err != nil {
        return false, err
    }
    
    // 3. Get API key for chosen provider
    var apiKey string
    var signupURL string
    
    switch providerChoice {
    case "openrouter":
        signupURL = "https://openrouter.ai/keys"
    case "anthropic":
        signupURL = "https://console.anthropic.com/settings/keys"
    case "openai":
        signupURL = "https://platform.openai.com/api-keys"
    case "google":
        signupURL = "https://aistudio.google.com/app/apikey"
    }
    
    keyForm := huh.NewForm(
        huh.NewGroup(
            huh.NewInput().
                Title(fmt.Sprintf("Enter your %s API key", providerChoice)).
                Description(fmt.Sprintf("Get your key at: %s", signupURL)).
                Password(true).
                Value(&apiKey).
                Validate(func(s string) error {
                    if s == "" {
                        return fmt.Errorf("API key cannot be empty")
                    }
                    return nil
                }),
        ),
    )
    
    if err := frontend.HandleForm(ctx, keyForm); err != nil {
        return false, err
    }
    
    // 4. Validate API key (optional but recommended)
    // TODO: Add validation by making a test API call
    
    // 5. Save config
    cfg := &Config{
        DefaultProvider: providerChoice,
        Providers: map[string]Provider{
            providerChoice: {
                APIKey:  apiKey,
                Enabled: true,
            },
        },
    }
    
    // Set default model based on provider
    switch providerChoice {
    case "openrouter":
        cfg.DefaultModel = "anthropic/claude-sonnet-4.5"
    case "anthropic":
        cfg.DefaultModel = "claude-sonnet-4.5"
    case "openai":
        cfg.DefaultModel = "gpt-4.1"
    case "google":
        cfg.DefaultModel = "gemini-2.5-flash"
    }
    
    if err := cfg.Save(); err != nil {
        return false, fmt.Errorf("failed to save config: %w", err)
    }
    
    return true, nil
}

// AutoSetupIfNeeded checks if config exists and prompts user to set up if not
// Returns true if setup was completed, false if skipped or already configured
func AutoSetupIfNeeded(ctx context.Context, frontend dagui.Frontend) (bool, error) {
    if ConfigExists() {
        return false, nil // Already configured
    }
    
    // Check if we're in an interactive terminal
    // (In non-interactive mode, just fail with helpful error)
    if !frontend.Opts().Interactive {
        return false, nil
    }
    
    // Prompt to run setup
    var runSetup bool
    form := huh.NewForm(
        huh.NewGroup(
            huh.NewConfirm().
                Title("No LLM configuration found").
                Description("Would you like to configure it now?").
                Value(&runSetup),
        ),
    )
    
    if err := frontend.HandleForm(ctx, form); err != nil {
        return false, err
    }
    
    if !runSetup {
        return false, nil
    }
    
    return InteractiveSetup(ctx, frontend)
}
```

**Features:**
- Uses charmbracelet/huh for TUI (already a dependency)
- Clear explanations of each provider
- Links to sign up pages displayed inline
- OpenRouter as recommended default
- Supports both manual setup and auto-setup on first use
- Validates API key format
- Can optionally test API keys before saving

### Phase 3: Modified LLMRouter Loading

**Architecture Decision:**
Use existing `file://` secret provider to read config file from `~/.config/dagger/llm/config.json`. The engine parses the JSON and populates the router. This keeps the implementation simple and leverages existing infrastructure.

**Config Schema (Shared between client and engine):**

**New file: `core/llmconfig/config.go`**

```go
package llmconfig

import (
    "encoding/json"
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

type Config struct {
    DefaultProvider string              `json:"default_provider"`
    DefaultModel    string              `json:"default_model"`
    Providers       map[string]Provider `json:"providers"`
}

type Provider struct {
    APIKey          string `json:"api_key"`
    BaseURL         string `json:"base_url,omitempty"`
    AzureVersion    string `json:"azure_version,omitempty"`
    DisableStreaming bool  `json:"disable_streaming,omitempty"`
    Enabled         bool   `json:"enabled"`
}

// Load reads config from disk, returns nil if not exists
func Load() (*Config, error) {
    data, err := os.ReadFile(ConfigFile)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil // No config is OK
        }
        return nil, err
    }
    
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}

// Save writes config to disk with proper permissions (0600)
func (c *Config) Save() error {
    // Create directory if needed
    if err := os.MkdirAll(ConfigRoot, 0755); err != nil {
        return err
    }
    
    // Lock file for atomic writes
    lockFile := ConfigFile + ".lock"
    lock := flock.New(lockFile)
    if err := lock.Lock(); err != nil {
        return err
    }
    defer lock.Unlock()
    
    // Marshal to JSON
    data, err := json.MarshalIndent(c, "", "  ")
    if err != nil {
        return err
    }
    
    // Write with 0600 permissions
    return os.WriteFile(ConfigFile, data, 0600)
}

// ConfigExists checks if config file exists
func ConfigExists() bool {
    _, err := os.Stat(ConfigFile)
    return err == nil
}

// Remove deletes the config file
func Remove() error {
    return os.Remove(ConfigFile)
}
```

**Update: `core/llm.go`**

```go
import (
    "github.com/dagger/dagger/core/llmconfig"
)

func NewLLMRouter(ctx context.Context, srv *dagql.Server) (_ *LLMRouter, rerr error) {
    router := new(LLMRouter)
    
    // Get the secret plaintext, from either a URI (provider lookup) or a plaintext (no-op)
    loadSecret := func(ctx context.Context, uriOrPlaintext string) (string, error) {
        // ... existing loadSecret implementation
    }
    
    ctx, span := Tracer(ctx).Start(ctx, "load LLM router config", telemetry.Internal(), telemetry.Encapsulate())
    defer telemetry.End(span, func() error { return rerr })
    
    // Priority order:
    // 1. Config file (~/.config/dagger/llm/config.json) - base layer
    // 2. .env file - middle layer (legacy support)
    // 3. Environment variables - top layer (overrides everything)
    
    // First: Try loading from config file via file:// secret provider
    configPath := "file://" + llmconfig.ConfigFile
    if configBytes, err := loadSecret(ctx, configPath); err == nil {
        var cfg llmconfig.Config
        if err := json.Unmarshal([]byte(configBytes), &cfg); err == nil {
            router.LoadFromConfig(&cfg)
        }
    }
    
    // Second: Load .env file (existing logic)
    env := make(map[string]string)
    if envFile, err := loadSecret(ctx, "file://.env"); err == nil {
        if e, err := godotenv.Unmarshal(envFile); err == nil {
            env = e
        }
    }
    
    // Third: Load environment variables (highest priority, overrides config file)
    err := router.LoadConfig(ctx, func(ctx context.Context, k string) (string, error) {
        // First lookup in the .env file
        if v, ok := env[k]; ok {
            return loadSecret(ctx, v)
        }
        // Second: lookup in client env directly
        if v, err := loadSecret(ctx, "env://"+k); err == nil {
            // Allow the env var itself to be a secret reference
            return loadSecret(ctx, v)
        }
        return "", nil
    })
    
    if err != nil {
        return nil, err
    }
    
    // If no config found at all, return helpful error
    if router.IsEmpty() {
        return nil, &ErrNoLLMConfig{}
    }
    
    return router, nil
}

// LoadFromConfig populates router from config file (base layer)
func (r *LLMRouter) LoadFromConfig(cfg *llmconfig.Config) {
    for name, provider := range cfg.Providers {
        if !provider.Enabled {
            continue
        }
        
        switch name {
        case "openrouter":
            // Only set if not already set (env vars take priority)
            if r.OpenAIAPIKey == "" {
                r.OpenAIAPIKey = provider.APIKey
            }
            if r.OpenAIBaseURL == "" {
                r.OpenAIBaseURL = "https://openrouter.ai/api/v1"
            }
        case "anthropic":
            if r.AnthropicAPIKey == "" {
                r.AnthropicAPIKey = provider.APIKey
            }
            if r.AnthropicBaseURL == "" && provider.BaseURL != "" {
                r.AnthropicBaseURL = provider.BaseURL
            }
        case "openai":
            if r.OpenAIAPIKey == "" {
                r.OpenAIAPIKey = provider.APIKey
            }
            if r.OpenAIBaseURL == "" && provider.BaseURL != "" {
                r.OpenAIBaseURL = provider.BaseURL
            }
            if !r.OpenAIDisableStreaming && provider.DisableStreaming {
                r.OpenAIDisableStreaming = provider.DisableStreaming
            }
            if r.OpenAIAzureVersion == "" && provider.AzureVersion != "" {
                r.OpenAIAzureVersion = provider.AzureVersion
            }
        case "google", "gemini":
            if r.GeminiAPIKey == "" {
                r.GeminiAPIKey = provider.APIKey
            }
            if r.GeminiBaseURL == "" && provider.BaseURL != "" {
                r.GeminiBaseURL = provider.BaseURL
            }
        }
    }
}

func (r *LLMRouter) IsEmpty() bool {
    return r.OpenAIAPIKey == "" && 
           r.AnthropicAPIKey == "" && 
           r.GeminiAPIKey == ""
}

type ErrNoLLMConfig struct{}

func (e *ErrNoLLMConfig) Error() string {
    return fmt.Sprintf(`No LLM configuration found.

To get started, run:
    dagger llm setup

Or set environment variables:
    export ANTHROPIC_API_KEY=sk-ant-...
    export OPENAI_API_KEY=sk-...
    export GEMINI_API_KEY=AIza...

For unified access to all models with a single key:
    https://openrouter.ai/keys
`)
}
```

### Phase 4: CLI Commands

**New file: `cmd/dagger/llm_config.go`**

```go
package main

import (
    "context"
    "fmt"
    "os"
    "strings"
    
    "github.com/spf13/cobra"
    "github.com/dagger/dagger/core/llmconfig"
    "github.com/dagger/dagger/dagql/idtui"
)

var llmCmd = &cobra.Command{
    Use:   "llm",
    Short: "Manage LLM configuration",
}

var llmSetupCmd = &cobra.Command{
    Use:   "setup",
    Short: "Configure LLM authentication interactively",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Create a frontend for interactive prompts
        frontend := idtui.NewPlain(cmd.OutOrStdout())
        
        configured, err := llmconfig.InteractiveSetup(cmd.Context(), frontend)
        if err != nil {
            return err
        }
        
        if configured {
            fmt.Fprintln(cmd.OutOrStdout(), "✓ LLM configuration saved successfully!")
        } else {
            fmt.Fprintln(cmd.OutOrStdout(), "Setup cancelled.")
        }
        
        return nil
    },
}

var llmConfigCmd = &cobra.Command{
    Use:   "config",
    Short: "Display current LLM configuration",
    RunE: func(cmd *cobra.Command, args []string) error {
        cfg, err := llmconfig.Load()
        if err != nil {
            return err
        }
        
        if cfg == nil {
            fmt.Fprintln(cmd.OutOrStdout(), "No LLM configuration found.")
            fmt.Fprintln(cmd.OutOrStdout(), "Run 'dagger llm setup' to configure.")
            return nil
        }
        
        // Pretty-print with API keys redacted
        fmt.Fprintf(cmd.OutOrStdout(), "Default Provider: %s\n", cfg.DefaultProvider)
        if cfg.DefaultModel != "" {
            fmt.Fprintf(cmd.OutOrStdout(), "Default Model: %s\n", cfg.DefaultModel)
        }
        fmt.Fprintf(cmd.OutOrStdout(), "\nConfigured Providers:\n")
        
        for name, provider := range cfg.Providers {
            if provider.Enabled {
                redacted := redactAPIKey(provider.APIKey)
                fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s: %s\n", name, redacted)
                if provider.BaseURL != "" {
                    fmt.Fprintf(cmd.OutOrStdout(), "    Base URL: %s\n", provider.BaseURL)
                }
            }
        }
        
        fmt.Fprintf(cmd.OutOrStdout(), "\nConfig file: %s\n", llmconfig.ConfigFile)
        return nil
    },
}

var llmAddKeyCmd = &cobra.Command{
    Use:   "add-key <provider>",
    Short: "Add or update API key for a provider",
    Long: `Add or update API key for a provider.

Supported providers:
  - openrouter: Unified access to 100+ models (https://openrouter.ai/keys)
  - anthropic: Claude models (https://console.anthropic.com/settings/keys)
  - openai: GPT models (https://platform.openai.com/api-keys)
  - google: Gemini models (https://aistudio.google.com/app/apikey)
`,
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        provider := args[0]
        
        // Validate provider name
        validProviders := []string{"openrouter", "anthropic", "openai", "google"}
        if !contains(validProviders, provider) {
            return fmt.Errorf("unsupported provider %q, must be one of: %s", 
                provider, strings.Join(validProviders, ", "))
        }
        
        // Prompt for API key
        fmt.Fprintf(cmd.OutOrStdout(), "Enter API key for %s: ", provider)
        var apiKey string
        if _, err := fmt.Scanln(&apiKey); err != nil {
            return err
        }
        
        apiKey = strings.TrimSpace(apiKey)
        if apiKey == "" {
            return fmt.Errorf("API key cannot be empty")
        }
        
        // TODO: Optionally validate key by making test API call
        
        // Load or create config
        cfg, err := llmconfig.Load()
        if err != nil {
            return err
        }
        if cfg == nil {
            cfg = &llmconfig.Config{
                DefaultProvider: provider,
                Providers: make(map[string]llmconfig.Provider),
            }
        }
        
        // Add or update provider
        cfg.Providers[provider] = llmconfig.Provider{
            APIKey:  apiKey,
            Enabled: true,
        }
        
        // If this is the first provider, set it as default
        if cfg.DefaultProvider == "" {
            cfg.DefaultProvider = provider
        }
        
        if err := cfg.Save(); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
        
        fmt.Fprintf(cmd.OutOrStdout(), "✓ API key for %s saved successfully!\n", provider)
        return nil
    },
}

var llmRemoveKeyCmd = &cobra.Command{
    Use:   "remove-key <provider>",
    Short: "Remove API key for a provider",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        provider := args[0]
        
        cfg, err := llmconfig.Load()
        if err != nil {
            return err
        }
        if cfg == nil {
            return fmt.Errorf("no LLM configuration found")
        }
        
        if _, ok := cfg.Providers[provider]; !ok {
            return fmt.Errorf("provider %q not found in config", provider)
        }
        
        delete(cfg.Providers, provider)
        
        // If this was the default provider, clear it
        if cfg.DefaultProvider == provider {
            cfg.DefaultProvider = ""
        }
        
        if err := cfg.Save(); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
        
        fmt.Fprintf(cmd.OutOrStdout(), "✓ API key for %s removed.\n", provider)
        return nil
    },
}

var llmSetDefaultCmd = &cobra.Command{
    Use:   "set-default <provider> [model]",
    Short: "Set default provider and optionally model",
    Args:  cobra.RangeArgs(1, 2),
    RunE: func(cmd *cobra.Command, args []string) error {
        provider := args[0]
        
        cfg, err := llmconfig.Load()
        if err != nil {
            return err
        }
        if cfg == nil {
            return fmt.Errorf("no LLM configuration found, run 'dagger llm setup' first")
        }
        
        // Verify provider exists
        if _, ok := cfg.Providers[provider]; !ok {
            return fmt.Errorf("provider %q not configured, run 'dagger llm add-key %s' first", 
                provider, provider)
        }
        
        cfg.DefaultProvider = provider
        if len(args) > 1 {
            cfg.DefaultModel = args[1]
        }
        
        if err := cfg.Save(); err != nil {
            return fmt.Errorf("failed to save config: %w", err)
        }
        
        fmt.Fprintf(cmd.OutOrStdout(), "✓ Default provider set to: %s\n", provider)
        if len(args) > 1 {
            fmt.Fprintf(cmd.OutOrStdout(), "✓ Default model set to: %s\n", args[1])
        }
        return nil
    },
}

var llmResetCmd = &cobra.Command{
    Use:   "reset",
    Short: "Reset LLM configuration (removes all stored credentials)",
    RunE: func(cmd *cobra.Command, args []string) error {
        if !llmconfig.ConfigExists() {
            fmt.Fprintln(cmd.OutOrStdout(), "No LLM configuration found.")
            return nil
        }
        
        // Confirm before deleting
        fmt.Fprint(cmd.OutOrStdout(), "This will delete all stored LLM credentials. Continue? [y/N]: ")
        var response string
        if _, err := fmt.Scanln(&response); err != nil {
            return err
        }
        
        response = strings.ToLower(strings.TrimSpace(response))
        if response != "y" && response != "yes" {
            fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
            return nil
        }
        
        if err := llmconfig.Remove(); err != nil {
            return err
        }
        
        fmt.Fprintln(cmd.OutOrStdout(), "✓ LLM configuration has been reset.")
        return nil
    },
}

func init() {
    llmCmd.AddCommand(
        llmSetupCmd,
        llmConfigCmd,
        llmAddKeyCmd,
        llmRemoveKeyCmd,
        llmSetDefaultCmd,
        llmResetCmd,
    )
    rootCmd.AddCommand(llmCmd)
}

// Helper functions

func redactAPIKey(key string) string {
    if len(key) <= 8 {
        return "***"
    }
    return key[:8] + "..." + "***"
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

**Commands Summary:**
- `dagger llm setup` - Interactive first-time configuration (TUI)
- `dagger llm config` - Display current config (keys redacted)
- `dagger llm add-key <provider>` - Add/update API key
- `dagger llm remove-key <provider>` - Remove API key
- `dagger llm set-default <provider> [model]` - Set defaults
- `dagger llm reset` - Remove all configuration

**Note:** The `llm models` command is deferred to a future phase as it requires additional API integration work.

### Phase 5: Integration with LLM Session Flow

**Update: `cmd/dagger/llm.go`**

Add auto-setup trigger when starting an LLM session:

```go
func (cmd *llmCmd) Command() *cobra.Command {
    // ... existing setup ...
    
    return &cobra.Command{
        Use:   "llm [flags] [message]",
        Short: "Start an interactive LLM session",
        RunE: func(c *cobra.Command, args []string) error {
            // Check if config exists, and if not, offer to set it up
            if !llmconfig.ConfigExists() {
                // Check if we have env vars as fallback
                hasEnvVars := os.Getenv("ANTHROPIC_API_KEY") != "" ||
                              os.Getenv("OPENAI_API_KEY") != "" ||
                              os.Getenv("GEMINI_API_KEY") != ""
                
                if !hasEnvVars {
                    // No config file and no env vars - prompt for setup
                    frontend := opts.Frontend
                    
                    configured, err := llmconfig.AutoSetupIfNeeded(c.Context(), frontend)
                    if err != nil {
                        return err
                    }
                    
                    if !configured {
                        // User declined setup - show error
                        return fmt.Errorf(`No LLM configuration found.

To get started, run:
    dagger llm setup

Or set environment variables:
    export ANTHROPIC_API_KEY=sk-ant-...
    export OPENAI_API_KEY=sk-...
    export GEMINI_API_KEY=AIza...

For unified access to all models with a single key:
    https://openrouter.ai/keys
`)
                    }
                }
            }
            
            // Continue with normal LLM session flow
            return cmd.runLLMSession(c, args)
        },
    }
}
```

This ensures:
1. **Automatic setup prompt** when user runs `dagger llm` without config
2. **Fallback to env vars** if they exist (backward compatibility)
3. **Skip prompt** if config already exists
4. **Graceful failure** with helpful error if user declines setup

### Phase 6: OpenRouter Integration Enhancements

**Why OpenRouter as Default:**
1. **Single API Key**: One key provides access to 100+ models across 20+ providers
2. **Unified Interface**: OpenAI-compatible API works with existing client code
3. **Cost Transparency**: Pricing data for all models via API
4. **Automatic Fallback**: If one provider is down, OpenRouter routes to alternatives
5. **No Multi-Key Management**: Users don't need separate keys for each provider

**Implementation:**
- Setup flow recommends OpenRouter first with clear explanation
- Show pricing comparison between direct provider vs OpenRouter in docs
- Provide clear sign-up flow: https://openrouter.ai/keys
- Auto-configure OpenRouter as OpenAI-compatible endpoint with base URL: `https://openrouter.ai/api/v1`

**Model Routing with OpenRouter:**
When OpenRouter is configured, all models become accessible using OpenRouter's model naming:
- `anthropic/claude-sonnet-4.5`
- `openai/gpt-4.1`
- `google/gemini-2.5-flash`
- `meta-llama/llama-3.2`
- And 100+ more

The router automatically uses the OpenAI client with OpenRouter's base URL.

### Phase 7: Backward Compatibility

**Environment Variable Priority:**

The loading order in `NewLLMRouter` ensures proper priority:

1. **Config file** (`~/.config/dagger/llm/config.json`) - Base layer, loaded first
2. **`.env` file** - Middle layer for local overrides (legacy support)
3. **Environment variables** - Top layer, always takes precedence

The `LoadFromConfig` method only sets router fields if they're empty, so env vars naturally override config file values.

**Migration Path:**
- **Existing users with env vars**: No changes required, env vars continue to work and take priority
- **New users**: Guided through `dagger llm setup` for easier onboarding
- **CI/CD**: Continue using env vars or secret providers (e.g., `env://MY_SECRET`)
- **Mixed mode**: Can use config file for local dev, env vars for CI/CD

**Auto-detection Logic:**

The current implementation in `NewLLMRouter` already supports this:

```go
// 1. Load config file (base layer)
configPath := "file://" + llmconfig.ConfigFile
if configBytes, err := loadSecret(ctx, configPath); err == nil {
    var cfg llmconfig.Config
    if err := json.Unmarshal([]byte(configBytes), &cfg); err == nil {
        router.LoadFromConfig(&cfg)  // Only sets empty fields
    }
}

// 2. Load .env file (middle layer)
// ... existing code ...

// 3. Load environment variables (top layer, overrides everything)
err := router.LoadConfig(ctx, getenv)
```

**Graceful Degradation:**
- If config file doesn't exist: silently skip, check env vars
- If config file is malformed: log warning, continue with env vars
- If no config at all: return `ErrNoLLMConfig` with helpful message

### Phase 8: Error Handling & User Experience

**Improved Error Messages:**

Already implemented in Phase 3 as `ErrNoLLMConfig`:

```go
type ErrNoLLMConfig struct{}

func (e *ErrNoLLMConfig) Error() string {
    return `No LLM configuration found.

To get started, run:
    dagger llm setup

Or set environment variables:
    export ANTHROPIC_API_KEY=sk-ant-...
    export OPENAI_API_KEY=sk-...
    export GEMINI_API_KEY=AIza...

For unified access to all models with a single key:
    https://openrouter.ai/keys
`
}
```

**Validation (Future Enhancement):**
- Test API keys during setup by making a minimal API call
- Provide clear feedback if keys are invalid
- Suggest fixes for common errors (expired keys, rate limits, incorrect format)
- Show example key formats per provider

**Progress Indicators:**
- Use TUI spinners during API validation (via charmbracelet/huh)
- Clear success/failure messages with checkmarks
- Helpful next steps after configuration complete

**File Permission Warnings:**
```go
// In config.Save()
if info, err := os.Stat(ConfigFile); err == nil {
    if info.Mode().Perm() != 0600 {
        fmt.Fprintf(os.Stderr, "Warning: Config file has insecure permissions %o, expected 0600\n", info.Mode().Perm())
    }
}
```

## Security Considerations

### File Permissions
- Config file: **0600** (owner read/write only)
- Config directory: **0755** (standard config directory)
- Lock file: **0600** (consistent with config)

### API Key Storage
- Keys stored in plaintext JSON (standard practice for local CLI tools)
- File permissions prevent other users from reading
- No encryption (would require passphrase, degrading UX)
- Follows same pattern as Docker config, AWS credentials, etc.

### API Key Redaction
- Always redact in logs and displays: `sk-ant-...xyz` → `sk-ant-...***`
- Never send API keys in telemetry
- Warn users if config file permissions are too open

### Lock File
- Use `github.com/gofrs/flock` for atomic writes (same as Cloud auth)
- 3-second timeout for lock acquisition
- Prevents corruption from concurrent writes

## Testing Strategy

### Unit Tests
- Config serialization/deserialization
- File locking behavior
- API key validation
- Provider routing logic

### Integration Tests
- Interactive setup flow (mocked prompts)
- Config migration from env vars
- OpenRouter model listing
- Multi-provider configuration

### Manual Testing
- First-time user experience
- Model switching workflow
- API key rotation
- Permission verification on multiple platforms

## Migration Guide for Users

### For New Users
```bash
# First time setup
dagger llm setup

# Follow prompts to configure OpenRouter or other providers
# Start using LLM features immediately
```

### For Existing Users (Environment Variables)
```bash
# No changes required! Your env vars still work.
# Optionally migrate to config file:
dagger llm setup
# Then remove env vars from your .env or .envrc
```

### For Advanced Users (Multiple Providers)
```bash
# Configure multiple providers
dagger llm add-key openrouter
dagger llm add-key anthropic
dagger llm add-key openai

# Set default
dagger llm set-default openrouter anthropic/claude-sonnet-4.5

# Switch models on the fly (in shell)
llm --model openai/gpt-4
```

## Implementation Phases

### Phase 1: Configuration Storage Layer (Week 1)
- [x] Create `core/llmconfig` package structure
- [ ] Implement `Config` struct and JSON serialization
- [ ] Add file I/O with proper permissions (0600)
- [ ] Implement file locking using `github.com/gofrs/flock`
- [ ] Write unit tests for config operations
- [ ] Test file permissions on Linux/macOS/Windows

### Phase 2: Interactive Setup Flow (Week 1-2)
- [ ] Implement `InteractiveSetup()` using charmbracelet/huh
- [ ] Add provider selection UI
- [ ] Implement API key input with password masking
- [ ] Add `AutoSetupIfNeeded()` for first-run experience
- [ ] Write tests for setup flow (mocked prompts)

### Phase 3: Engine Integration (Week 2)
- [ ] Update `NewLLMRouter` to load config via `file://` provider
- [ ] Implement `LoadFromConfig()` method
- [ ] Add JSON parsing and error handling
- [ ] Ensure env var priority is maintained
- [ ] Add `ErrNoLLMConfig` error type
- [ ] Write integration tests for router loading

### Phase 4: CLI Commands (Week 2)
- [ ] Create `cmd/dagger/llm_config.go`
- [ ] Implement `dagger llm setup` command
- [ ] Implement `dagger llm config` command (display)
- [ ] Implement `dagger llm add-key` command
- [ ] Implement `dagger llm remove-key` command
- [ ] Implement `dagger llm set-default` command
- [ ] Implement `dagger llm reset` command
- [ ] Add command tests

### Phase 5: LLM Session Integration (Week 2-3)
- [ ] Update `cmd/dagger/llm.go` to trigger auto-setup
- [ ] Add config existence check before session start
- [ ] Implement fallback to env vars for backward compatibility
- [ ] Test end-to-end flow: setup → session start
- [ ] Update error messages for missing config

### Phase 6: OpenRouter Enhancements (Week 3)
- [ ] Update setup flow to recommend OpenRouter first
- [ ] Add clear OpenRouter benefits explanation
- [ ] Provide sign-up URL in prompts
- [ ] Ensure OpenRouter model naming works correctly
- [ ] Test OpenRouter integration end-to-end
- [ ] Document OpenRouter setup in user guide

### Phase 7: Backward Compatibility Testing (Week 3)
- [ ] Test with existing env vars (no config file)
- [ ] Test with config file only (no env vars)
- [ ] Test with both (env vars should override)
- [ ] Test with `.env` file
- [ ] Test CI/CD scenario (env vars + secret providers)
- [ ] Verify graceful degradation when config is malformed

### Phase 8: Polish & Documentation (Week 3-4)
- [ ] Add file permission warnings
- [ ] Implement API key validation (optional)
- [ ] Add progress indicators and spinners
- [ ] Write user documentation
- [ ] Write migration guide
- [ ] Update CLI help text
- [ ] Add examples for common workflows
- [ ] Test on all platforms (Linux, macOS, Windows)

### Phase 9: Rollout (Week 4)
- [ ] Merge to main branch
- [ ] Update changelog
- [ ] Announce in community channels
- [ ] Monitor for issues
- [ ] Collect user feedback

## Future Enhancements

### V2 Features (Post-MVP)
1. **Model aliases**: `dagger llm alias my-agent anthropic/claude-sonnet-4.5`
2. **Cost budgets**: `dagger llm set-budget $10/day`
3. **Usage analytics**: `dagger llm usage --last-week`
4. **Team sharing**: Export/import sanitized configs
5. **Model testing**: `dagger llm benchmark <model>` - compare speed/quality
6. **Prompt templates**: Store and reuse common prompts
7. **API key rotation**: `dagger llm rotate-key <provider>`
8. **Multi-org support**: Switch between different OpenRouter/Cloud accounts

### Enterprise Features
1. **Centralized key management**: Fetch keys from vault/secrets manager
2. **OIDC integration**: Use corporate identity for API access
3. **Audit logging**: Track all LLM API usage
4. **Compliance**: Ensure data residency requirements

## Key Architecture Decisions

### 1. Use file:// Secret Provider (Not a New Provider)

**Decision:** Leverage the existing `file://` secret provider to read the config file from `~/.config/dagger/llm/config.json` instead of creating a new `config://` or `llmconfig://` provider.

**Rationale:**
- Simpler implementation - no new provider code needed
- The engine already has full support for reading files via `file://`
- Home directory expansion (`~`) is already handled
- Config parsing happens in the engine, keeping the schema coupled (acceptable trade-off)
- Natural priority layering: config file (base) → .env (middle) → env vars (top)

**Trade-offs:**
- Engine and client share config schema (tight coupling)
- Config file must be valid JSON (no partial reads of individual fields)
- Cannot selectively fetch individual keys without parsing whole file

**Accepted because:** The config file is small, parsing is fast, and tight coupling between engine/client config schema is acceptable for this use case.

### 2. Config File Location

**Decision:** Store config at `~/.config/dagger/llm/config.json` (XDG standard)

**Rationale:**
- Follows XDG Base Directory Specification
- Consistent with modern CLI tools (gh, kubectl, docker)
- Uses `github.com/adrg/xdg` package (already a dependency)
- Platform-specific defaults work correctly (Linux, macOS, Windows)

### 3. Priority Order for Configuration

**Decision:** Config file < .env file < Environment variables

**Rationale:**
- Environment variables are the highest priority (standard practice)
- Allows CI/CD to override config file with secrets
- `.env` file sits in the middle for local overrides
- Config file is the base layer for persistent user settings
- `LoadFromConfig()` only sets empty fields, allowing natural override behavior

### 4. Hybrid Auto-Setup Approach

**Decision:** Auto-prompt for setup on first `dagger llm` run, with option to skip and run `dagger llm setup` manually later.

**Rationale:**
- Best of both worlds: automatic for new users, explicit for those who prefer it
- Respects user agency (can decline auto-setup)
- Fails gracefully with helpful error message if declined
- Still allows `dagger llm setup` to be run manually anytime
- Skips auto-prompt if env vars are already set (backward compatibility)

### 5. OpenRouter as Recommended Default

**Decision:** Prominently recommend OpenRouter in setup flow, but don't require it.

**Rationale:**
- Single API key for 100+ models reduces friction
- OpenAI-compatible API works with existing client code
- Automatic fallback if providers are down
- Simplifies key management significantly
- Users can still choose direct providers if preferred (enterprise, cost optimization, etc.)

### 6. No API Key Encryption

**Decision:** Store API keys in plaintext JSON with 0600 file permissions.

**Rationale:**
- Standard practice for CLI tools (Docker, AWS CLI, Kubernetes, etc.)
- Encryption would require passphrase, degrading UX
- File permissions prevent other users from reading
- Users who need encryption can use secret providers (env://, vault://, etc.)
- Alternative: use OS keychain via `libsecret://` provider for higher security

### 7. Graceful Backward Compatibility

**Decision:** Maintain full backward compatibility with existing env var workflows.

**Rationale:**
- Existing users don't need to change anything
- CI/CD pipelines continue to work unchanged
- No breaking changes
- New config system is purely additive
- Env vars naturally override config file (users expect this)

## Success Metrics

- **Setup time**: < 2 minutes from first `dagger llm setup` to working LLM
- **Key switching**: < 10 seconds to rotate API key
- **Error rate**: Zero failed authentications due to config issues
- **Adoption**: 80%+ of users on config file within 3 months
- **Support tickets**: 50% reduction in LLM auth-related issues

## Documentation Updates

### User Guide
- New "Getting Started with LLM" section
- OpenRouter setup guide
- Provider comparison table
- Migration guide from env vars

### Reference
- `dagger llm` command documentation
- Config file schema reference
- Environment variable reference (legacy)
- Security best practices

### Examples
- Basic OpenRouter setup
- Multi-provider configuration
- CI/CD setup patterns
- Model switching examples
