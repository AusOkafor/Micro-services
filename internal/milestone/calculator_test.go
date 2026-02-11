package milestone

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCalculateAmounts_PercentageWithRoundingDeltaAppliedToFinal(t *testing.T) {
	total := decimal.RequireFromString("100.00")
	templates := []MilestoneTemplate{
		{Type: TemplateTypePercentage, Value: decimal.NewFromInt(33), IsFinal: false}, // 33.00
		{Type: TemplateTypePercentage, Value: decimal.NewFromInt(33), IsFinal: false}, // 33.00
		{Type: TemplateTypePercentage, Value: decimal.NewFromInt(34), IsFinal: true},  // 34.00
	}

	got, err := CalculateAmounts(total, templates, DefaultCurrencyScale)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 milestones, got %d", len(got))
	}

	sum := decimal.Zero
	for _, m := range got {
		sum = sum.Add(m.Amount)
	}
	if !sum.Equal(total) {
		t.Fatalf("expected sum %s, got %s", total, sum)
	}
	if !got[2].IsFinal {
		t.Fatalf("expected last milestone final")
	}
}

func TestValidateTemplate_FinalMustBeLast(t *testing.T) {
	templates := []MilestoneTemplate{
		{Type: TemplateTypeFixed, Value: decimal.RequireFromString("10"), IsFinal: true},
		{Type: TemplateTypeFixed, Value: decimal.RequireFromString("90"), IsFinal: false},
	}
	if err := ValidateTemplate(templates); err == nil {
		t.Fatalf("expected error")
	}
}


