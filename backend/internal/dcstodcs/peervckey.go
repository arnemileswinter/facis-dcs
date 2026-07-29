package dcstodcs

import (
	"crypto/ecdsa"
	"fmt"
	"strings"

	"digital-contracting-service/internal/base/identity"
)

// PeerAssertionKey returns the key a peer's proof names, provided that peer
// publishes it as one that may make assertions.
//
// The verification method is taken from the proof rather than guessed. This
// instance labels its credential key `#dcs-vc`, but that is a local convention,
// not something DID Core requires: another implementation publishes `#key-1`, a
// UUID, or an absolute DID URL, and deriving the id from our own label works
// only for as long as every peer runs this software. A proof already says which
// key made it; the document says whether that key was allowed to.
//
// Being listed in assertionMethod is the authorization. A DID document
// deliberately separates its relationships — our own gendid publishes a
// key-agreement key in the same document — so a key that is merely present is
// not a key entitled to assert anything.
func PeerAssertionKey(doc *identity.DIDDocument, verificationMethodID string) (*ecdsa.PublicKey, error) {
	if doc == nil {
		return nil, fmt.Errorf("no peer did document")
	}
	methodID := strings.TrimSpace(verificationMethodID)
	if methodID == "" {
		return nil, fmt.Errorf("the proof names no verification method")
	}

	docID, err := doc.GetID()
	if err != nil {
		return nil, fmt.Errorf("peer did.json: %w", err)
	}
	// A bare fragment is relative to the document that carries it.
	if strings.HasPrefix(methodID, "#") {
		methodID = docID + methodID
	}
	// The key has to belong to the peer we are talking to, not to a document it
	// points at: an absolute DID URL naming somebody else proves nothing here.
	if base, _, _ := strings.Cut(methodID, "#"); base != docID {
		return nil, fmt.Errorf("proof names verification method %q, which does not belong to %q", verificationMethodID, docID)
	}

	if !assertionMethodAuthorizes(doc, methodID) {
		return nil, fmt.Errorf("peer did.json does not list %q as an assertionMethod, so it may not make assertions", methodID)
	}

	for i := range doc.VerificationMethod {
		if doc.VerificationMethod[i].ID != methodID {
			continue
		}
		key, err := doc.VerificationMethod[i].PublicKeyJWK.ECPublicKey()
		if err != nil {
			return nil, fmt.Errorf("peer verification method %q: %w", methodID, err)
		}
		return key, nil
	}
	return nil, fmt.Errorf("peer did.json publishes no verification method %q", methodID)
}

// assertionMethodAuthorizes reports whether the document lists this method as
// one that may make assertions, by reference or embedded inline.
func assertionMethodAuthorizes(doc *identity.DIDDocument, methodID string) bool {
	for _, entry := range doc.AssertionMethod {
		switch value := entry.(type) {
		case string:
			if value == methodID {
				return true
			}
		case map[string]any:
			if id, _ := value["id"].(string); id == methodID {
				return true
			}
		}
	}
	return false
}
