package docs

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type Config struct {
	Title       string
	DocumentURL string
	CDNVersion  string
	CSSURL      string
	BundleURL   string
}
type swaggerHandler struct {
	html []byte
	csp  string
}

const page = `<!doctype html><html><head><meta charset="utf-8"><title>{{.Title}}</title><link rel="stylesheet" href="{{.CSSURL}}"></head><body><div id="swagger-ui"></div><script src="{{.BundleURL}}"></script><script>window.onload=function(){SwaggerUIBundle({url: {{.DocumentURL}},dom_id:'#swagger-ui'});};</script></body></html>`

func NewSwaggerHandler(cfg Config) (http.Handler, error) {
	if cfg.Title == "" {
		cfg.Title = "API documentation"
	}
	if cfg.DocumentURL == "" {
		cfg.DocumentURL = "/openapi.json"
	}
	if !strings.HasPrefix(cfg.DocumentURL, "/") {
		return nil, fmt.Errorf("document URL must be relative to the current origin")
	}
	if cfg.CDNVersion == "" {
		cfg.CDNVersion = "5.32.11"
	}
	if cfg.CSSURL == "" {
		cfg.CSSURL = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@" + cfg.CDNVersion + "/swagger-ui.css"
	}
	if cfg.BundleURL == "" {
		cfg.BundleURL = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@" + cfg.CDNVersion + "/swagger-ui-bundle.js"
	}
	tmpl, err := template.New("swagger").Parse(page)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, cfg); err != nil {
		return nil, err
	}
	origins := map[string]struct{}{}
	for _, raw := range []string{cfg.CSSURL, cfg.BundleURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("invalid asset URL %q", raw)
		}
		origins[u.Scheme+"://"+u.Host] = struct{}{}
	}
	list := make([]string, 0, len(origins))
	for origin := range origins {
		list = append(list, origin)
	}
	sort.Strings(list)
	csp := "default-src 'none'; style-src 'self' " + strings.Join(list, " ") + "; script-src 'self' 'unsafe-inline' " + strings.Join(list, " ") + "; img-src 'self' data:; connect-src 'self'; font-src 'self' " + strings.Join(list, " ")
	return swaggerHandler{html: []byte(b.String()), csp: csp}, nil
}
func (h swaggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(405), 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", h.csp)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(200)
	if r.Method != http.MethodHead {
		_, _ = w.Write(h.html)
	}
}
