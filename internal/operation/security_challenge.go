package operation

import (
	"net/http"
	"strings"

	"github.com/sevlumen/oashttp/v2/internal/core"
	internalsecurity "github.com/sevlumen/oashttp/v2/internal/security"
)

func applyProviderChallenge(w http.ResponseWriter, provider core.SecurityProvider, failure *internalsecurity.Failure) {
	if provider == nil || failure == nil {
		return
	}
	scheme := provider.SecurityScheme()
	if scheme.Type != "http" || strings.TrimSpace(scheme.Scheme) == "" {
		return
	}

	httpScheme := strings.TrimSpace(scheme.Scheme)
	if strings.EqualFold(httpScheme, "bearer") {
		httpScheme = "Bearer"
	}

	switch failure.Status {
	case http.StatusUnauthorized:
		w.Header().Set("WWW-Authenticate", httpScheme)
	case http.StatusForbidden:
		if strings.EqualFold(httpScheme, "Bearer") {
			w.Header().Set("WWW-Authenticate", `Bearer error="insufficient_scope"`)
		}
	}
}
