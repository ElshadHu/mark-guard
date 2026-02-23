// Package config will handle loading and parsing of .markguard.yaml (potentially)
package config

import (
	"errors"
	"fmt"
	"os"

	"go.yaml.in/yaml/v3"
)

// Config is the top level configuration
type Config struct {
	LLM  LLMConfig  `yaml:"llm"`
	Docs DocsConfig `yaml:"docs"`
}

// LLMConfig holds settings for LLM provider
type LLMConfig struct {
	BaseURL   string `yaml:"base_url"`
	APIKeyEnv string `yaml:"api_key_env"`
	Model     string `yaml:"model"`
}

// DocsConfig holds settings for the LLM provider
type DocsConfig struct {
	Paths   []string `yaml:"paths"`
	Exclude []string `yaml:"exclude"`
}

// defaults returns a Config with sensible default values
func defaults() Config {
	return Config{LLM: LLMConfig{
		BaseURL:   "http://api.openai.com/v1",
		APIKeyEnv: "OPENAI_API_KEY",
		Model:     "gpt-40",
	},
		Docs: DocsConfig{
			Paths:   []string{"docs/", "README.md"},
			Exclude: nil,
		},
	}
}

// Load reads the config file at path  and returns a Config
func Load(path string) (*Config, error) {
	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cfg, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)

	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}
