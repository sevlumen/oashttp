package oashttp

import (
	"context"
	"net/http"
	"testing"
)

type snapshotSecurityProvider struct {
	scheme SecurityScheme
}

func (p *snapshotSecurityProvider) SecurityScheme() SecurityScheme { return p.scheme }

func (p *snapshotSecurityProvider) Authenticate(context.Context, *http.Request) (*Principal, error) {
	return &Principal{}, nil
}

func TestNewSnapshotsServers(t *testing.T) {
	servers := []Server{{URL: "https://original.example", Description: "original"}}
	app := New(Config{Servers: servers})

	servers[0].URL = "https://changed.example"
	servers[0].Description = "changed"

	if got := app.config.Servers[0]; got.URL != "https://original.example" || got.Description != "original" {
		t.Fatalf("app server = %#v, want original snapshot", got)
	}
}

func TestNewSnapshotsSecurityProviders(t *testing.T) {
	providerA := &snapshotSecurityProvider{scheme: SecurityScheme{Type: "apiKey", Name: "X-API-Key", In: "header"}}
	providerB := &snapshotSecurityProvider{scheme: SecurityScheme{Type: "apiKey", Name: "X-Other-Key", In: "header"}}

	providers := map[string]SecurityProvider{"apiKey": providerA}
	app := New(Config{SecurityProviders: providers})

	providers["apiKey"] = providerB
	providers["late"] = providerB
	delete(providers, "apiKey")

	if got := app.config.SecurityProviders["apiKey"]; got != providerA {
		t.Fatalf("apiKey provider = %#v, want original provider", got)
	}
	if _, ok := app.config.SecurityProviders["late"]; ok {
		t.Fatal("provider added after New() leaked into app config")
	}
}

func TestNewPreservesNilConfigContainers(t *testing.T) {
	app := New(Config{})

	if app.config.Servers != nil {
		t.Fatalf("Servers = %#v, want nil", app.config.Servers)
	}
	if app.config.SecurityProviders != nil {
		t.Fatalf("SecurityProviders = %#v, want nil", app.config.SecurityProviders)
	}
}
