package identity

import "testing"

func TestDIDWebPathSplitsAuthorityAndSegments(t *testing.T) {
	for _, tc := range []struct {
		did      string
		host     string
		segments []string
	}{
		{"did:web:example.com", "example.com", []string{}},
		{"did:web:example.com%3A8991", "example.com:8991", []string{}},
		{"did:web:example.com:tenant:b", "example.com", []string{"tenant", "b"}},
		{"did:web:localhost%3A18080:b", "localhost:18080", []string{"b"}},
	} {
		host, segments, err := DIDWebPath(tc.did)
		if err != nil {
			t.Fatalf("%s: %v", tc.did, err)
		}
		if host != tc.host {
			t.Errorf("%s: host = %q, want %q", tc.did, host, tc.host)
		}
		if len(segments) != len(tc.segments) {
			t.Fatalf("%s: segments = %v, want %v", tc.did, segments, tc.segments)
		}
		for i := range segments {
			if segments[i] != tc.segments[i] {
				t.Errorf("%s: segment %d = %q, want %q", tc.did, i, segments[i], tc.segments[i])
			}
		}
	}
}

// A bare authority resolves via /.well-known; an identifier with path segments
// resolves under those segments and must NOT use /.well-known, or every DID on
// one host collapses onto the same document.
func TestDIDWebDocumentPathFollowsSegments(t *testing.T) {
	if got := DIDWebDocumentPath(nil); got != "/.well-known/did.json" {
		t.Errorf("bare authority: got %q", got)
	}
	if got := DIDWebDocumentPath([]string{"b"}); got != "/b/did.json" {
		t.Errorf("single segment: got %q", got)
	}
	if got := DIDWebDocumentPath([]string{"tenant", "b"}); got != "/tenant/b/did.json" {
		t.Errorf("nested segments: got %q", got)
	}
}

// Two instances under one host must not be confusable: this is what the
// agreement-credential issuer check compares.
func TestDIDWebBaseURLDistinguishesInstancesOnOneHost(t *testing.T) {
	a := DIDWebBaseURL("https", "example.com", []string{"a"})
	b := DIDWebBaseURL("https", "example.com", []string{"b"})
	root := DIDWebBaseURL("https", "example.com", nil)
	if a == b || a == root || b == root {
		t.Fatalf("bases collided: a=%q b=%q root=%q", a, b, root)
	}
	if root != "https://example.com" {
		t.Errorf("root base = %q", root)
	}
}

func TestDIDWebPathRejectsMalformed(t *testing.T) {
	for _, did := range []string{"did:key:z6Mk", "did:web:", "did:web:example.com::b"} {
		if _, _, err := DIDWebPath(did); err == nil {
			t.Errorf("expected error for %q", did)
		}
	}
}

// DIDWebToHostname stays authority-only: certificate hostname verification
// checks the authority, not the path.
func TestDIDWebToHostnameIgnoresSegments(t *testing.T) {
	host, err := DIDWebToHostname("did:web:example.com%3A8991:tenant:b")
	if err != nil {
		t.Fatal(err)
	}
	if host != "example.com:8991" {
		t.Errorf("host = %q", host)
	}
}
