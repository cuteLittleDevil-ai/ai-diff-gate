package modelspec

import (
	"fmt"
	"os"
)

// Model holds per-model endpoint configuration.
type Model struct {
	ID          string
	Provider    string
	DisplayName string
	Model       string
	BaseURL     string
	APIKeyEnv   string
	APIKey      string
	Enabled     bool
}

// ResolveAPIKey returns the API key (env first, then inline).
func (m Model) ResolveAPIKey() (string, error) {
	if m.APIKeyEnv != "" {
		if v := os.Getenv(m.APIKeyEnv); v != "" {
			return v, nil
		}
	}
	if m.APIKey != "" {
		return m.APIKey, nil
	}
	if m.APIKeyEnv != "" {
		return "", fmt.Errorf("environment variable %s is not set", m.APIKeyEnv)
	}
	return "", fmt.Errorf("no API key configured for model %q", m.ID)
}
