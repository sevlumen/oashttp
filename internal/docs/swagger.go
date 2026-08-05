package docs

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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

type pageData struct {
	Title       string
	CSSURL      string
	BundleURL   string
	Initializer template.JS
}

const page = `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><link rel="stylesheet" href="{{.CSSURL}}"></head><body><div id="swagger-ui"></div><script src="{{.BundleURL}}"></script><script>{{.Initializer}}</script></body></html>`

func NewSwaggerHandler(cfg Config) (http.Handler, error) {
	if cfg.Title == "" {
		cfg.Title = "API documentation"
	}
	if cfg.DocumentURL == "" {
		cfg.DocumentURL = "/openapi.json"
	}
	if err := validateDocumentURL(cfg.DocumentURL); err != nil {
		return nil, err
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

	documentJSON, err := json.Marshal(cfg.DocumentURL)
	if err != nil {
		return nil, err
	}
	initializer := "window.onload=function(){SwaggerUIBundle({url: " + string(documentJSON) + ",dom_id:'#swagger-ui'});};"

	tmpl, err := template.New("swagger").Parse(page)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, pageData{Title: cfg.Title, CSSURL: cfg.CSSURL, BundleURL: cfg.BundleURL, Initializer: template.JS(initializer)}); err != nil {
		return nil, err
	}

	origins := map[string]struct{}{}
	for _, raw := range []string{cfg.CSSURL, cfg.BundleURL} {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return nil, fmt.Errorf("invalid asset URL %q", raw)
		}
		origins[u.Scheme+"://"+u.Host] = struct{}{}
	}
	list := make([]string, 0, len(origins))
	for origin := range origins {
		list = append(list, origin)
	}
	sort.Strings(list)

	hash := sha256.Sum256([]byte(initializer))
	scriptHash := "'sha256-" + base64.StdEncoding.EncodeToString(hash[:]) + "'"
	csp := "default-src 'none'; style-src 'self' " + strings.Join(list, " ") + "; script-src 'self' " + scriptHash + " " + strings.Join(list, " ") + "; img-src 'self' data:; connect-src 'self'; font-src 'self' " + strings.Join(list, " ")
	return swaggerHandler{html: []byte(b.String()), csp: csp}, nil
}

func validateDocumentURL(raw string) error {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.Contains(raw, "\\") || strings.ContainsAny(raw, "\r\n\x00") {
		return fmt.Errorf("document URL must be a root-relative URL on the current origin")
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.IsAbs() || u.Host != "" {
		return fmt.Errorf("invalid document URL %q", raw)
	}
	return nil
}

func (h swaggerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", h.csp)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(h.html)
	}
}
