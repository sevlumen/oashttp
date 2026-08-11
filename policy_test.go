package oashttp

import (
	"os"
	"strings"
	"testing"
)

func TestSecurityPolicyDocumentsV2AndV1Support(t *testing.T) {
	data, err := os.ReadFile("SECURITY.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"| `v2.x` | Yes |", "| `v1.x` | Security fixes only |"} {
		if !strings.Contains(text, required) {
			t.Fatalf("SECURITY.md missing %q", required)
		}
	}
}

func TestSupportPolicyTargetsV2(t *testing.T) {
	data, err := os.ReadFile("SUPPORT.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{"current v2 module", "Within major version 2", "v1 module receives security fixes only", "`MapHandler`"} {
		if !strings.Contains(text, required) {
			t.Fatalf("SUPPORT.md missing %q", required)
		}
	}
}
