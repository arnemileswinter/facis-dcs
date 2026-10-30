package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeFrontendSetsFramingAndCSPHeaders(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{"index.html": "<html></html>", "app.js": "1"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	noIndex := t.TempDir()

	for _, tc := range []struct{ dir, path string }{
		{dir, "/ui/"},                 // index.html
		{dir, "/ui/contracts/42"},     // client route, index.html fallback
		{dir, "/ui/app.js"},           // asset
		{noIndex, "/ui/contracts/42"}, // 404
	} {
		path := tc.path
		rec := httptest.NewRecorder()
		serveFrontend(rec, httptest.NewRequest(http.MethodGet, path, nil), tc.dir, "/ui")

		if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options = %q, want DENY", path, got)
		}
		csp := rec.Header().Get("Content-Security-Policy")
		for _, directive := range []string{"frame-ancestors 'none'", "script-src 'self'", "object-src 'none'"} {
			if !strings.Contains(csp, directive) {
				t.Errorf("%s: Content-Security-Policy %q lacks %q", path, csp, directive)
			}
		}
	}
}
