package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	genauth "digital-contracting-service/gen/auth"
	"digital-contracting-service/internal/service"

	"goa.design/clue/log"
	goahttp "goa.design/goa/v3/http"
)

// mountPresentationProxyEndpoint forwards wallet direct_post submissions to
// CV at /v1/tenants/{tenant}/presentation/proof/{id}, while extracting the
// OID4VP `state` from the form body so the local presentation state can be
// marked completed for the frontend's polling loop.
//
// CV's response_uri (configured via publicBasePath) lands the wallet on
// `/api/presentation/proof/{id}`. The Vite dev proxy and the production
// gateway route that path here so we can observe completion locally.
func mountPresentationProxyEndpoint(ctx context.Context, mux goahttp.Muxer, authSvc genauth.Service) {
	cvURL, tenant, uiBasePath := service.PresentationConfig(authSvc)

	mux.Handle("POST", "/presentation/proof/{id}", func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		id := strings.TrimSpace(params["id"])
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		if cvURL == "" {
			http.Error(w, "CREDENTIAL_VERIFICATION_URL not configured", http.StatusInternalServerError)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		state := ""
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			// Parse without consuming r.Body (already read).
			if err := r.ParseForm(); err == nil {
				state = strings.TrimSpace(r.PostForm.Get("state"))
			}
			if state == "" {
				// Fallback: parse from the captured body.
				r2 := http.Request{Body: io.NopCloser(bytes.NewReader(body)), Header: r.Header}
				if err := r2.ParseForm(); err == nil {
					state = strings.TrimSpace(r2.PostForm.Get("state"))
				}
			}
		}

		// Forward to CV.
		target := fmt.Sprintf("%s/v1/tenants/%s/presentation/proof/%s", cvURL, tenant, id)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to build cv request: %v", err), http.StatusInternalServerError)
			return
		}
		for k, vs := range r.Header {
			lk := strings.ToLower(k)
			if lk == "host" || lk == "content-length" {
				continue
			}
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("cv proxy failed: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

		if resp.StatusCode >= 200 && resp.StatusCode < 300 && state != "" {
			location := uiBasePath + "auth/success"
			if !service.MarkPresentationCompleted(authSvc, state, location) {
				log.Printf(r.Context(), "presentation-proxy: failed to mark state %q completed", state)
			} else {
				log.Printf(r.Context(), "presentation-proxy: marked state %q completed", state)
			}
		}

		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
	})

	log.Printf(ctx, "HTTP %q mounted on %s %s", "PresentationProxy", "POST", "/presentation/proof/{id}")
}
