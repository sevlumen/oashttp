package core

import "testing"

func TestNormalizeUUID(t *testing.T) {
	got, err := NormalizeUUID("550E8400-E29B-41D4-A716-446655440000")
	if err != nil {
		t.Fatal(err)
	}
	if got != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("got=%q", got)
	}
}

func TestNormalizeUUIDRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "550e8400e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-44665544000z"} {
		if _, err := NormalizeUUID(value); err == nil {
			t.Fatalf("NormalizeUUID(%q) succeeded", value)
		}
	}
}
