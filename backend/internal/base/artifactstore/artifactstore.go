// Package artifactstore is the encrypting layer above the IPFS client
// (DCS-NFR-SEC-14): every artifact is encrypted with a per-scope random CEK
// before it reaches IPFS, and the CEK exists only as wrapped records in
// content_encryption_keys. Shredding those records (DCS-NFR-COMP-03,
// DCS-NFR-SEC-13) makes the stored ciphertext permanently opaque; the store
// then reports a ShreddedError instead of content.
package artifactstore

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"digital-contracting-service/internal/base/envelope"
	"digital-contracting-service/internal/base/ipfs"
)

// Kind is the erasure unit an artifact belongs to.
type Kind string

const (
	// KindContract scopes all artifacts of one contract (IRI): PDFs, archive
	// snapshot, audit-entry bodies. The Art. 17 erasure unit.
	KindContract Kind = "contract"
	// KindTemplate scopes a template's PDFs.
	KindTemplate Kind = "template"
	// KindInstance scopes instance-level artifacts (Merkle checkpoints,
	// reports, audit entries without a contract). Not shreddable.
	KindInstance Kind = "instance"
)

// Scope identifies one CEK: a kind plus the resource IRI (or the instance DID).
type Scope struct {
	Kind Kind
	ID   string
}

func ContractScope(iri string) Scope { return Scope{Kind: KindContract, ID: iri} }
func TemplateScope(iri string) Scope { return Scope{Kind: KindTemplate, ID: iri} }

// AAD is the canonical scope string bound into every ciphertext as GCM AAD.
func (s Scope) AAD() string { return string(s.Kind) + ":" + s.ID }

// ShreddedError reports that the scope's CEK records were destroyed: the
// content is erased for good. Callsites map it to a 4xx, never a 500.
type ShreddedError struct{ Scope Scope }

func (e *ShreddedError) Error() string {
	return fmt.Sprintf("content encryption key for %s %s has been shredded; the content is erased", e.Scope.Kind, e.Scope.ID)
}

// IsShredded reports whether err is (or wraps) a ShreddedError.
func IsShredded(err error) bool {
	var shredded *ShreddedError
	return errors.As(err, &shredded)
}

// ObjectStore is the dumb bytes-in/bytes-out IPFS surface the store encrypts
// over (ipfs.APIClient satisfies it).
type ObjectStore interface {
	CreateFile(ctx context.Context, data any) (*ipfs.IPFSResult, error)
	FetchFile(cid string) (*ipfs.IPFSResult, error)
}

// cekCacheLimit bounds the in-memory plaintext-CEK cache.
const cekCacheLimit = 256

// Store encrypts artifacts per scope before IPFS storage. The plaintext CEK
// lives only in this in-memory cache; at rest it exists solely wrapped to the
// instance's keyAgreement key.
type Store struct {
	objects   ObjectStore
	keys      CEKRepository
	agreement envelope.KeyAgreement
	ownDID    string
	ownPub    *ecdsa.PublicKey

	mu    sync.Mutex
	cache map[Scope][]byte
	order []Scope
}

func New(objects ObjectStore, keys CEKRepository, agreement envelope.KeyAgreement, ownDID string, ownPub *ecdsa.PublicKey) *Store {
	return &Store{
		objects:   objects,
		keys:      keys,
		agreement: agreement,
		ownDID:    ownDID,
		ownPub:    ownPub,
		cache:     make(map[Scope][]byte),
	}
}

// InstanceScope is the scope for instance-level artifacts.
func (s *Store) InstanceScope() Scope { return Scope{Kind: KindInstance, ID: s.ownDID} }

// Put encrypts plaintext under the scope's CEK (created and wrapped on first
// use) and stores the ciphertext in IPFS, returning its CID.
func (s *Store) Put(ctx context.Context, scope Scope, plaintext []byte) (string, error) {
	blob, err := s.Encrypt(ctx, scope, plaintext)
	if err != nil {
		return "", err
	}
	res, err := s.objects.CreateFile(ctx, blob)
	if err != nil {
		return "", fmt.Errorf("store %s %s artifact: %w", scope.Kind, scope.ID, err)
	}
	if res == nil || res.Identifier.Value == "" {
		return "", fmt.Errorf("store %s %s artifact: object store returned no CID", scope.Kind, scope.ID)
	}
	return res.Identifier.Value, nil
}

