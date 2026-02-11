package webhook

import (
	"regexp"
	"strings"
)

var noteKVRe = regexp.MustCompile(`(?i)(?:^|[\s,;])([a-zA-Z0-9_]+)=([a-zA-Z0-9-]+)`)

// ParseKeyFromNote extracts a key=value token from a note string.
// It is intentionally tolerant because notes can include prefixes, punctuation, and user text.
//
// Example note:
//   "service_workflow: milestone_id=abc-123 service_id=def-456"
func ParseKeyFromNote(note string, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}

	matches := noteKVRe.FindAllStringSubmatch(note, -1)
	for _, m := range matches {
		if len(m) != 3 {
			continue
		}
		k := strings.ToLower(m[1])
		if k == strings.ToLower(key) {
			return m[2]
		}
	}
	return ""
}


