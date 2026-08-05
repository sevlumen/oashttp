package oashttp

import "testing"

func TestStableVersion(t *testing.T) {
	if Version != "1.0.1" {
		t.Fatalf("Version=%q", Version)
	}
}
