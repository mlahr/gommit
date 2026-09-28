package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Provider        string `toml:"provider"`
	Model           string `toml:"model"`
	BaseURL         string `toml:"base_url"`
	Style           string `toml:"style"`
	PerFileLimit    int    `toml:"per_file_limit"`
	MaxPromptChars  int    `toml:"max_prompt_chars"`
	CleanOutput     bool   `toml:"clean_output"`
	Timeout         int    `toml:"timeout"`
	OpenRouterRef   string `toml:"openrouter_referer"`
	OpenRouterTitle string `toml:"openrouter_title"`
	APIKeyEnv       string `toml:"api_key_env"`
}

func DefaultConfig() Config {
	return Config{
		Provider:        "openai",
		Model:           "",
		BaseURL:         "",
		Style:           "conventional",
		PerFileLimit:    20000,
		MaxPromptChars:  0,
		CleanOutput:     false,
		Timeout:         120,
		OpenRouterRef:   "",
		OpenRouterTitle: "",
		APIKeyEnv:       "",
	}
}

func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gommit", "config.toml"), nil
}

func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	if path == "" {
		return cfg, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if info.IsDir() {
		return cfg, fmt.Errorf("config path is a directory: %s", path)
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func ApplyEnvOverrides(cfg *Config) {
	setStringEnv(&cfg.Provider, "GOMMIT_PROVIDER")
	setStringEnv(&cfg.Model, "GOMMIT_MODEL")
	setStringEnv(&cfg.BaseURL, "GOMMIT_BASE_URL")
	setStringEnv(&cfg.Style, "GOMMIT_STYLE")
	setIntEnv(&cfg.PerFileLimit, "GOMMIT_PER_FILE_LIMIT")
	setIntEnv(&cfg.MaxPromptChars, "GOMMIT_MAX_PROMPT_CHARS")
	setBoolEnv(&cfg.CleanOutput, "GOMMIT_CLEAN_OUTPUT")
	setIntEnv(&cfg.Timeout, "GOMMIT_TIMEOUT")
	setStringEnv(&cfg.OpenRouterRef, "GOMMIT_OPENROUTER_REFERER")
	setStringEnv(&cfg.OpenRouterTitle, "GOMMIT_OPENROUTER_TITLE")
	setStringEnv(&cfg.OpenRouterRef, "OPENROUTER_REFERER")
	setStringEnv(&cfg.OpenRouterTitle, "OPENROUTER_TITLE")
	setStringEnv(&cfg.APIKeyEnv, "GOMMIT_API_KEY_ENV")
}

func setStringEnv(target *string, key string) {
	val := strings.TrimSpace(os.Getenv(key))
	if val != "" {
		*target = val
	}
}

func setIntEnv(target *int, key string) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return
	}
	*target = parsed
}

func setBoolEnv(target *bool, key string) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return
	}
	*target = strings.EqualFold(val, "true") || val == "1"
}

func ResolveAPIKey(provider, keyEnv string) (string, error) {
	if name := strings.TrimSpace(keyEnv); name != "" {
		val := strings.TrimSpace(os.Getenv(name))
		if val == "" {
			return "", fmt.Errorf("missing API key in env var %q (provider %q)", name, provider)
		}
		return val, nil
	}
	keys := []string{"GOMMIT_API_KEY"}
	switch strings.ToLower(provider) {
	case "openai":
		keys = append([]string{"OPENAI_API_KEY"}, keys...)
	case "openrouter":
		keys = append([]string{"OPENROUTER_API_KEY"}, keys...)
	case "anthropic":
		keys = append([]string{"ANTHROPIC_API_KEY"}, keys...)
	}
	for _, key := range keys {
		val := strings.TrimSpace(os.Getenv(key))
		if val != "" {
			return val, nil
		}
	}
	return "", fmt.Errorf("missing API key for provider %q", provider)
}

func DefaultBaseURL(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "https://api.openai.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	case "anthropic":
		return ""
	default:
		return ""
	}
}
