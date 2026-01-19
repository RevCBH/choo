package escalate

import "fmt"

// Config holds escalation configuration
type Config struct {
	Backends     []string
	SlackWebhook string
	WebhookURL   string
}

// FromConfig creates an Escalator from configuration
func FromConfig(cfg Config) (Escalator, error) {
	var escalators []Escalator

	for _, backend := range cfg.Backends {
		switch backend {
		case "terminal":
			escalators = append(escalators, NewTerminal())
		case "slack":
			if cfg.SlackWebhook == "" {
				return nil, fmt.Errorf("slack backend requires webhook URL")
			}
			escalators = append(escalators, NewSlack(cfg.SlackWebhook))
		case "webhook":
			if cfg.WebhookURL == "" {
				return nil, fmt.Errorf("webhook backend requires URL")
			}
			escalators = append(escalators, NewWebhook(cfg.WebhookURL))
		default:
			return nil, fmt.Errorf("unknown escalation backend: %s", backend)
		}
	}

	if len(escalators) == 0 {
		return NewTerminal(), nil
	}

	if len(escalators) == 1 {
		return escalators[0], nil
	}

	return NewMulti(escalators...), nil
}
