package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"digital-contracting-service/internal/pathutil"

	goahttp "goa.design/goa/v3/http"
)

func mountFrontend(mux goahttp.Muxer) {
	const staticDir = "/app/web/dist"

	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		return
	}

	apiPathPrefix := pathutil.NormalizePath(os.Getenv("DCS_API_PATH"), "", false)
	uiBasePath := pathutil.NormalizePath(os.Getenv("DCS_UI_PATH"), "/ui/", true)
	apiPrefixPath := strings.TrimSuffix(apiPathPrefix, "/")
	if apiPrefixPath == "" {
		apiPrefixPath = "/"
	}

	apiRoot := apiPathPrefix
	if apiRoot == "" {
		apiRoot = "/"
	}

	if uiBasePath != apiRoot {
		if apiPathPrefix == "" {
			mux.Handle("GET", "/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, uiBasePath, http.StatusMovedPermanently)
			})
		} else {
			mux.Handle("GET", apiPrefixPath, func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, uiBasePath, http.StatusMovedPermanently)
			})
			mux.Handle("GET", apiPrefixPath+"/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, uiBasePath, http.StatusMovedPermanently)
			})
		}
	}

	uiPrefix := strings.TrimSuffix(uiBasePath, "/")
	if uiPrefix == "" {
		uiPrefix = "/"
	}

	if uiPrefix != "/" {
		mux.Handle("GET", uiPrefix, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, uiBasePath, http.StatusMovedPermanently)
		})
	}

	pattern := uiPrefix + "/*"
	if uiPrefix == "/" {
		pattern = "/*"
	}

	mux.Handle("GET", pattern, func(w http.ResponseWriter, r *http.Request) {
		serveFrontend(w, r, staticDir, uiPrefix)
	})

	if uiPrefix == "/" {
		mux.Handle("GET", "/", func(w http.ResponseWriter, r *http.Request) {
			serveFrontend(w, r, staticDir, uiPrefix)
		})
	}
}

func serveFrontend(w http.ResponseWriter, r *http.Request, staticDir, uiBasePath string) {
	path := r.URL.Path
	if uiBasePath != "/" {
		path = strings.TrimPrefix(path, uiBasePath)
	}
	if path == "" {
		path = "/"
	}

	path = filepath.Clean(path)
	path = strings.TrimPrefix(path, "/")
	fullPath := filepath.Join(staticDir, path)

	absStaticDir, err := filepath.Abs(staticDir)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	relPath, err := filepath.Rel(absStaticDir, absFullPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		http.NotFound(w, r)
		return
	}

	if info, err := os.Stat(absFullPath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, absFullPath)
		return
	}

	indexPath := filepath.Join(absStaticDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	http.NotFound(w, r)
}

func normalizeBasePath(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = fallback
	}
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	if trimmed != "/" && !strings.HasSuffix(trimmed, "/") {
		trimmed += "/"
	}
	if trimmed == "/" {
		return "/"
	}
	return trimmed
}
