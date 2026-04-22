package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"digital-contracting-service/internal/base/datatype/userrole"
	"digital-contracting-service/internal/ocmw"

	"github.com/jmoiron/sqlx"

	"goa.design/clue/log"
	goahttp "goa.design/goa/v3/http"
)

type bootstrapOfferRequest struct {
	HolderDID string `json:"holder_did"`
	Role      string `json:"role"`
}

type bootstrapOfferResponse struct {
	OfferURI string                 `json:"offer_uri"`
	Offer    map[string]interface{} `json:"offer,omitempty"`
}

func mountBootstrapEndpoint(ctx context.Context, mux goahttp.Muxer, issuanceClient *ocmw.IssuanceClient, db *sqlx.DB) {
	mux.Handle("GET", "/bootstrap/status", func(w http.ResponseWriter, r *http.Request) {
		exists, err := adminExists(r.Context(), db)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to check admin bootstrap state: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"claimed": exists})
	})

	mux.Handle("POST", "/bootstrap/admin-offer", func(w http.ResponseWriter, r *http.Request) {
		// Only allow if no admin exists
		exists, err := adminExists(r.Context(), db)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to check admin bootstrap state: %v", err), http.StatusInternalServerError)
			return
		}
		if exists {
			http.Error(w, "admin already registered", http.StatusConflict)
			return
		}
		var req bootstrapOfferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", http.StatusBadRequest)
			return
		}
		req.HolderDID = strings.TrimSpace(req.HolderDID)
		req.Role = strings.TrimSpace(req.Role)
		if req.HolderDID == "" {
			http.Error(w, "holder_did is required", http.StatusBadRequest)
			return
		}
		// The bootstrap admin holds every defined role so the first user can
		// fully operate the system. The optional `role` field is preserved
		// for backwards compatibility with callers that want a single role.
		var roleClaim interface{}
		if req.Role != "" {
			roleClaim = req.Role
		} else {
			all := userrole.All()
			roles := make([]string, 0, len(all))
			for _, r := range all {
				roles = append(roles, r.String())
			}
			roleClaim = roles
		}
		offer, err := issuanceClient.CreateOffer(r.Context(), ocmw.CredentialOfferRequest{
			HolderDID:      req.HolderDID,
			CredentialType: "DCSRoleCredential",
			Claims: map[string]interface{}{
				"id":   req.HolderDID,
				"role": roleClaim,
			},
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to create credential offer: %v", err), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(bootstrapOfferResponse{
			OfferURI: offer.OfferURI,
			Offer:    offer.Raw,
		})
	})

	mux.Handle("POST", "/bootstrap/admin-claimed", func(w http.ResponseWriter, r *http.Request) {
		var req bootstrapOfferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON payload", http.StatusBadRequest)
			return
		}

		req.HolderDID = strings.TrimSpace(req.HolderDID)
		if req.HolderDID == "" {
			http.Error(w, "holder_did is required", http.StatusBadRequest)
			return
		}

		if err := markAdminClaimed(r.Context(), db, req.HolderDID); err != nil {
			http.Error(w, fmt.Sprintf("failed to persist admin bootstrap state: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
	log.Printf(ctx, "HTTP %q mounted on %s %s", "BootstrapAdminOffer", "POST", "/bootstrap/admin-offer")
	log.Printf(ctx, "HTTP %q mounted on %s %s", "BootstrapAdminClaimed", "POST", "/bootstrap/admin-claimed")
	log.Printf(ctx, "HTTP %q mounted on %s %s", "BootstrapStatus", "GET", "/bootstrap/status")
}

func ensureBootstrapStateTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS admin_bootstrap_state (
	id SMALLINT PRIMARY KEY CHECK (id = 1),
	holder_did TEXT NOT NULL,
	claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	return err
}

func adminExists(ctx context.Context, db *sqlx.DB) (bool, error) {
	if err := ensureBootstrapStateTable(ctx, db); err != nil {
		return false, err
	}

	var exists bool
	err := db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM admin_bootstrap_state WHERE id = 1)`)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func markAdminClaimed(ctx context.Context, db *sqlx.DB, holderDID string) error {
	if err := ensureBootstrapStateTable(ctx, db); err != nil {
		return err
	}

	_, err := db.ExecContext(ctx, `
INSERT INTO admin_bootstrap_state (id, holder_did, claimed_at, updated_at)
VALUES (1, $1, NOW(), NOW())
ON CONFLICT (id)
DO UPDATE SET holder_did = EXCLUDED.holder_did, updated_at = NOW()`, holderDID)
	return err
}
