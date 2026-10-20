package config

import (
	"fmt"
	"os"
	"time"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/modelspec"
	"gopkg.in/yaml.v3"
)

// Config is the root application configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Fanout  FanoutConfig  `yaml:"fanout"`
	Models  []ModelConfig `yaml:"models"`
	Compare CompareConfig `yaml:"compare"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type FanoutConfig struct {
	TimeoutSeconds int `yaml:"timeout_seconds"`
}

type ModelConfig struct {
	ID          string `yaml:"id"`
	Provider    string `yaml:"provider"`
	DisplayName string `yaml:"display_name"`
	Model       string `yaml:"model"`
	BaseURL     string `yaml:"base_url"`
	APIKeyEnv   string `yaml:"api_key_env"`
	APIKey      string `yaml:"api_key"`
	Enabled     *bool  `yaml:"enabled"`
}

type CompareConfig struct {
	Granularity       string `yaml:"granularity"`
	IgnoreWhitespace  bool   `yaml:"ignore_whitespace"`
}

// Load reads and parses a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	applyDefaults(&cfg)
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = ":8080"
	}
	if cfg.Fanout.TimeoutSeconds <= 0 {
		cfg.Fanout.TimeoutSeconds = 60
	}
	if cfg.Compare.Granularity == "" {
		cfg.Compare.Granularity = "line"
	}
	for i := range cfg.Models {
		if cfg.Models[i].Enabled == nil {
			enabled := true
			cfg.Models[i].Enabled = &enabled
		}
	}
}

// FanoutTimeout returns the fan-out per-model timeout duration.
func (c *Config) FanoutTimeout() time.Duration {
	return time.Duration(c.Fanout.TimeoutSeconds) * time.Second
}

// EnabledModels returns model configs that are marked enabled.
func (c *Config) EnabledModels() []ModelConfig {
	var out []ModelConfig
	for _, m := range c.Models {
		if m.Enabled != nil && *m.Enabled {
			out = append(out, m)
		}
	}
	return out
}

// ToModel converts YAML model config to runtime spec.
func (m ModelConfig) ToModel() modelspec.Model {
	enabled := true
	if m.Enabled != nil {
		enabled = *m.Enabled
	}
	return modelspec.Model{
		ID:          m.ID,
		Provider:    m.Provider,
		DisplayName: m.DisplayName,
		Model:       m.Model,
		BaseURL:     m.BaseURL,
		APIKeyEnv:   m.APIKeyEnv,
		APIKey:      m.APIKey,
		Enabled:     enabled,
	}
}
