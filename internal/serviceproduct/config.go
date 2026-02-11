package serviceproduct

import (
	"encoding/json"

	"github.com/shopspring/decimal"

	"microservice/internal/milestone"
)

// Config is stored as JSONB in `service_product_configs.config`.
// Keep this versioned so we can evolve without breaking existing records.
type Config struct {
	Version   int                      `json:"version"`
	Currency  string                   `json:"currency,omitempty"`
	Templates []milestone.MilestoneTemplate `json:"templates"`

	// Optional: if you want to lock percent-only templates early, this can be used later.
	// For now we validate structural rules and (if all percent) require sum==100.
}

func ParseAndValidate(raw json.RawMessage) (Config, error) {
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Config{}, milestone.ValidationError{Code: "VALIDATION_FAILED", Message: "invalid config json"}
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}

	if err := milestone.ValidateTemplate(cfg.Templates); err != nil {
		return Config{}, err
	}

	// If all milestones are percentage-based, enforce sum == 100 exactly.
	allPercent := true
	sum := decimal.Zero
	for _, t := range cfg.Templates {
		if t.Type != milestone.TemplateTypePercentage {
			allPercent = false
			break
		}
		sum = sum.Add(t.Value)
	}
	if allPercent && !sum.Equal(decimal.NewFromInt(100)) {
		return Config{}, milestone.ValidationError{Code: "MILESTONE_SUM_INVALID", Message: "percentage milestones must sum to 100"}
	}

	return cfg, nil
}


