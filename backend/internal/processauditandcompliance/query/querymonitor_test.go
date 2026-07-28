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

// A deployment the target never received is the failure an operator most needs
// told about: the contract is in force on this side and absent on the other,
// and nothing else in the system says so. The alert must name the target and
// the reason, or it only reports that something went wrong somewhere.
func TestDeploymentFailureRisksNameTargetAndReason(t *testing.T) {
	checkedAt := time.Now().UTC()
	requested := checkedAt.Add(-2 * time.Minute)
	target := "http://dcs-orce:1880/contract-target"

	risks := deploymentFailureRisksFrom([]failedDispatch{{
		DID:           "did:web:example:contract:1",
		CorrelationID: "corr-1",
		TargetURL:     &target,
		Reason:        "connection refused",
		RequestedAt:   requested,
	}}, checkedAt)

	require.Len(t, risks, 1)
	require.Equal(t, RiskTypeDeploymentFailed, risks[0].RiskType)
	require.Equal(t, "did:web:example:contract:1", risks[0].DID)
	require.Contains(t, risks[0].Detail, target)
	require.Contains(t, risks[0].Detail, "connection refused")
	require.Contains(t, risks[0].Detail, "corr-1")
	require.True(t, risks[0].DetectedAt.Equal(checkedAt))
}

// Each failed dispatch is its own alert: a contract redeployed after a failure
// must not have its earlier failure silently folded away.
func TestDeploymentFailureRisksOnePerDispatch(t *testing.T) {
	risks := deploymentFailureRisksFrom([]failedDispatch{
		{DID: "did:web:example:contract:2", CorrelationID: "a", Reason: "timeout"},
		{DID: "did:web:example:contract:2", CorrelationID: "b", Reason: "502"},
	}, time.Now().UTC())
	require.Len(t, risks, 2)
}

// A row without a contract DID cannot be acted on and must not become an alert.
func TestDeploymentFailureRisksSkipDispatchesWithoutContractDID(t *testing.T) {
	risks := deploymentFailureRisksFrom([]failedDispatch{{CorrelationID: "a", Reason: "timeout"}}, time.Now().UTC())
	require.Empty(t, risks)
}

// An unconfigured target still has to read as a target in the alert rather than
// as an empty string an operator cannot interpret.
func TestDeploymentFailureRisksTolerateMissingTargetURL(t *testing.T) {
	risks := deploymentFailureRisksFrom([]failedDispatch{
		{DID: "did:web:example:contract:3", CorrelationID: "a", Reason: "contract target URL is empty"},
	}, time.Now().UTC())
	require.Len(t, risks, 1)
	require.Contains(t, risks[0].Detail, "contract target URL is empty")
}

func TestDeploymentFailureRisksEmptyWhenNothingFailed(t *testing.T) {
	require.Empty(t, deploymentFailureRisksFrom(nil, time.Now().UTC()))
}
