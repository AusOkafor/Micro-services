package webhook

import "strings"

// NormalizeTopic converts Shopify topic strings (often like "orders/paid") into a stable internal form.
// Examples:
// - "orders/paid" -> "orders_paid"
// - "app/uninstalled" -> "app_uninstalled"
func NormalizeTopic(topic string) string {
	t := strings.TrimSpace(strings.ToLower(topic))
	t = strings.ReplaceAll(t, "/", "_")
	t = strings.ReplaceAll(t, ".", "_")
	t = strings.ReplaceAll(t, "-", "_")
	for strings.Contains(t, "__") {
		t = strings.ReplaceAll(t, "__", "_")
	}
	return strings.Trim(t, "_")
}


