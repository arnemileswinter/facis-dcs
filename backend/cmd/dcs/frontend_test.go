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
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/ui/", "/ui/contracts/42", "/ui/../etc/passwd"} {
		rec := httptest.NewRecorder()
		serveFrontend(rec, httptest.NewRequest(http.MethodGet, path, nil), dir, "/ui")

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
