package main

import (
	"fmt"
	"net/http"
)

const readinessEndpointPath = "/readyz"

// livenessEndpointPath answers 200 while the process serves HTTP, during
// initialization too (DCS-NFR-PER-03). The liveness probe uses it, so
// Kubernetes restarts a process that stops serving.
const livenessEndpointPath = "/livez"

func readinessResponse(status int, message string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, _ = fmt.Fprintln(w, message)
	}
}

func bootstrapHTTPHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(livenessEndpointPath, readinessResponse(http.StatusOK, "alive"))
	mux.Handle("/", readinessResponse(
		http.StatusServiceUnavailable,
		"starting up: initialization has not completed",
	))
	return mux
}

func mountReadinessEndpoint(mux *http.ServeMux) {
	mux.Handle(readinessEndpointPath, readinessResponse(http.StatusOK, "ready"))
	mux.Handle(livenessEndpointPath, readinessResponse(http.StatusOK, "alive"))
}
