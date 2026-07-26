package qry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnauthorizedAccessRisksFlagEachDeniedActorOnce(t *testing.T) {
	checkedAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	contractDID := "did:web:facis.example:contract:denied"
	denials := []accessDenial{
		{DID: contractDID, RetrievedBy: "UnrelatedCorp"},
		// The same actor denied twice on the same contract is one risk, the
		// same per-(contract, actor) dedup MISSING_APPROVAL applies per
		// (contract, approver).
		{DID: contractDID, RetrievedBy: "UnrelatedCorp"},
		{DID: contractDID, RetrievedBy: "OtherCorp"},
	}

	risks := unauthorizedAccessRisks(denials, checkedAt)

	require.Len(t, risks, 2)
	for _, risk := range risks {
		require.Equal(t, contractDID, risk.DID)
		require.Equal(t, RiskTypeUnauthorizedAccess, risk.RiskType)
		require.Equal(t, checkedAt, risk.DetectedAt)
	}
	require.Contains(t, risks[0].Detail, "UnrelatedCorp")
	require.Contains(t, risks[1].Detail, "OtherCorp")
}

func TestUnauthorizedAccessRisksSkipDenialsWithoutContractDID(t *testing.T) {
	risks := unauthorizedAccessRisks([]accessDenial{
		{DID: "", RetrievedBy: "UnrelatedCorp"},
	}, time.Now().UTC())

	require.Empty(t, risks)
}

func TestUnauthorizedAccessRisksEmptyWhenNoDenialsPersisted(t *testing.T) {
	risks := unauthorizedAccessRisks(nil, time.Now().UTC())

	require.NotNil(t, risks)
	require.Empty(t, risks)
}
