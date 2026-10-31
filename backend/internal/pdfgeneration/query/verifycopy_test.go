package query

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pdfgen "digital-contracting-service/gen/pdf_generation"
	"digital-contracting-service/internal/pdfgeneration/pdfcore"
	"digital-contracting-service/internal/pdfgeneration/provenance"
)

const copyTestDID = "did:example:contract-copy"

// token returns the value of "key=value" in a fake PDF, or "".
func token(pdf []byte, key string) string {
	for _, field := range strings.Fields(string(pdf)) {
		if v, ok := strings.CutPrefix(field, key+"="); ok {
			return v
		}
	}
	return ""
}

// newCopyPDFCore fakes the pdf-core endpoints a copy verification calls. A fake
// PDF is a line of fields: "manifest" when it embeds a C2PA manifest,
// "ref=<url>" for its XMP manifest reference, "pages=<x>" for its page content
// and "payload=<x>" for its machine-readable payload.
func newCopyPDFCore(t *testing.T) *pdfcore.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest/extract":
			body, _ := io.ReadAll(r.Body)
			if !bytes.Contains(body, []byte(" manifest ")) {
				http.Error(w, `{"name":"bad_request"}`, http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte("jumb"))
		case "/manifest/reference":
			body, _ := io.ReadAll(r.Body)
			if ref := token(body, "ref"); ref != "" {
				_, _ = w.Write([]byte(ref))
				return
			}
			http.Error(w, `{"name":"not_found"}`, http.StatusNotFound)
		case "/payload/extract":
			body, _ := io.ReadAll(r.Body)
			_, _ = w.Write([]byte(token(body, "payload")))
		case "/verify/content-match":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			pdf, ref := []byte(r.FormValue("pdf")), []byte(r.FormValue("reference"))
			_ = json.NewEncoder(w).Encode(map[string]any{"match": token(pdf, "pages") == token(ref, "pages")})
		case "/verify":
			_ = json.NewEncoder(w).Encode(map[string]any{"match": true, "c2pa_signature_valid": true})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return pdfcore.New(srv.URL, func([]byte) ([]byte, error) { return make([]byte, 64), nil })
}

// storedVerification is the verification of the contract PDF that holds the
// remote manifest.
func storedVerification() *pdfgen.PDFVerifyResult {
	return &pdfgen.PDFVerifyResult{
		Match:               true,
		C2paManifestFound:   true,
		C2paSignatureStatus: provenance.CheckValid,
		VcProofStatus:       provenance.CheckNotAvailable,
		StatusListCheck:     "not_available",
		LifecycleStatus:     ptrToString("active"),
		PdfSignatureStatus:  pdfSignatureNotAvailable,
	}
}

func verifyTestCopy(t *testing.T, copyPDF string) (*pdfgen.PDFVerifyResult, bool) {
	t.Helper()
	fetched := false
	original := func(context.Context) ([]byte, error) {
		fetched = true
		return []byte("%PDF manifest ref=" + provenance.RemoteManifestURL(copyTestDID) + " pages=A payload=P"), nil
	}
	result, err := verifyCopy(context.Background(), newCopyPDFCore(t), nil, nil,
		VerifyContractPdfCopyQry{DID: copyTestDID, PDF: []byte(copyPDF)}, storedVerification(), original)
	if err != nil {
		t.Fatalf("verifyCopy: %v", err)
	}
	return result, fetched
}

// A copy whose embedded manifest was removed is verified from the remote
// manifest it references (DCS-OR-C2PA-008).
func TestStrippedCopyIsVerifiedFromTheRemoteManifest(t *testing.T) {
	result, fetched := verifyTestCopy(t, "%PDF ref="+provenance.RemoteManifestURL(copyTestDID)+" pages=A payload=P")
	if !fetched {
		t.Fatal("the contract PDF that holds the remote manifest was not fetched")
	}
	if !result.Match || !result.C2paManifestFound || result.C2paSignatureStatus != provenance.CheckValid {
		t.Fatalf("the stripped copy did not verify from the remote manifest: %+v", result)
	}
	if result.C2paManifestSource == nil || *result.C2paManifestSource != manifestSourceRemote {
		t.Fatalf("c2pa_manifest_source = %v, want %q", result.C2paManifestSource, manifestSourceRemote)
	}
}

// The remote manifest describes the contract PDF; a copy with other pages or
// another payload does not verify from it.
func TestStrippedCopyWithOtherContentDoesNotVerify(t *testing.T) {
	ref := " ref=" + provenance.RemoteManifestURL(copyTestDID)
	for name, copyPDF := range map[string]string{
		"other pages":   "%PDF" + ref + " pages=B payload=P",
		"other payload": "%PDF" + ref + " pages=A payload=Q",
	} {
		t.Run(name, func(t *testing.T) {
			result, _ := verifyTestCopy(t, copyPDF)
			if result.Match {
				t.Fatalf("a copy with %s verified: %+v", name, result)
			}
			if result.Discrepancy == nil || *result.Discrepancy != discrepancyHashMismatch {
				t.Fatalf("discrepancy = %v, want %q", result.Discrepancy, discrepancyHashMismatch)
			}
		})
	}
}

// A stripped copy that references no manifest, or the manifest of another
// contract, has no manifest to verify against.
func TestStrippedCopyWithoutThisContractsReferenceHasNoManifest(t *testing.T) {
	for name, copyPDF := range map[string]string{
		"no reference":       "%PDF pages=A payload=P",
		"another contract's": "%PDF ref=" + provenance.RemoteManifestURL("did:example:other") + " pages=A payload=P",
	} {
		t.Run(name, func(t *testing.T) {
			result, fetched := verifyTestCopy(t, copyPDF)
			if fetched {
				t.Fatal("a manifest was fetched for a copy that does not reference this contract's")
			}
			if result.Match || result.C2paManifestFound {
				t.Fatalf("a copy without this contract's manifest verified: %+v", result)
			}
		})
	}
}

// A copy that carries its manifest is verified against it.
func TestCopyWithAnEmbeddedManifestIsVerifiedAgainstIt(t *testing.T) {
	result, fetched := verifyTestCopy(t, "%PDF manifest pages=A payload=P")
	if fetched {
		t.Fatal("the remote manifest was fetched for a copy that embeds its manifest")
	}
	if !result.Match || !result.C2paManifestFound {
		t.Fatalf("the copy did not verify against its embedded manifest: %+v", result)
	}
	if result.C2paManifestSource == nil || *result.C2paManifestSource != manifestSourceEmbedded {
		t.Fatalf("c2pa_manifest_source = %v, want %q", result.C2paManifestSource, manifestSourceEmbedded)
	}
}
