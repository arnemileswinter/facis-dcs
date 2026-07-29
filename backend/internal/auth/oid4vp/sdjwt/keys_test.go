package sdjwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"testing"
	"time"
)

// mintTestCert issues a certificate for pub, signed by signer/signerCert, for
// chain-validation tests.
func mintTestCert(t *testing.T, cn string, pub *ecdsa.PublicKey, isCA bool, signer *ecdsa.PrivateKey, signerCert *x509.Certificate) *x509.Certificate {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, signerCert, pub, signer)
	if err != nil {
		t.Fatalf("create certificate %q: %v", cn, err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse certificate %q: %v", cn, err)
	}
	return cert
}

func mintSelfSignedCA(t *testing.T, cn string) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}
	return key, cert
}

func x5cHeaderValue(certs ...*x509.Certificate) any {
	out := make([]any, 0, len(certs))
	for _, c := range certs {
		out = append(out, base64.StdEncoding.EncodeToString(c.Raw))
	}
	return out
}

func TestVerificationKeyFromX5C_TrustedChainReturnsLeafKey(t *testing.T) {
	caKey, caCert := mintSelfSignedCA(t, "Test Trust Root")
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafCert := mintTestCert(t, "Test Issuer", &leafKey.PublicKey, false, caKey, caCert)

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	key, err := verificationKeyFromX5C(x5cHeaderValue(leafCert), roots, "Test Issuer")
	if err != nil {
		t.Fatalf("expected the chain to verify, got: %v", err)
	}
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("expected an *ecdsa.PublicKey, got %T", key)
	}
	if pub.X.Cmp(leafKey.X) != 0 || pub.Y.Cmp(leafKey.Y) != 0 {
		t.Fatalf("returned key does not match the leaf certificate's public key")
	}
}

func TestVerificationKeyFromX5C_UntrustedChainIsRefused(t *testing.T) {
	// The leaf is issued by a REAL CA, but the trust pool only knows about a
	// DIFFERENT, unrelated CA — this must fail, not fall back to trusting the
	// leaf on its own say-so.
	caKey, caCert := mintSelfSignedCA(t, "Issuing CA (not trusted)")
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafCert := mintTestCert(t, "Test Issuer", &leafKey.PublicKey, false, caKey, caCert)

	_, unrelatedCert := mintSelfSignedCA(t, "Unrelated Trusted Root")
	roots := x509.NewCertPool()
	roots.AddCert(unrelatedCert)

	if _, err := verificationKeyFromX5C(x5cHeaderValue(leafCert), roots, "Test Issuer"); err == nil {
		t.Fatal("expected an untrusted certificate chain to be refused")
	}
}

func TestVerificationKeyFromX5C_NoTrustAnchorsConfiguredIsRefused(t *testing.T) {
	caKey, caCert := mintSelfSignedCA(t, "Some CA")
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafCert := mintTestCert(t, "Test Issuer", &leafKey.PublicKey, false, caKey, caCert)

	// roots == nil: an x5c-bearing credential with nothing configured to
	// verify it against must be refused, never silently trusted.
	if _, err := verificationKeyFromX5C(x5cHeaderValue(leafCert), nil, "Test Issuer"); err == nil {
		t.Fatal("expected a nil trust pool to refuse the credential")
	}
}

func TestVerificationKeyFromX5C_IntermediateChainVerifiesAgainstRoot(t *testing.T) {
	rootKey, rootCert := mintSelfSignedCA(t, "Root CA")
	intermediateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate intermediate key: %v", err)
	}
	intermediateCert := mintTestCert(t, "Intermediate CA", &intermediateKey.PublicKey, true, rootKey, rootCert)

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	leafCert := mintTestCert(t, "Test Issuer", &leafKey.PublicKey, false, intermediateKey, intermediateCert)

	roots := x509.NewCertPool()
	roots.AddCert(rootCert)

	// x5c is leaf-first (RFC 7517 §4.7): [leaf, intermediate].
	if _, err := verificationKeyFromX5C(x5cHeaderValue(leafCert, intermediateCert), roots, "Test Issuer"); err != nil {
		t.Fatalf("expected leaf -> intermediate -> root to verify, got: %v", err)
	}
}

func TestVerificationKeyFromX5C_EmptyHeaderIsRejected(t *testing.T) {
	roots := x509.NewCertPool()
	if _, err := verificationKeyFromX5C([]any{}, roots, "Test Issuer"); err == nil {
		t.Fatal("expected an empty x5c header to be rejected")
	}
}

// A chain proves an anchor vouched for the certificate; it says nothing about
// WHOSE certificate it is. Without binding the leaf to the claimed issuer, any
// certificate under any configured anchor — a TLS server certificate included —
// signs credentials asserting any issuer identity.
func TestVerificationKeyFromX5C_LeafMustIdentifyTheClaimedIssuer(t *testing.T) {
	caKey, caCert := mintSelfSignedCA(t, "Shared Trust Root")
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	// A perfectly valid certificate under the anchor, belonging to someone else.
	leafCert := mintTestCert(t, "Some Other Service", &leafKey.PublicKey, false, caKey, caCert)

	roots := x509.NewCertPool()
	roots.AddCert(caCert)

	if _, err := verificationKeyFromX5C(x5cHeaderValue(leafCert), roots, "https://victim.example/issuer"); err == nil {
		t.Fatal("a chain that verifies but names a different subject must be refused")
	}

	// The same certificate is fine for the identity it actually carries.
	if _, err := verificationKeyFromX5C(x5cHeaderValue(leafCert), roots, "Some Other Service"); err != nil {
		t.Fatalf("the leaf must be accepted for its own identity: %v", err)
	}
}

func TestIssuerAuthority(t *testing.T) {
	cases := map[string]string{
		"did:web:example.com:issuer":             "example.com",
		"did:web:dcs-b.localhost%3A18080:issuer": "dcs-b.localhost:18080",
		"https://example.com/issuer":             "example.com",
		"urn:something:else":                     "",
	}
	for iss, want := range cases {
		if got := issuerAuthority(iss); got != want {
			t.Errorf("%s → %q, want %q", iss, got, want)
		}
	}
}