// Get fetches the CID from IPFS and decrypts it with the scope's CEK,
// returning the byte-identical plaintext artifact.
func (s *Store) Get(ctx context.Context, scope Scope, cid string) ([]byte, error) {
	cek, err := s.cek(ctx, scope, false)
	if err != nil {
		return nil, err
	}
	res, err := s.objects.FetchFile(cid)
	if err != nil {
		return nil, fmt.Errorf("fetch %s %s artifact %s: %w", scope.Kind, scope.ID, cid, err)
	}
	plaintext, err := envelope.Decrypt(cek, res.Data, scope.AAD())
	if err != nil {
		return nil, fmt.Errorf("decrypt %s %s artifact %s: %w", scope.Kind, scope.ID, cid, err)
	}
	return plaintext, nil
}

// Encrypt encrypts a value under the scope's CEK without storing it (used for
// the encrypted audit-entry bodies, which live inside plaintext entry objects).
func (s *Store) Encrypt(ctx context.Context, scope Scope, plaintext []byte) ([]byte, error) {
	cek, err := s.cek(ctx, scope, true)
	if err != nil {
		return nil, err
	}
	return envelope.Encrypt(cek, plaintext, scope.AAD())
}

// Decrypt reverses Encrypt. After a shred it returns a ShreddedError.
func (s *Store) Decrypt(ctx context.Context, scope Scope, blob []byte) ([]byte, error) {
	cek, err := s.cek(ctx, scope, false)
	if err != nil {
		return nil, err
	}
	return envelope.Decrypt(cek, blob, scope.AAD())
}

