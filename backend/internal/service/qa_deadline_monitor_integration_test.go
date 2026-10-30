package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	cwepg "digital-contracting-service/internal/contractworkflowengine/db/pg"
	pacpg "digital-contracting-service/internal/processauditandcompliance/db/pg"
	qry "digital-contracting-service/internal/processauditandcompliance/query"
)

// DCS-FR-CWE-09 (ADR-33): the scheduled sweep flags an obligation of a contract
// in force whose deadline passed without a report, and stops once the target
// reported on the rule.
func TestQADeadlineMonitorFlagsUnreportedPassedDeadlinePostgres(t *testing.T) {
	database, _ := qaPostgresSchema(t)
	_, err := database.Exec(`INSERT INTO contracts (did,origin,created_by,state,template_did,template_version,contract_data) VALUES ('active','','Org A','ACTIVE','tpl',1,
		'{"dcs:policies":{"odrl:obligation":[{"@id":"urn:uuid:report","odrl:constraint":[{"odrl:leftOperand":{"@id":"odrl:dateTime"},"odrl:operator":{"@id":"odrl:lt"},"odrl:rightOperand":"2026-01-01"}]}]}}')`)
	require.NoError(t, err)
	monitor := qry.ComplianceMonitor{DB: database, ATRepo: &cwepg.PostgresApprovalTaskRepo{}, CRepo: &cwepg.PostgresContractRepo{}, FRepo: &pacpg.PostgresRiskFindingRepo{}}
	deadlineRisks := func() int {
		result, err := monitor.Handle(context.Background(), qry.MonitorQry{MonitoredBy: qry.SystemMonitorPrincipal})
		require.NoError(t, err)
		count := 0
		for _, risk := range result.Risks {
			if risk.RiskType == qry.RiskTypeDeadlineMissed {
				count++
			}
		}
		return count
	}

	require.Equal(t, 1, deadlineRisks())

	_, err = database.Exec(`INSERT INTO contract_kpis (did,metric,value,verdict,rule_id) VALUES ('active','reported','yes','satisfied','urn:uuid:report')`)
	require.NoError(t, err)
	require.Zero(t, deadlineRisks())
}
