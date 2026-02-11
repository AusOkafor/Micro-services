package milestone

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type TemplateType string

const (
	TemplateTypeFixed      TemplateType = "fixed"
	TemplateTypePercentage TemplateType = "percentage"
)

type MilestoneTemplate struct {
	Type    TemplateType    `json:"type"`
	Value   decimal.Decimal `json:"value"`
	IsFinal bool            `json:"isFinal"`
}

type ValidationError struct {
	Code    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ValidateTemplate enforces the milestone contract:
// - Deposit is milestone[0] at the call site (sequence is external); this function validates the template list itself.
// - Exactly one final milestone, and it must be last.
// - All values must be > 0.
func ValidateTemplate(templates []MilestoneTemplate) error {
	if len(templates) == 0 {
		return ValidationError{Code: "MILESTONE_TEMPLATE_EMPTY", Message: "milestone template cannot be empty"}
	}

	finalIdx := -1
	for i, t := range templates {
		if t.Value.LessThanOrEqual(decimal.Zero) {
			return ValidationError{Code: "MILESTONE_VALUE_INVALID", Message: "milestone value must be > 0"}
		}
		switch t.Type {
		case TemplateTypeFixed, TemplateTypePercentage:
		default:
			return ValidationError{Code: "MILESTONE_TYPE_INVALID", Message: "milestone type must be fixed or percentage"}
		}
		if t.IsFinal {
			if finalIdx != -1 {
				return ValidationError{Code: "FINAL_MILESTONE_DUPLICATE", Message: "exactly one final milestone is required"}
			}
			finalIdx = i
		}
	}

	if finalIdx == -1 {
		return ValidationError{Code: "FINAL_MILESTONE_MISSING", Message: "final milestone is required"}
	}
	if finalIdx != len(templates)-1 {
		return ValidationError{Code: "FINAL_MILESTONE_NOT_LAST", Message: "final milestone must be last"}
	}

	return nil
}


