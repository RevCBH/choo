package provider

import "fmt"

// FromConfig creates a Provider from the given configuration.
// If cfg.Type is empty, defaults to Claude for backward compatibility.
// Returns an error for unknown provider types.
func FromConfig(cfg Config) (Provider, error) {
	switch cfg.Type {
	case ProviderClaude, "":
		// Empty type defaults to Claude for backward compatibility
		return NewClaude(cfg.Command), nil
	case ProviderCodex:
		return NewCodex(cfg.Command), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
	}
}
