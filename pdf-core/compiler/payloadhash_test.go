package compiler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// The rendered "Payload hash" is a BACKLINK from the human-visible PDF to the
// machine-readable attachment: it must be the content hash of the EXACT verbatim
// embedded bytes, so A (render), B (recompile), and any verifier compute the same
// value and it genuinely resolves to the embedded payload.
func TestRenderedPayloadHashBacklinksEmbeddedBytes(t *testing.T) {
	pdf, err := CompilePDF(WithSigner(context.Background(), NewCapturingSigner()), []byte(richFilledContractPayload), CanonicalCompiledAt)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := ExtractEmbeddedJSONLD(pdf)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(embedded)
	want := hex.EncodeToString(sum[:])[:24]
	text := renderedText(t, pdf)
	if !bytes.Contains(text, []byte(want)) {
		t.Fatalf("rendered payload hash is not the content hash of the embedded bytes; expected the page to render %s", want)
	}
}
