package compiler

import "testing"

// An unanchored search for "19 0 obj" also matches inside "100019 0 obj". Both
// signing gates resolved objects that way, so an appended revision could
// supersede a page stream or the JSON-LD attachment with tampered content and
// carry a decoy whose id merely ENDS in the target's digits holding the
// original — the gate read the decoy and reported a match on a document whose
// visible page had been replaced.
func TestObjectHeaderLookupIsNotFooledByAnIDSuffix(t *testing.T) {
	pdf := []byte("%PDF-1.7\n" +
		"19 0 obj\n<< >>\nstream\nORIGINAL\nendstream\nendobj\n" +
		"100019 0 obj\n<< >>\nstream\nDECOY\nendstream\nendobj\n")

	if got := findLastObjectHeaderOffset(pdf, 19); got != 9 {
		t.Errorf("last header for 19 resolved to %d, not the real definition at 9", got)
	}
	if got := findFirstObjectHeaderOffset(pdf, 19); got != 9 {
		t.Errorf("first header for 19 resolved to %d, not 9", got)
	}
	// The decoy is still findable by its own id.
	if got := findLastObjectHeaderOffset(pdf, 100019); got <= 9 {
		t.Errorf("object 100019 resolved to %d, which is not its own definition", got)
	}

	content, err := extractStreamContentByObjID(pdf, 19)
	if err != nil {
		t.Fatalf("extract object 19: %v", err)
	}
	if string(content) != "ORIGINAL" {
		t.Errorf("extracted %q — a suffix-colliding decoy was read instead of object 19", content)
	}
}

// A superseding revision must win: the gate compares what a reader renders.
func TestObjectHeaderLookupPrefersTheLatestDefinition(t *testing.T) {
	pdf := []byte("%PDF-1.7\n" +
		"19 0 obj\n<< >>\nstream\nFIRST\nendstream\nendobj\n" +
		"19 0 obj\n<< >>\nstream\nSUPERSEDED\nendstream\nendobj\n")

	content, err := extractStreamContentByObjID(pdf, 19)
	if err != nil {
		t.Fatalf("extract object 19: %v", err)
	}
	if string(content) != "SUPERSEDED" {
		t.Errorf("extracted %q, want the latest definition", content)
	}
}
