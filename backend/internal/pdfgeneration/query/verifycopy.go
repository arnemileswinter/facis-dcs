package query

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"

	pdfgen "digital-contracting-service/gen/pdf_generation"
	"digital-contracting-service/internal/pdfgeneration/pdfcore"
	"digital-contracting-service/internal/pdfgeneration/provenance"
)

// Values of PDFVerifyResult.C2paManifestSource.
const (
	manifestSourceEmbedded = "embedded"
	manifestSourceRemote   = "remote"
)

type VerifyContractPdfCopyQry struct {
	DID string
	PDF []byte
}

// VerifyContractPdfCopyHandler verifies a copy of a contract PDF that a caller
// submits (DCS-OR-C2PA-008). The remote manifest of a contract is the manifest
// embedded in the contract PDF this instance stores: GET /c2pa/manifest/{did}
// serves it from there.
type VerifyContractPdfCopyHandler struct {
	Stored *VerifyContractPdfHandler
	Export *ExportContractPdfHandler
}

func (h *VerifyContractPdfCopyHandler) Handle(ctx context.Context, qry VerifyContractPdfCopyQry) (*pdfgen.PDFVerifyResult, error) {
	stored, err := h.Stored.Handle(ctx, VerifyContractPdfQry{DID: qry.DID})
	if err != nil {
		return nil, err
	}
	storedPDF := func(ctx context.Context) ([]byte, error) {
		pdf, err := h.Export.Handle(ctx, ExportContractPdfQry{DID: qry.DID})
		if err != nil {
			return nil, err
		}
		defer func() { _ = pdf.Close() }()
		return io.ReadAll(pdf)
	}
	return verifyCopy(ctx, h.Stored.PDFCore, h.Stored.Credentials, h.Stored.CredentialStatus, qry, stored, storedPDF)
}

// verifyCopy verifies a copy against its embedded manifest when it has one.
// Otherwise it follows the remote manifest reference in the copy's XMP
// metadata. The reference must name this contract's remote manifest. Then the
// result is the verification of the stored contract PDF that holds the remote
// manifest, and the copy must show the same pages and carry the same payload as
// that PDF.
func verifyCopy(
	ctx context.Context,
	pdfCore *pdfcore.Client,
	credentials *provenance.CredentialVerifier,
	statuses *provenance.CredentialStatusVerifier,
	qry VerifyContractPdfCopyQry,
	stored *pdfgen.PDFVerifyResult,
	storedPDF func(context.Context) ([]byte, error),
) (*pdfgen.PDFVerifyResult, error) {
	lifecycle := ""
	if stored.LifecycleStatus != nil {
		lifecycle = *stored.LifecycleStatus
	}

	if manifest, err := pdfCore.ExtractManifest(ctx, qry.PDF); err == nil && len(manifest) > 0 {
		result, err := runVerify(ctx, qry.PDF, pdfCore, credentials, statuses, lifecycle)
		if err != nil {
			return nil, err
		}
		result.C2paManifestSource = ptrToString(manifestSourceEmbedded)
		return result, nil
	}

	ref, err := pdfCore.ManifestReference(ctx, qry.PDF)
	if err != nil {
		return nil, fmt.Errorf("read the remote manifest reference of the copy of %s: %w", qry.DID, err)
	}
	if ref == "" || ref != provenance.RemoteManifestURL(qry.DID) {
		return &pdfgen.PDFVerifyResult{
			Match:               false,
			C2paManifestFound:   false,
			C2paSignatureStatus: provenance.CheckNotAvailable,
			VcProofStatus:       provenance.CheckNotAvailable,
			StatusListCheck:     "not_available",
			LifecycleStatus:     stored.LifecycleStatus,
			PdfSignatureStatus:  pdfSignatureNotAvailable,
			Discrepancy:         ptrToString(discrepancyFailed),
		}, nil
	}

	original, err := storedPDF(ctx)
	if err != nil {
		return nil, fmt.Errorf("get the contract PDF of %s that holds its remote manifest: %w", qry.DID, err)
	}
	samePages, mismatch, err := pdfCore.MatchContent(ctx, qry.PDF, original)
	if err != nil {
		return nil, fmt.Errorf("compare the pages of the copy of %s: %w", qry.DID, err)
	}
	copyPayload, err := pdfCore.ExtractPayload(ctx, qry.PDF)
	if err != nil {
		return nil, fmt.Errorf("read the payload of the copy of %s: %w", qry.DID, err)
	}
	originalPayload, err := pdfCore.ExtractPayload(ctx, original)
	if err != nil {
		return nil, fmt.Errorf("read the payload of the contract PDF of %s: %w", qry.DID, err)
	}

	result := *stored
	result.C2paManifestSource = ptrToString(manifestSourceRemote)
	if !samePages || !bytes.Equal(copyPayload, originalPayload) {
		result.Match = false
		result.Discrepancy = ptrToString(discrepancyHashMismatch)
		log.Printf("pdfgeneration: the copy of %s does not match the contract PDF: %s", qry.DID, mismatch)
	}
	return &result, nil
}
