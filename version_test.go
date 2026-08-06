package oashttp

import "testing"

func TestStableVersion(t *testing.T) {
	if Version != "2.0.0" {
		t.Fatalf("Version=%q", Version)
	}
}
