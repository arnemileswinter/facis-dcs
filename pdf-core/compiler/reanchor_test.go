package compiler

import (
	"bytes"
	"testing"
	"time"
)

// A signature is applied after the lifecycle manifest so that it commits to the
// provenance, which leaves that manifest's whole-file hard binding covering
// less than the file it now lives in. ADR-26 resolves this by re-anchoring
// afterwards: a provenance-only manifest whose binding covers the signed bytes,
// appended so the signature's byte range stays untouched and the signature
// keeps verifying in external tools.
//
// Re-anchoring changes no payload, so it cannot go through the amendment path,
// which refuses an unchanged document.
func TestReanchorProvenanceAppendsOverASignedDocument(t *testing.T) {
	compiled, err := CompilePDF(testSigningContext(), []byte(minimalPayloadBase), time.Now())
	if err != nil {
		t.Fatalf("CompilePDF: %v", err)
	}
	signedLike := appendSignatureRevision(compiled)

	reanchored, err := ReanchorProvenance(testSigningContext(), signedLike, "", time.Now())
	if err != nil {
		t.Fatalf("ReanchorProvenance: %v", err)
	}

	if !bytes.HasPrefix(reanchored, signedLike) {
		t.Fatal("re-anchoring must append; rewriting signed bytes would invalidate the signature")
	}
	c2paBytes, err := extractEmbeddedStreamByFileSpecName(reanchored, "content_credential.c2pa")
	if err != nil {
		t.Fatalf("extract C2PA: %v", err)
	}
	before, _ := extractEmbeddedStreamByFileSpecName(signedLike, "content_credential.c2pa")
	beforeCount, _ := extractTopLevelManifestBoxes(before)
	afterCount, err := extractTopLevelManifestBoxes(c2paBytes)
	if err != nil {
		t.Fatalf("extract manifests: %v", err)
	}
	if len(afterCount) != len(beforeCount)+1 {
		t.Fatalf("manifest count = %d, want %d (one re-anchor appended)", len(afterCount), len(beforeCount)+1)
	}
}

// The amendment path must keep refusing an unchanged payload: re-anchoring is
// the deliberate exception, not a general loosening of that guard.
func TestUnchangedPayloadIsStillRefusedByTheAmendmentPath(t *testing.T) {
	compiled, err := CompilePDF(testSigningContext(), []byte(minimalPayloadBase), time.Now())
	if err != nil {
		t.Fatalf("CompilePDF: %v", err)
	}
	payload, err := ExtractEmbeddedJSONLD(compiled)
	if err != nil {
		t.Fatalf("extract payload: %v", err)
	}
	if _, err := UpdatePDF(testSigningContext(), compiled, payload, time.Now()); err == nil {
		t.Fatal("an unchanged payload must still be refused as no-changes")
	}
}

// appendSignatureRevision stands in for the PAdES layer: bytes appended after
// the manifest, carrying the markers isPAdESSigned looks for.
func appendSignatureRevision(pdf []byte) []byte {
	return append(append([]byte(nil), pdf...),
		[]byte("\n% signature revision\n/Type /Sig /ByteRange [0 1 2 3]\n%%EOF\n")...)
}
