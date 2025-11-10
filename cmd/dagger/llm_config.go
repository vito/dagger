package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/vito/go-interact/interact"

	"github.com/dagger/dagger/core/llmconfig"
)

// llmCmd is the parent command for all LLM-related subcommands
var llmCmd = &cobra.Command{
	Use:   "llm",
	Short: "Manage LLM configuration and authentication",
	Long: `Manage LLM (Large Language Model) configuration and authentication.

The llm command provides subcommands to configure API keys for various LLM providers
(OpenRouter, Anthropic, OpenAI, Google), view current configuration, and manage defaults.

Configuration is stored in ~/.config/dagger/llm/config.json with 0600 permissions.

For interactive setup, run:
    dagger llm setup

To view current configuration:
    dagger llm config`,
}

func init() {
	// Register all llm subcommands
	llmCmd.AddCommand(
		llmSetupCmd,
		llmConfigCmd,
		llmAddKeyCmd,
		llmRemoveKeyCmd,
		llmSetDefaultCmd,
		llmResetCmd,
		llmShowConfigCmd,
	)
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

var llmSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure LLM authentication interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use a standalone prompt handler for form-based input
		// (Frontend may be Pretty which requires a running TUI)
		handler := newStandalonePromptHandler(cmd.OutOrStderr())
		configured, err := llmconfig.InteractiveSetup(cmd.Context(), handler)
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
	Args: cobra.ExactArgs(1),
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

		// Load or create config
		cfg, err := llmconfig.Load()
		if err != nil {
			return err
		}
		if cfg == nil {
			cfg = &llmconfig.Config{
				DefaultProvider: provider,
				Providers:       make(map[string]llmconfig.Provider),
			}
		}

		// Add or update provider
		providerCfg := llmconfig.Provider{
			APIKey:  apiKey,
			Enabled: true,
		}

		// Set BaseURL for OpenRouter
		if provider == "openrouter" {
			providerCfg.BaseURL = "https://openrouter.ai/api/v1"
		}

		cfg.Providers[provider] = providerCfg

		// If this is the first provider, set it as default
		if cfg.DefaultProvider == "" {
			cfg.DefaultProvider = provider
		}

		// Set default model if not set
		if cfg.DefaultModel == "" {
			switch provider {
			case "openrouter":
				cfg.DefaultModel = "anthropic/claude-sonnet-4.5"
			case "anthropic":
				cfg.DefaultModel = "claude-sonnet-4.5"
			case "openai":
				cfg.DefaultModel = "gpt-4.1"
			case "google":
				cfg.DefaultModel = "gemini-2.5-flash"
			}
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

var llmShowConfigCmd = &cobra.Command{
	Use:   "show-config",
	Short: "Show raw LLM configuration (JSON)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := llmconfig.Load()
		if err != nil {
			return err
		}

		if cfg == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "No LLM configuration found.")
			return nil
		}

		// Pretty-print the entire config as JSON
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return nil
	},
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

// standalonePromptHandler is a simple prompt handler that doesn't require
// a running TUI session (unlike frontendPretty which needs bubbletea running)
type standalonePromptHandler struct {
	output io.Writer
}

func newStandalonePromptHandler(w io.Writer) *standalonePromptHandler {
	return &standalonePromptHandler{output: w}
}

func (h *standalonePromptHandler) HandlePrompt(ctx context.Context, _, prompt string, dest any) error {
	return interact.NewInteraction(prompt).Resolve(dest)
}

func (h *standalonePromptHandler) HandleForm(ctx context.Context, form *huh.Form) error {
	return form.RunWithContext(ctx)
}
