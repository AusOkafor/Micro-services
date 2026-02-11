package milestone

import (
	"github.com/shopspring/decimal"
)

// CalculatedMilestone is an instance amount computed from a template.
// Sequence is assigned by the caller (deposit is expected to be sequence 0).
type CalculatedMilestone struct {
	Amount decimal.Decimal
	IsFinal bool
}

type CurrencyScale int32

const DefaultCurrencyScale CurrencyScale = 2

// CalculateAmounts computes milestone amounts from templates.
//
// Rules:
// - The sum of calculated milestone amounts must equal total (validated).
// - Percentages are applied against total (not remaining) for determinism.
// - Rounding is applied to the configured scale; any rounding delta is applied to the final milestone to force sum equality.
func CalculateAmounts(total decimal.Decimal, templates []MilestoneTemplate, scale CurrencyScale) ([]CalculatedMilestone, error) {
	if err := ValidateTemplate(templates); err != nil {
		return nil, err
	}
	if total.LessThanOrEqual(decimal.Zero) {
		return nil, ValidationError{Code: "SERVICE_TOTAL_INVALID", Message: "service total must be > 0"}
	}

	if scale <= 0 {
		scale = DefaultCurrencyScale
	}

	out := make([]CalculatedMilestone, 0, len(templates))
	sum := decimal.Zero
	for _, t := range templates {
		var amt decimal.Decimal
		switch t.Type {
		case TemplateTypeFixed:
			amt = t.Value
		case TemplateTypePercentage:
			// Value is percentage like 30 for 30%.
			amt = total.Mul(t.Value).Div(decimal.NewFromInt(100))
		default:
			return nil, ValidationError{Code: "MILESTONE_TYPE_INVALID", Message: "milestone type must be fixed or percentage"}
		}
		amt = amt.Round(int32(scale))
		out = append(out, CalculatedMilestone{Amount: amt, IsFinal: t.IsFinal})
		sum = sum.Add(amt)
	}

	delta := total.Round(int32(scale)).Sub(sum)
	if !delta.IsZero() {
		// Apply rounding delta to the final milestone (must exist and be last by contract).
		last := len(out) - 1
		out[last] = CalculatedMilestone{
			Amount: out[last].Amount.Add(delta).Round(int32(scale)),
			IsFinal: true,
		}
		sum = sum.Add(delta).Round(int32(scale))
	}

	if !sum.Equal(total.Round(int32(scale))) {
		return nil, ValidationError{Code: "MILESTONE_SUM_MISMATCH", Message: "milestone amounts do not sum to service total"}
	}

	// Prevent zero/negative final milestone due to delta.
	if out[len(out)-1].Amount.LessThanOrEqual(decimal.Zero) {
		return nil, ValidationError{Code: "FINAL_MILESTONE_INVALID", Message: "final milestone amount must be > 0"}
	}

	return out, nil
}


