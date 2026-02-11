package webhook

import "testing"

func TestParseKeyFromNote(t *testing.T) {
	note := "service_workflow: milestone_id=abc-123 service_id=def-456"
	if got := ParseKeyFromNote(note, "milestone_id"); got != "abc-123" {
		t.Fatalf("expected abc-123, got %q", got)
	}
	if got := ParseKeyFromNote(note, "service_id"); got != "def-456" {
		t.Fatalf("expected def-456, got %q", got)
	}
}

func TestParseKeyFromNote_ToleratesPunctuation(t *testing.T) {
	note := "hello,milestone_id=zzz;service_id=yyy other=xxx"
	if got := ParseKeyFromNote(note, "milestone_id"); got != "zzz" {
		t.Fatalf("expected zzz, got %q", got)
	}
}


