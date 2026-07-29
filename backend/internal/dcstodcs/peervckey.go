package dcstodcs

import (
	"crypto/ecdsa"
	"fmt"

	"digital-contracting-service/internal/base/identity"
)

// PeerVCPublicKey returns the key a peer signs verifiable credentials with.
//
// It is NOT the document's first verification method: gendid publishes that
// slot as the eIDAS/JAdES identity key (dcs-did) and the VC key (dcs-vc) as a
// method of its own, precisely so a verifier can tell them apart. Taking the
// first one verified every signing summary against the wrong key, which fails
// on every genuine ship — found by array position rather than by the id the
// signer actually names in its proof.
func PeerVCPublicKey(doc *identity.DIDDocument) (*ecdsa.PublicKey, string, error) {
	if doc == nil {
		return nil, "", fmt.Errorf("no peer did document")
	}
	docID, err := doc.GetID()
	if err != nil {
		return nil, "", fmt.Errorf("peer did.json: %w", err)
	}
	wantID := vcVerificationMethodID(docID)
	for i := range doc.VerificationMethod {
		if doc.VerificationMethod[i].ID != wantID {
			continue
		}
		key, err := doc.VerificationMethod[i].PublicKeyJWK.ECPublicKey()
		if err != nil {
			return nil, "", fmt.Errorf("peer VC verificationMethod key: %w", err)
		}
		return key, wantID, nil
	}
	return nil, "", fmt.Errorf("peer did.json publishes no %q verificationMethod", wantID)
}
