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

// DCS-FR-CWE-31: a contract whose target reports a breaching KPI must raise an
// alert, and the alert has to say WHICH metric and what value — otherwise an
// operator learns only that something is wrong.
func TestUnderperformanceRisksNameMetricAndValue(t *testing.T) {
	checkedAt := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC)
	observed := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	risks := underperformanceRisksFromKPIs([]violatingKPI{
		{DID: "did:web:example:contract:1", Metric: "coverage", Value: "80", ObservedAt: observed},
	}, checkedAt)

	require.Len(t, risks, 1)
	require.Equal(t, RiskTypeUnderperformance, risks[0].RiskType)
	require.Equal(t, "did:web:example:contract:1", risks[0].DID)
	require.Contains(t, risks[0].Detail, "coverage")
	require.Contains(t, risks[0].Detail, "80")
	require.True(t, risks[0].DetectedAt.Equal(checkedAt))
}

// Each breaching report is its own alert: collapsing them would hide that a
// contract is missing several targets.
func TestUnderperformanceRisksOnePerReport(t *testing.T) {
	risks := underperformanceRisksFromKPIs([]violatingKPI{
		{DID: "did:web:example:contract:2", Metric: "coverage", Value: "80"},
		{DID: "did:web:example:contract:2", Metric: "delivery_days", Value: "12"},
	}, time.Now().UTC())
	require.Len(t, risks, 2)
}

// A row without a contract DID cannot be acted on and must not become an alert.
func TestUnderperformanceRisksSkipReportsWithoutContractDID(t *testing.T) {
	risks := underperformanceRisksFromKPIs([]violatingKPI{{Metric: "coverage", Value: "80"}}, time.Now().UTC())
	require.Empty(t, risks)
}

func TestUnderperformanceRisksEmptyWhenNothingViolating(t *testing.T) {
	require.Empty(t, underperformanceRisksFromKPIs(nil, time.Now().UTC()))
}
