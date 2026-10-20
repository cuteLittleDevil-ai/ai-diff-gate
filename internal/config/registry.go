package config

import (
	"fmt"
	"net/http"

	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/adapter"
	"github.com/cuteLittleDevil-ai/ai-diff-gate/internal/provider"
)

// BuildProviders constructs Provider instances from configuration.
func BuildProviders(cfg *Config, client *http.Client) ([]provider.Provider, error) {
	if client == nil {
		client = http.DefaultClient
	}
	var providers []provider.Provider
	for _, m := range cfg.EnabledModels() {
		p, err := buildOne(m, client)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, nil
}

func buildOne(m ModelConfig, client *http.Client) (provider.Provider, error) {
	spec := m.ToModel()
	switch m.Provider {
	case "openai":
		return adapter.NewOpenAI(spec, client), nil
	case "anthropic":
		return adapter.NewAnthropic(spec, client), nil
	default:
		return nil, fmt.Errorf("unsupported provider %q for model %q", m.Provider, m.ID)
	}
}

// ModelByID looks up a model config by id (enabled or not).
func (c *Config) ModelByID(id string) (ModelConfig, bool) {
	for _, m := range c.Models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelConfig{}, false
}