// cek returns the scope's plaintext CEK, creating and persisting a wrapped
// record on first use when create is set. A shredded scope never yields a key
// again — not even for writes.
func (s *Store) cek(ctx context.Context, scope Scope, create bool) ([]byte, error) {
	if cek := s.cached(scope); cek != nil {
		return cek, nil
	}

	record, err := s.keys.Fetch(ctx, scope, s.ownDID)
	if err != nil {
		return nil, fmt.Errorf("read wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	if record != nil && record.ShreddedAt != nil {
		return nil, &ShreddedError{Scope: scope}
	}

	if record == nil {
		if !create {
			return nil, fmt.Errorf("no content encryption key for %s %s", scope.Kind, scope.ID)
		}
		cek, err := s.createCEK(ctx, scope)
		if err != nil || cek != nil {
			return cek, err
		}
		// Lost the insert race: a concurrent writer persisted the record first.
		record, err = s.keys.Fetch(ctx, scope, s.ownDID)
		if err != nil {
			return nil, fmt.Errorf("read wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
		}
		if record == nil {
			return nil, fmt.Errorf("wrapped cek for %s %s vanished after insert conflict", scope.Kind, scope.ID)
		}
		if record.ShreddedAt != nil {
			return nil, &ShreddedError{Scope: scope}
		}
	}

	var wrapped envelope.WrappedCEK
	if err := json.Unmarshal(record.WrappedCEK, &wrapped); err != nil {
		return nil, fmt.Errorf("decode wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	cek, err := envelope.Unwrap(&wrapped, s.agreement)
	if err != nil {
		return nil, fmt.Errorf("unwrap cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	s.remember(scope, cek)
	return cek, nil
}

// createCEK draws a fresh CEK, wraps it to the instance's keyAgreement key and
// persists it. A nil,nil return means an insert conflict (concurrent creator).
func (s *Store) createCEK(ctx context.Context, scope Scope) ([]byte, error) {
	cek, err := envelope.NewCEK()
	if err != nil {
		return nil, err
	}
	wrapped, err := envelope.Wrap(cek, s.ownPub)
	if err != nil {
		return nil, fmt.Errorf("wrap cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return nil, fmt.Errorf("encode wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	inserted, err := s.keys.Insert(ctx, scope, s.ownDID, encoded)
	if err != nil {
		return nil, fmt.Errorf("persist wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	if !inserted {
		return nil, nil
	}
	s.remember(scope, cek)
	return cek, nil
}

// WrapForPeer wraps the scope's CEK to a counterparty instance's keyAgreement
// public key and records the peer as a recipient of the scope. The returned
// record travels in the peer ship payload; the local recipient row documents
// which foreign instances hold the CEK.
func (s *Store) WrapForPeer(ctx context.Context, scope Scope, peerDID string, peerPub *ecdsa.PublicKey) (*envelope.WrappedCEK, error) {
	cek, err := s.cek(ctx, scope, false)
	if err != nil {
		return nil, err
	}
	wrapped, err := envelope.Wrap(cek, peerPub)
	if err != nil {
		return nil, fmt.Errorf("wrap cek of %s %s for peer %s: %w", scope.Kind, scope.ID, peerDID, err)
	}
	encoded, err := json.Marshal(wrapped)
	if err != nil {
		return nil, fmt.Errorf("encode wrapped cek of %s %s for peer %s: %w", scope.Kind, scope.ID, peerDID, err)
	}
	// Idempotent: a live recipient row from an earlier ship stays in place.
	if _, err := s.keys.Insert(ctx, scope, peerDID, encoded); err != nil {
		return nil, fmt.Errorf("persist peer recipient cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	return wrapped, nil
}

// AdoptPeerCEK unwraps a peer-shipped wrapped CEK with the instance's own HSM,
// re-wraps it to the own keyAgreement key and persists it as the scope's CEK.
// A live own record already in place wins (repeat ships are idempotent); a
// shredded scope is never resurrected.
func (s *Store) AdoptPeerCEK(ctx context.Context, scope Scope, wrapped *envelope.WrappedCEK) error {
	record, err := s.keys.Fetch(ctx, scope, s.ownDID)
	if err != nil {
		return fmt.Errorf("read wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	if record != nil && record.ShreddedAt != nil {
		return &ShreddedError{Scope: scope}
	}
	if record != nil {
		return nil
	}

	cek, err := envelope.Unwrap(wrapped, s.agreement)
	if err != nil {
		return fmt.Errorf("unwrap peer cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	rewrapped, err := envelope.Wrap(cek, s.ownPub)
	if err != nil {
		return fmt.Errorf("re-wrap peer cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	encoded, err := json.Marshal(rewrapped)
	if err != nil {
		return fmt.Errorf("encode re-wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	inserted, err := s.keys.Insert(ctx, scope, s.ownDID, encoded)
	if err != nil {
		return fmt.Errorf("persist re-wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
	}
	if !inserted {
		// Lost the race against a concurrent creator or a shred; the next read
		// resolves the winner through the repository.
		record, err = s.keys.Fetch(ctx, scope, s.ownDID)
		if err != nil {
			return fmt.Errorf("read wrapped cek for %s %s: %w", scope.Kind, scope.ID, err)
		}
		if record != nil && record.ShreddedAt != nil {
			return &ShreddedError{Scope: scope}
		}
		return nil
	}
	s.remember(scope, cek)
	return nil
}

func (s *Store) cached(scope Scope) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache[scope]
}

func (s *Store) remember(scope Scope, cek []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cache[scope]; ok {
		s.cache[scope] = cek
		return
	}
	if len(s.order) >= cekCacheLimit {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.cache, oldest)
	}
	s.cache[scope] = cek
	s.order = append(s.order, scope)
}

// Forget drops a scope's plaintext CEK from the in-memory cache (part of a
// shred: the wrapped records are marked destroyed and the cache must not keep
// the key alive).
func (s *Store) Forget(scope Scope) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cache[scope]; !ok {
		return
	}
	delete(s.cache, scope)
	for i, cached := range s.order {
		if cached == scope {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
}
