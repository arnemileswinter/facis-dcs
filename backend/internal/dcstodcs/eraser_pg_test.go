package dcstodcs

import (
	"context"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/artifactstore"
	"digital-contracting-service/internal/base/pgtest"
)

const erasedDID = "did:web:dcs.example:contract:erase-1"

// seedErasableContract writes a contract with plaintext content in every
// Postgres place that holds it, plus its live wrapped CEK.
func seedErasableContract(t *testing.T, db *sqlx.DB) {
	t.Helper()
	statements := []string{
		`INSERT INTO contracts (did, created_by, state, template_did, template_version, contract_data)
		 VALUES ($1, 'did:web:creator', 'SIGNED', 'did:web:tpl', 1, '{"clause":"secret terms"}')`,
		`INSERT INTO contract_history (did, created_by, state, template_did, template_version, contract_data)
		 VALUES ($1, 'did:web:creator', 'DRAFT', 'did:web:tpl', 1, '{"clause":"older secret terms"}')`,
		`INSERT INTO contract_negotiations (did, contract_version, change_request, created_by)
		 VALUES ($1, 1, '{"clause":"proposed secret terms"}', 'did:web:creator')`,
		`INSERT INTO contract_negotiation_drafts (contract_did, saved_by, change_request)
		 VALUES ($1, 'did:web:creator', '{"clause":"draft secret terms"}')`,
		`INSERT INTO contract_archive_entries (did, contract_version, stored_by, contract_snapshot, content_hash, snapshot_cid, evidence)
		 VALUES ($1, 1, 'did:web:creator', '{"contract_data":{"clause":"secret terms"}}',
		         'sha256:` + "0000000000000000000000000000000000000000000000000000000000000000" + `', 'bafy-snapshot',
		         '{"source":"SIGNING_WORKFLOW_COMPLETION","snapshot_hash_algorithm":"SHA-256"}')`,
		`INSERT INTO signature_ceremonies (id, contract_did, field_name, requested_by, nonce, expires_at, pinned_payload)
		 VALUES (gen_random_uuid(), $1, 'sig', 'did:web:creator', 'n', now(), 'secret terms'::bytea)`,
		`INSERT INTO content_encryption_keys (scope_kind, scope_id, recipient_did, wrapped_cek)
		 VALUES ('contract', $1, 'did:web:dcs.example', '{"alg":"ECDH-ES+A256KW","encrypted_key":"x"}')`,
	}
	for _, statement := range statements {
		_, err := db.Exec(statement, erasedDID)
		require.NoError(t, err, statement)
	}
}

func TestShredDestroysTheWrappedKeyAndErasesPlaintextContent(t *testing.T) {
	db := pgtest.Open(t)
	seedErasableContract(t, db)

	shredder := &AuditedShredder{
		DB:        db,
		Keys:      &artifactstore.PostgresCEKRepo{DB: db},
		Artifacts: artifactstore.New(nil, nil, nil, "", "", nil),
	}
	affected, err := shredder.Shred(context.Background(), erasedDID, "did:web:archive-manager", "Art. 17 request")
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)

	var liveKeys int
	require.NoError(t, db.Get(&liveKeys, `SELECT count(*) FROM content_encryption_keys WHERE scope_id = $1 AND wrapped_cek IS NOT NULL`, erasedDID))
	require.Zero(t, liveKeys, "a shredded scope must not keep any wrapped CEK")

	var plaintext int
	require.NoError(t, db.Get(&plaintext, `
		SELECT (SELECT count(*) FROM contracts WHERE did = $1 AND contract_data::text LIKE '%secret%')
		     + (SELECT count(*) FROM contract_history WHERE did = $1 AND contract_data::text LIKE '%secret%')
		     + (SELECT count(*) FROM contract_negotiations WHERE did = $1 AND change_request::text LIKE '%secret%')
		     + (SELECT count(*) FROM contract_negotiation_drafts WHERE contract_did = $1)
		     + (SELECT count(*) FROM contract_archive_entries WHERE did = $1 AND contract_snapshot::text LIKE '%secret%')
		     + (SELECT count(*) FROM signature_ceremonies WHERE contract_did = $1 AND pinned_payload IS NOT NULL)`, erasedDID))
	require.Zero(t, plaintext, "plaintext contract content survived the erasure")

	var shredEvents int
	require.NoError(t, db.Get(&shredEvents, `SELECT count(*) FROM outbox_events WHERE did = $1 AND event_type = 'KEY_SHREDDED'`, erasedDID))
	require.Equal(t, 1, shredEvents)
}

// The archive snapshot can only be erased, and only after the contract's keys
// are shredded: any other change stays refused by the database.
func TestArchiveSnapshotIsErasableOnlyAfterTheKeysAreShredded(t *testing.T) {
	db := pgtest.Open(t)
	seedErasableContract(t, db)

	_, err := db.Exec(`UPDATE contract_archive_entries SET contract_snapshot = '{}'::jsonb WHERE did = $1`, erasedDID)
	require.ErrorContains(t, err, "immutable", "the snapshot was erasable while the keys were live")

	_, err = db.Exec(`UPDATE content_encryption_keys SET wrapped_cek = NULL, shredded_at = now() WHERE scope_id = $1`, erasedDID)
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE contract_archive_entries SET contract_snapshot = '{"forged":true}'::jsonb WHERE did = $1`, erasedDID)
	require.ErrorContains(t, err, "immutable", "a shredded scope let the snapshot be replaced, not erased")

	_, err = db.Exec(`UPDATE contract_archive_entries SET contract_snapshot = '{}'::jsonb WHERE did = $1`, erasedDID)
	require.NoError(t, err)
}

// A shredded record without its key is the only shape the table accepts: a
// shred that left the wrapped CEK behind is refused.
func TestShreddedKeyRowMustNotKeepItsWrappedKey(t *testing.T) {
	db := pgtest.Open(t)
	seedErasableContract(t, db)

	_, err := db.Exec(`UPDATE content_encryption_keys SET shredded_at = now() WHERE scope_id = $1`, erasedDID)
	require.ErrorContains(t, err, "chk_cek_shredded_has_no_key")
}
