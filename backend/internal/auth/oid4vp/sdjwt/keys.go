package sdjwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JWK is an EC P-256 public key used for SD-JWT verification.
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
	D   string `json:"d,omitempty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
}

type jwksDocument struct {
	Keys []JWK `json:"keys"`
}

// TrustConfig provides issuer trust queries used during JWT signature verification.
type TrustConfig interface {
	IssuerTrusted(iss string) bool
	VCTAllowed(vct string) bool
	IssuerJWKS(iss string) (json.RawMessage, error)
	// X5CTrustRoots returns the trust anchors an x5c-bearing credential's
	// certificate chain must verify against, or nil if none are configured —
	// in which case an x5c-bearing credential must be refused outright.
	X5CTrustRoots() *x509.CertPool
}

// --- Issuer credential JWT: verification key resolution ---

// ResolveIssuerVerificationKey returns the public key used to verify a credential issuer JWT.
//
// Trust and key material are resolved inside the JWT keyfunc so verification never proceeds
// with an untrusted or unknown issuer key. Resolution order:
//
//  1. header.jwk — embedded JWK matched against the issuer entry in trust configuration.
//  2. header.x5c — rejected until chain validation lands with the trust migration.
//  3. header.kid — lookup in the issuer JWKS bundled in trust configuration.
func ResolveIssuerVerificationKey(cfg TrustConfig, token *jwt.Token) (any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("trust config is not configured")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("credential jwt claims are invalid")
	}

	iss, _ := claims["iss"].(string)
	if strings.TrimSpace(iss) == "" {
		return nil, fmt.Errorf("credential jwt missing iss")
	}
	if !cfg.IssuerTrusted(iss) {
		return nil, fmt.Errorf("issuer %q is not trusted", iss)
	}

	jwksRaw, err := cfg.IssuerJWKS(iss)
	if err != nil {
		return nil, err
	}

	if rawJWK, ok := token.Header["jwk"]; ok {
		return verificationKeyFromHeaderJWK(jwksRaw, rawJWK)
	}

	if _, ok := token.Header["x5c"]; ok {
		return verificationKeyFromX5C(token.Header["x5c"], cfg.X5CTrustRoots())
	}

	return verificationKeyFromTrustedJWKS(jwksRaw, token)
}

// ResolveIssuerVerificationKeyForPID resolves the issuer key for PID credentials signed with x5c.
func ResolveIssuerVerificationKeyForPID(cfg TrustConfig, token *jwt.Token) (any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("trust config is not configured")
	}

	rawX5C, ok := token.Header["x5c"]
	if !ok {
		return nil, fmt.Errorf("pid credential jwt requires x5c")
	}

	return verificationKeyFromX5C(rawX5C, cfg.X5CTrustRoots())
}

// ResolveIssuerVerificationKeyForPoA resolves a PoA issuer key. Existing
// trust-registry/JWKS credentials remain supported, while an x5c-bearing PoA
// is accepted only as the v1 one-hop profile: issuer leaf followed directly
// by the configured self-signed CA root.
func ResolveIssuerVerificationKeyForPoA(cfg TrustConfig, token *jwt.Token) (any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("trust config is not configured")
	}
	if _, ok := token.Header["x5c"]; !ok {
		return ResolveIssuerVerificationKey(cfg, token)
	}

	certs, err := parseX5C(token.Header["x5c"])
	if err != nil {
		return nil, err
	}
	if len(certs) != 2 {
		return nil, fmt.Errorf("PoA x5c must contain exactly issuer leaf and root, got %d certificates", len(certs))
	}
	leaf, root := certs[0], certs[1]
	if leaf.Issuer.String() != root.Subject.String() || root.Issuer.String() != root.Subject.String() {
		return nil, fmt.Errorf("PoA x5c is not a direct issuer-leaf to self-signed root chain")
	}
	if !root.BasicConstraintsValid || !root.IsCA {
		return nil, fmt.Errorf("PoA x5c root is not a certificate authority")
	}
	if err := root.CheckSignatureFrom(root); err != nil {
		return nil, fmt.Errorf("PoA x5c root is not self-signed: %w", err)
	}
	return verificationKeyFromCertificates(certs, cfg.X5CTrustRoots())
}

// verificationKeyFromX5C parses the full x5c chain (leaf first, per RFC 7517
// §4.7), verifies leaf -> intermediates -> roots, and returns the leaf's
// public key. roots being nil (no trust anchors configured) is refused, not
// silently accepted off the chain's own say-so — an x5c header proves
// nothing about WHO the leaf belongs to without a trust anchor to verify
// against; trusting an unverified chain would let anyone mint their own
// key+cert and self-certify as any issuer.
func verificationKeyFromX5C(raw any, roots *x509.CertPool) (any, error) {
	if roots == nil {
		return nil, fmt.Errorf("no x5c trust anchors are configured")
	}

	certs, err := parseX5C(raw)
	if err != nil {
		return nil, err
	}
	return verificationKeyFromCertificates(certs, roots)
}

func verificationKeyFromCertificates(certs []*x509.Certificate, roots *x509.CertPool) (any, error) {
	if roots == nil {
		return nil, fmt.Errorf("no x5c trust anchors are configured")
	}
	leaf := certs[0]
	intermediates := x509.NewCertPool()
	for _, cert := range certs[1:] {
		intermediates.AddCert(cert)
	}

	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("x5c certificate chain does not verify against configured trust anchors: %w", err)
	}

	switch pk := leaf.PublicKey.(type) {
	case *ecdsa.PublicKey:
		if pk.Curve != elliptic.P256() {
			return nil, fmt.Errorf("x5c leaf certificate is not P-256")
		}
		return pk, nil
	default:
		return nil, fmt.Errorf("x5c leaf certificate public key is not ECDSA")
	}
}

func parseX5C(raw any) ([]*x509.Certificate, error) {
	certsRaw, ok := raw.([]any)
	if !ok || len(certsRaw) == 0 {
		return nil, fmt.Errorf("x5c header is empty")
	}
	certs := make([]*x509.Certificate, 0, len(certsRaw))
	for i, entry := range certsRaw {
		certB64, ok := entry.(string)
		if !ok || strings.TrimSpace(certB64) == "" {
			return nil, fmt.Errorf("x5c[%d] is invalid", i)
		}
		der, err := base64.StdEncoding.DecodeString(certB64)
		if err != nil {
			return nil, fmt.Errorf("decode x5c[%d]: %w", i, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("parse x5c[%d]: %w", i, err)
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func verificationKeyFromHeaderJWK(jwksRaw json.RawMessage, rawJWK any) (any, error) {
	headerKey, err := JWKFromAny(rawJWK)
	if err != nil {
		return nil, err
	}

	err = assertJWKTrusted(jwksRaw, headerKey)
	if err != nil {
		return nil, err
	}

	return ecPublicKey(headerKey.X, headerKey.Y)
}

func verificationKeyFromTrustedJWKS(jwksRaw json.RawMessage, token *jwt.Token) (any, error) {
	var doc jwksDocument
	err := json.Unmarshal(jwksRaw, &doc)

	if err != nil {
		return nil, fmt.Errorf("parse issuer jwks: %w", err)
	}

	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		// Without a kid the key choice must be unambiguous.
		if len(doc.Keys) != 1 {
			return nil, fmt.Errorf("credential jwt has no kid and issuer jwks has %d keys", len(doc.Keys))
		}
		return trustedECKey(doc.Keys[0])
	}

	for _, key := range doc.Keys {
		if key.Kid == kid {
			return trustedECKey(key)
		}
	}

	return nil, fmt.Errorf("no matching issuer jwk for kid %q", kid)
}

func trustedECKey(key JWK) (any, error) {
	if key.Kty != "EC" || key.Crv != "P-256" {
		return nil, fmt.Errorf("issuer jwk %q is not an EC P-256 key", key.Kid)
	}

	return ecPublicKey(key.X, key.Y)
}

func assertJWKTrusted(jwksRaw json.RawMessage, candidate JWK) error {
	var doc jwksDocument
	err := json.Unmarshal(jwksRaw, &doc)

	if err != nil {
		return fmt.Errorf("parse issuer jwks: %w", err)
	}

	for _, trusted := range doc.Keys {
		if publicJWKsEqual(candidate, trusted) {
			return nil
		}
	}

	return fmt.Errorf("credential issuer jwk is not trusted")
}

// --- Holder KB-JWT: verification key ---

func holderVerificationKey(cnfJWK JWK, token *jwt.Token) (any, error) {
	_ = token

	return ecPublicKey(cnfJWK.X, cnfJWK.Y)
}

// --- JWK primitives ---

// JWKFromAny parses a JWK from a JWT header or claim value.
func JWKFromAny(raw any) (JWK, error) {
	switch v := raw.(type) {
	case map[string]any:
		return ecP256PublicKeyFromMap(v)
	case JWK:
		return ecP256PublicKeyFromJWK(v)
	default:
		return JWK{}, fmt.Errorf("unsupported jwk value")
	}
}

// DIDJWKFromPublicJWK builds a did:jwk identifier from an EC public JWK.
func DIDJWKFromPublicJWK(key JWK) (string, error) {
	if strings.TrimSpace(key.D) != "" {
		return "", fmt.Errorf("did:jwk must not include private key")
	}

	public, err := ecP256PublicKeyFromJWK(key)
	if err != nil {
		return "", err
	}

	payload, err := json.Marshal(map[string]string{
		"crv": public.Crv,
		"kty": public.Kty,
		"x":   public.X,
		"y":   public.Y,
	})
	if err != nil {
		return "", err
	}

	return "did:jwk:" + base64.RawURLEncoding.EncodeToString(payload), nil
}

// JWKFromDIDJWK decodes a did:jwk identifier into public-key material.
func JWKFromDIDJWK(did string) (JWK, error) {
	did = strings.TrimSpace(did)
	if !strings.HasPrefix(did, "did:jwk:") {
		return JWK{}, fmt.Errorf("subject is not a did:jwk identifier")
	}

	encoded := strings.TrimPrefix(did, "did:jwk:")
	if encoded == "" {
		return JWK{}, fmt.Errorf("did:jwk payload is empty")
	}

	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return JWK{}, fmt.Errorf("decode did:jwk payload: %w", err)
	}

	var payload map[string]any
	err = json.Unmarshal(raw, &payload)
	if err != nil {
		return JWK{}, fmt.Errorf("parse did:jwk payload: %w", err)
	}

	return ecP256PublicKeyFromMap(payload)
}

// HolderSubjectMatches reports whether credential sub and cnf.jwk identify the same holder key.
func HolderSubjectMatches(sub string, cnfJWK JWK) error {
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return fmt.Errorf("credential missing sub")
	}

	cnf, err := ecP256PublicKeyFromJWK(cnfJWK)
	if err != nil {
		return fmt.Errorf("credential cnf.jwk: %w", err)
	}

	subject, err := JWKFromDIDJWK(sub)
	if err != nil {
		return fmt.Errorf("credential sub: %w", err)
	}

	if !publicJWKsEqual(subject, cnf) {
		return fmt.Errorf("credential sub does not match cnf.jwk holder binding")
	}

	return nil
}

func ecP256PublicKeyFromMap(raw map[string]any) (JWK, error) {
	return ecP256PublicKeyFromJWK(JWK{
		Kty: stringValue(raw["kty"]),
		Crv: stringValue(raw["crv"]),
		X:   stringValue(raw["x"]),
		Y:   stringValue(raw["y"]),
	})
}

func ecP256PublicKeyFromJWK(key JWK) (JWK, error) {
	key.Kty = strings.TrimSpace(key.Kty)
	key.Crv = strings.TrimSpace(key.Crv)
	key.X = strings.TrimSpace(key.X)
	key.Y = strings.TrimSpace(key.Y)

	if key.Kty != "EC" {
		return JWK{}, fmt.Errorf("unsupported jwk kty %q", key.Kty)
	}
	if key.Crv == "" {
		key.Crv = "P-256"
	}
	if key.Crv != "P-256" {
		return JWK{}, fmt.Errorf("unsupported jwk crv %q", key.Crv)
	}
	if key.X == "" || key.Y == "" {
		return JWK{}, fmt.Errorf("jwk is missing public key material")
	}

	return key, nil
}

func publicJWKsEqual(a, b JWK) bool {
	aNorm, errA := ecP256PublicKeyFromJWK(a)
	bNorm, errB := ecP256PublicKeyFromJWK(b)
	if errA != nil || errB != nil {
		return false
	}

	return aNorm.Kty == bNorm.Kty &&
		aNorm.Crv == bNorm.Crv &&
		aNorm.X == bNorm.X &&
		aNorm.Y == bNorm.Y
}

func stringValue(v any) string {
	s, _ := v.(string)

	return strings.TrimSpace(s)
}

func ecPublicKey(xB64, yB64 string) (*ecdsa.PublicKey, error) {
	x, err := decodeCoordinate(xB64)

	if err != nil {
		return nil, err
	}

	y, err := decodeCoordinate(yB64)

	if err != nil {
		return nil, err
	}

	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

func decodeCoordinate(value string) (*big.Int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)

	if err != nil {
		return nil, err
	}

	return new(big.Int).SetBytes(raw), nil
}
