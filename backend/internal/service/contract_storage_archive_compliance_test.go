package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/contractworkflowengine/db"
)

func complianceJSON(t *testing.T, value any) *datatype.JSON {
	t.Helper()
	j, err := datatype.NewJSON(value)
	require.NoError(t, err)
	return &j
}

func complianceVerdicts(t *testing.T, entry db.ContractArchiveEntry, meta *db.ContractMetadata, now time.Time) map[string]string {
	t.Helper()
	verdicts := map[string]string{}
	for _, check := range archiveComplianceChecks(entry, meta, now) {
		if check.Passed {
			verdicts[check.Rule] = "pass"
			continue
		}
		require.NotNil(t, check.Reason, "a failed rule must say why")
		verdicts[check.Rule] = *check.Reason
	}
	return verdicts
}

func TestCompliantArchiveEntryPassesEveryRule(t *testing.T) {
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	retention := now.AddDate(5, 0, 0)
	start, end := now.AddDate(-1, 0, 0), now.AddDate(1, 0, 0)
	name := "Supply agreement"
	entry := db.ContractArchiveEntry{
		ContractSnapshot: datatype.JSON(`{"did":"x"}`),
		ContentHash:      "sha256:" + zeroHash,
		SignatureMeta:    complianceJSON(t, map[string]any{"status": "SIGNED", "credential_type": "AES"}),
		CredentialHashes: complianceJSON(t, map[string]any{"did:web:signer": "sha256:abc"}),
		TSAReceipt:       complianceJSON(t, map[string]any{"token": "t"}),
		RetentionUntil:   &retention,
	}
	meta := &db.ContractMetadata{Name: &name, StartDate: &start, ExpDate: &end,
		Responsible: &db.Responsible{Creator: "did:web:a", Counterparty: "did:web:b"}}

	for rule, verdict := range complianceVerdicts(t, entry, meta, now) {
		require.Equal(t, "pass", verdict, rule)
	}
}

func TestNonCompliantArchiveEntryIsFlaggedPerRule(t *testing.T) {
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	retention := now.AddDate(0, 0, -1)
	entry := db.ContractArchiveEntry{
		ContractSnapshot: datatype.JSON(`{"did":"x"}`),
		ContentHash:      "sha256:" + zeroHash,
		SignatureMeta:    complianceJSON(t, map[string]any{}),
		CredentialHashes: complianceJSON(t, map[string]any{}),
		TSAReceipt:       complianceJSON(t, map[string]any{}),
		RetentionUntil:   &retention,
	}
	meta := &db.ContractMetadata{Responsible: &db.Responsible{Creator: "did:web:a"}}

	verdicts := complianceVerdicts(t, entry, meta, now)
	require.Contains(t, verdicts["retention"], "retention period ended on 2026-10-29")
	require.Contains(t, verdicts["signature_policy"], "no completed signature recorded")
	require.Contains(t, verdicts["signature_policy"], "no signer credential hashes recorded")
	require.Equal(t, "missing name, parties, validity period", verdicts["metadata_completeness"])
	require.Equal(t, "no TSA receipt", verdicts["evidence_completeness"])
}
