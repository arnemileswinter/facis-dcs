package provenance

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestSigningSummaryBindsExistingCredentialToPoAContext(t *testing.T) {
	signer := &captureSigner{}
	_, _, err := IssueSigningSummaryVC(context.Background(), signer, "did:example:issuer", SigningSummary{
		ContractID: "did:example:contract", SignerDID: "did:jwk:holder",
		CeremonyID: "ceremony-1", FieldName: "did:web:party",
		ContentHash: "content", PDFHash: "pdf", CredentialType: "AES",
		KBSDHash: "sd", SignedAt: time.Now().UTC(),
		PoAPresentation: "issuer~disclosure~kb", PoANonce: "nonce-1",
		PoAAudience: "dcs-signature-ceremony",
	})
	if err != nil {
		t.Fatal(err)
	}
	var vc struct {
		CredentialSubject map[string]any `json:"credentialSubject"`
	}
	if err := json.Unmarshal(signer.lastUnsigned, &vc); err != nil {
		t.Fatal(err)
	}
	if vc.CredentialSubject["poa_presentation"] != "issuer~disclosure~kb" ||
		vc.CredentialSubject["poa_nonce"] != "nonce-1" ||
		vc.CredentialSubject["poa_audience"] != "dcs-signature-ceremony" {
		t.Fatalf("PoA context missing from ContractSigningSummaryCredential: %#v", vc.CredentialSubject)
	}
}
