package request

import (
	"crypto/elliptic"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"digital-contracting-service/internal/base/hsm"
	"digital-contracting-service/internal/base/identity"
)

// X5CSigner signs OpenID4VP Document-Retrieval request objects (JAR) with the
// DCS's own DID/hostname certificate chain in the header (x5c) rather than a
// bare jwk. client_id_scheme=x509_san_dns requires this: a real wallet
// resolves trust from the leaf certificate's SAN — which must equal
// client_id — not from an out-of-band key lookup keyed by kid (docretrieval.go).
// It reuses the same HSM-backed key and eIDAS-shaped certificate chain
// already published at /.well-known/did.json and used for DCS-to-DCS JAdES
// sync (jades.Sign) — the DCS attesting as itself, never as a contracting party.
type X5CSigner struct {
	did *identity.DIDDocument
}

// NewX5CSigner builds an x509_san_dns JAR signer over the DCS's own DID
// document identity.
func NewX5CSigner(did *identity.DIDDocument) (*X5CSigner, error) {
	if did == nil {
		return nil, fmt.Errorf("did document is required for x5c JAR signing")
	}
	if len(did.VerificationMethod) == 0 {
		return nil, fmt.Errorf("did document has no verification method")
	}
	if len(did.VerificationMethod[0].PublicKeyJWK.X5C) == 0 {
		return nil, fmt.Errorf("did document carries no x5c certificate chain")
	}
	return &X5CSigner{did: did}, nil
}

// ClientID returns the DNS hostname the signer's own certificate identifies
// (VerifyEIDASCertificate already asserts the leaf matches it) — the
// client_id an x509_san_dns request object must declare.
func (s *X5CSigner) ClientID() (string, error) {
	if s == nil || s.did == nil {
		return "", fmt.Errorf("x5c signer is not configured")
	}
	return s.did.GetHostname()
}

// SignAuthorizationRequestJWT returns a compact oauth-authz-req+jwt signed by
// the DID document's HSM key, with the x5c certificate chain embedded in the
// header instead of a bare jwk.
func (s *X5CSigner) SignAuthorizationRequestJWT(claims jwt.MapClaims) (string, error) {
	if s == nil || s.did == nil {
		return "", fmt.Errorf("x5c signer is not configured")
	}
	kid := s.did.VerificationMethod[0].ID
	x5c := []string(s.did.VerificationMethod[0].PublicKeyJWK.X5C)
	extraHeaders := map[string]any{"x5c": x5c}

	return signES256JWT(kid, claims, extraHeaders, func(signingInput string) ([]byte, error) {
		der, err := s.did.Sign([]byte(signingInput))
		if err != nil {
			return nil, fmt.Errorf("x5c jar signing failed: %w", err)
		}
		return hsm.ECDSADERToRaw(der, elliptic.P256())
	})
}

// X509SANDNSClientPrefix is the OpenID4VP client identifier prefix for a
// verifier that proves itself with an X.509 certificate whose SAN carries the
// DNS name it claims.
const X509SANDNSClientPrefix = "x509_san_dns"

// X509SANDNSClientID renders the client identifier a wallet is given. The
// prefix is part of the identifier, not a separate parameter: a bare value is
// read as the "pre-registered" prefix, which means "you already know me out of
// band" and is refused by any wallet that has no such prior arrangement.
func X509SANDNSClientID(hostname string) string {
	host := strings.TrimSpace(hostname)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, X509SANDNSClientPrefix+":") {
		return host
	}
	return X509SANDNSClientPrefix + ":" + host
}
