package oashttp

import (
	"strings"
	"testing"
)

func TestParseUUID(t *testing.T) {
	for _, value := range []string{"550e8400-e29b-41d4-a716-446655440000", "550E8400-E29B-41D4-A716-446655440000"} {
		got, err := ParseUUID(value)
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != strings.ToLower(value) {
			t.Fatalf("String()=%q", got.String())
		}
	}
	for _, value := range []string{"", "abc", "550e8400e29b41d4a716446655440000", "550e8400-e29b-41d4-z716-446655440000"} {
		if _, err := ParseUUID(value); err == nil {
			t.Fatalf("ParseUUID(%q) succeeded", value)
		}
	}
}
