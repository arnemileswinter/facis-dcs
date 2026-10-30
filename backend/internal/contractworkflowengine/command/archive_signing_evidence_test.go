package command

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

// DCS-FR-CSA-08, DCS-FR-CWE-20: the archive entry holds every signature of the
// contract and the credential hashes of each, never the credentials.
func TestBuildArchiveEntryStoresEverySignatureWithHashedCredentials(t *testing.T) {
	now := time.Now().UTC()
	signature := func(n string) ArchiveSigningEvidence {
		return ArchiveSigningEvidence{
			Signer: "did:web:signer-" + n, CredentialType: "AES", CeremonyID: "ceremony-" + n, Field: "signature-" + n, SignedAt: now,
			PDFCID: "bafy-pdf-" + n, PDFHash: strings.Repeat(n, 64), CredentialHashes: map[string]string{"presentation": "sha256:" + strings.Repeat("b", 64)},
		}
	}
	peers := json.RawMessage(`[{"party":"did:web:peer","signatory":"did:jwk:peer"}]`)
	entry, err := BuildArchiveEntry(&db.Contract{DID: "did:web:contract", ContractVersion: 1, State: contractstate.Signed.String(), CreatedAt: now, UpdatedAt: now},
		"did:web:operator", []ArchiveSigningEvidence{signature("1"), signature("2")}, peers)
	if err != nil {
		t.Fatal(err)
	}

	var metadata struct {
		Status         string           `json:"status"`
		PDFCID         string           `json:"pdf_cid"`
		Signatures     []map[string]any `json:"signatures"`
		PeerSignatures []map[string]any `json:"peer_signatures"`
	}
	if err := json.Unmarshal(*entry.SignatureMeta, &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Status != "SIGNED" || metadata.PDFCID != "bafy-pdf-2" {
		t.Errorf("the entry must name the final signed PDF: %+v", metadata)
	}
	if len(metadata.Signatures) != 2 {
		t.Fatalf("want both signatures, got %d", len(metadata.Signatures))
	}
	for _, sig := range metadata.Signatures {
		for _, key := range []string{"signer", "credential_type", "ceremony_id", "field", "signed_at", "pdf_cid", "pdf_hash"} {
			if sig[key] == "" || sig[key] == nil {
				t.Errorf("signature %v misses %s", sig["field"], key)
			}
		}
	}
	if len(metadata.PeerSignatures) != 1 {
		t.Errorf("want the peer signature, got %v", metadata.PeerSignatures)
	}

	var hashes map[string]map[string]string
	if err := json.Unmarshal(*entry.CredentialHashes, &hashes); err != nil {
		t.Fatal(err)
	}
	if len(hashes) != 2 || hashes["ceremony-1"]["presentation"] == "" || hashes["ceremony-2"]["presentation"] == "" {
		t.Errorf("want the credential hashes of both ceremonies, got %v", hashes)
	}
	if strings.Contains(strings.ToLower(string(*entry.CredentialHashes)), "vp_token") {
		t.Fatalf("credential evidence is not hash-only: %s", *entry.CredentialHashes)
	}
}

// DCS-FR-SM-10: the notarized and timestamped content hash is a proof of
// execution over the signer identities, so another signer gives another hash.
func TestArchiveContentHashCoversTheSignerIdentities(t *testing.T) {
	now := time.Now().UTC()
	contract := &db.Contract{DID: "did:web:contract", ContractVersion: 1, State: contractstate.Signed.String(), CreatedAt: now, UpdatedAt: now}
	entry := func(signer string) db.ContractArchiveEntry {
		t.Helper()
		built, err := BuildArchiveEntry(contract, "did:web:operator", []ArchiveSigningEvidence{{Signer: signer, CredentialType: "AES", SignedAt: now, PDFHash: strings.Repeat("a", 64)}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return built
	}
	first, second := entry("did:jwk:alice"), entry("did:jwk:bob")
	if !strings.Contains(string(first.ContractSnapshot), "did:jwk:alice") {
		t.Fatalf("the snapshot does not name the signer: %s", first.ContractSnapshot)
	}
	if first.ContentHash == second.ContentHash {
		t.Fatal("the content hash does not cover the signer identity")
	}
}
