package artifactstore

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
)

// twoInstanceStores builds a sender and a receiver store, each with its own
// key-agreement key, object store and CEK repository — the two federation
// sides of a peer CEK exchange.
func twoInstanceStores(t *testing.T) (sender, receiver *Store, senderObjects *fakeObjectStore, receiverRepo *memoryCEKRepo) {
	t.Helper()
	senderKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate sender key: %v", err)
	}
	receiverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate receiver key: %v", err)
	}
	senderObjects = newFakeObjectStore()
	receiverRepo = &memoryCEKRepo{}
	sender = New(senderObjects, &memoryCEKRepo{}, softwareKeyAgreement{priv: senderKey}, "did:web:sender.local", &senderKey.PublicKey)
	receiver = New(newFakeObjectStore(), receiverRepo, softwareKeyAgreement{priv: receiverKey}, "did:web:receiver.local", &receiverKey.PublicKey)
	return sender, receiver, senderObjects, receiverRepo
}

func TestPeerCEKRewrapRoundtrip(t *testing.T) {
	sender, receiver, senderObjects, receiverRepo := twoInstanceStores(t)
	ctx := context.Background()
	scope := ContractScope("did:web:sender.local:contract:federated")
	pdf := []byte("%PDF-1.7\nshared contract")

	cid, err := sender.Put(ctx, scope, pdf)
	if err != nil {
		t.Fatalf("sender Put: %v", err)
	}

	receiverKey := receiver.ownPub
	wrapped, err := sender.WrapForPeer(ctx, scope, "did:web:receiver.local", receiverKey)
	if err != nil {
		t.Fatalf("WrapForPeer: %v", err)
	}

	if err := receiver.AdoptPeerCEK(ctx, scope, wrapped); err != nil {
		t.Fatalf("AdoptPeerCEK: %v", err)
	}

	// The receiver persisted an own re-wrapped record, not the wire one.
	record, err := receiverRepo.Fetch(ctx, scope, "did:web:receiver.local")
	if err != nil || record == nil {
		t.Fatalf("receiver holds no own wrapped cek record: %v", err)
	}

	// Same CEK on both sides: the receiver decrypts the sender's stored
	// ciphertext to the byte-identical PDF — after a cache drop, so the CEK
	// really comes out of the persisted re-wrapped record via the receiver's
	// own key agreement.
	receiver.Forget(scope)
	plaintext, err := receiver.Decrypt(ctx, scope, senderObjects.blobs[cid])
	if err != nil {
		t.Fatalf("receiver Decrypt of sender ciphertext: %v", err)
	}
	if !bytes.Equal(plaintext, pdf) {
		t.Fatal("receiver did not recover the byte-identical PDF")
	}

	// Repeat ships are idempotent: the live own record wins, no duplicate.
	if err := receiver.AdoptPeerCEK(ctx, scope, wrapped); err != nil {
		t.Fatalf("second AdoptPeerCEK: %v", err)
	}
	records, _ := receiverRepo.List(ctx, scope)
	if len(records) != 1 {
		t.Fatalf("expected exactly one receiver record after repeat adopt, got %d", len(records))
	}
}

func TestWrapForPeerRecordsRecipientRow(t *testing.T) {
	sender, receiver, _, _ := twoInstanceStores(t)
	ctx := context.Background()
	scope := ContractScope("did:web:sender.local:contract:recipients")

	if _, err := sender.Put(ctx, scope, []byte("content")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := sender.WrapForPeer(ctx, scope, "did:web:receiver.local", receiver.ownPub); err != nil {
		t.Fatalf("WrapForPeer: %v", err)
	}

	record, err := sender.keys.Fetch(ctx, scope, "did:web:receiver.local")
	if err != nil || record == nil {
		t.Fatalf("no recipient row for the peer on the sender: %v", err)
	}
	// Repeating the wrap (every ship carries one) keeps a single live row.
	if _, err := sender.WrapForPeer(ctx, scope, "did:web:receiver.local", receiver.ownPub); err != nil {
		t.Fatalf("repeat WrapForPeer: %v", err)
	}
	records, _ := sender.keys.List(ctx, scope)
	if len(records) != 2 { // own + peer recipient
		t.Fatalf("expected own + one peer recipient row, got %d rows", len(records))
	}
}

func TestWrapForPeerWithoutCEKFails(t *testing.T) {
	sender, receiver, _, _ := twoInstanceStores(t)
	if _, err := sender.WrapForPeer(context.Background(), ContractScope("did:web:sender.local:contract:none"), "did:web:receiver.local", receiver.ownPub); err == nil {
		t.Fatal("wrapping a non-existent CEK must fail")
	}
}

func TestAdoptPeerCEKNeverResurrectsShreddedScope(t *testing.T) {
	sender, receiver, _, receiverRepo := twoInstanceStores(t)
	ctx := context.Background()
	scope := ContractScope("did:web:sender.local:contract:erased")

	if _, err := sender.Put(ctx, scope, []byte("content")); err != nil {
		t.Fatalf("sender Put: %v", err)
	}
	wrapped, err := sender.WrapForPeer(ctx, scope, "did:web:receiver.local", receiver.ownPub)
	if err != nil {
		t.Fatalf("WrapForPeer: %v", err)
	}

	// The receiver had a CEK and shredded it (peer erase completed).
	if err := receiver.AdoptPeerCEK(ctx, scope, wrapped); err != nil {
		t.Fatalf("AdoptPeerCEK: %v", err)
	}
	if _, err := receiverRepo.Shred(ctx, scope, "did:web:sender.local", "erasure requested by peer"); err != nil {
		t.Fatalf("Shred: %v", err)
	}
	receiver.Forget(scope)

	if err := receiver.AdoptPeerCEK(ctx, scope, wrapped); !IsShredded(err) {
		t.Fatalf("adopt after shred: got %v, want ShreddedError", err)
	}
	records, _ := receiverRepo.List(ctx, scope)
	for _, record := range records {
		if record.ShreddedAt == nil {
			t.Fatal("a live CEK record regrew on a shredded scope")
		}
	}
}
