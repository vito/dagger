# LLM Authentication Flow - Implementation Summary

## What We Solved

### The Missing Pieces

1. **Engine-Client Communication**: Engine runs in a container and can't directly access `~/.config/dagger/llm/config.json` on the client machine.
   - **Solution**: Use existing `file://` secret provider to read config file from client and send to engine

2. **Bootstrap Problem**: Original plan had circular dependency where `dagger llm setup` required LLM system initialization.
   - **Solution**: CLI commands operate directly on config file, no engine initialization needed

3. **Auto-Setup Flow**: No clear path for first-time users between error message and actual setup.
   - **Solution**: Hybrid approach - auto-prompt on first `dagger llm` run, with option to decline and use manual `dagger llm setup`

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        CLIENT SIDE                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ~/.config/dagger/llm/config.json (0600 permissions)       │
│  ↓                                                          │
│  CLI Commands (dagger llm setup/config/add-key/...)        │
│  - Direct file operations                                  │
│  - No engine initialization required                       │
│  - Interactive TUI with charmbracelet/huh                  │
│                                                             │
│  When starting LLM session:                                │
│  ↓                                                          │
│  1. Check if config exists                                 │
│  2. Check if env vars exist (fallback)                     │
│  3. If neither: AutoSetupIfNeeded() → InteractiveSetup()  │
│  4. Start session                                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                          ↓
                   file:// provider
                          ↓
┌─────────────────────────────────────────────────────────────┐
│                        ENGINE SIDE                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  NewLLMRouter():                                           │
│  1. Load config via: file://~/.config/dagger/llm/config.json│
│  2. Parse JSON → populate LLMRouter (base layer)           │
│  3. Load .env file (middle layer)                          │
│  4. Load env vars (top layer, overrides everything)        │
│  5. If empty → return ErrNoLLMConfig                       │
│                                                             │
│  Priority: config.json < .env < env vars                   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Key Features

### For New Users
```bash
$ dagger llm
# → Auto-prompt: "No LLM configuration found. Would you like to configure it now?"
# → Interactive setup with TUI
# → Choose provider (OpenRouter recommended)
# → Enter API key
# → Start using LLM immediately
```

### For Existing Users (Env Vars)
```bash
$ export ANTHROPIC_API_KEY=sk-ant-...
$ dagger llm
# → Works exactly as before, no changes needed
```

### For Advanced Users
```bash
$ dagger llm setup                    # Initial setup
$ dagger llm config                   # View current config
$ dagger llm add-key openrouter      # Add OpenRouter
$ dagger llm add-key anthropic       # Add direct Anthropic
$ dagger llm set-default openrouter  # Set default
$ dagger llm reset                   # Clear all config
```

## Configuration Schema

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
      "enabled": false
    }
  }
}
```

## Priority System

1. **Environment variables** (highest) - Always override everything
2. **`.env` file** (middle) - Local overrides
3. **Config file** (base) - Persistent user settings

This allows:
- Local development: use config file
- CI/CD: use env vars
- Mixed: config file + env var overrides

## Security

- Config file: **0600** permissions (owner read/write only)
- Keys stored in plaintext (standard for CLI tools)
- File:// provider already handles security
- Optional: use `libsecret://` for OS keychain integration

## Backward Compatibility

✅ **Zero Breaking Changes**

- Existing env var workflows: unchanged
- CI/CD pipelines: unchanged
- `.env` files: still supported
- Secret providers (`env://`, `vault://`, etc.): still work
- New config system is purely additive

## Implementation Phases

See [LLM_AUTH_FLOW_PLAN.md](LLM_AUTH_FLOW_PLAN.md) for detailed implementation phases.

**Estimated Timeline:** 3-4 weeks

**Core Work:**
1. Week 1: Config storage + CLI commands
2. Week 2: Engine integration + LLM session flow
3. Week 3: OpenRouter enhancements + backward compat testing
4. Week 4: Polish + documentation + rollout

## Success Criteria

- ✅ Setup time < 2 minutes for new users
- ✅ Zero friction for existing users (env vars still work)
- ✅ Single API key via OpenRouter (recommended default)
- ✅ Clear error messages and helpful prompts
- ✅ Works on Linux, macOS, Windows
- ✅ CI/CD workflows unchanged
