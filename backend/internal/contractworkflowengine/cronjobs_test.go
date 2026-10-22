package contractworkflowengine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/eventtype"
	"digital-contracting-service/internal/contractworkflowengine/datatype/expirationpolicy"
	database "digital-contracting-service/internal/contractworkflowengine/db"
)

func renewalDueContract() database.ContractMetadata {
	exp := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	notice := 30
	policy := "renewal"
	return database.ContractMetadata{
		DID:             "did:example:contract-1",
		ContractVersion: 3,
		State:           "SIGNED",
		ExpDate:         &exp,
		ExpNoticePeriod: &notice,
		ExpPolicy:       &policy,
		Responsible:     &database.Responsible{Creator: "did:example:origin", Counterparty: "did:example:peer"},
	}
}

// DCS-FR-CWE-22: subscribers receive the deadline itself, so the payload must
// carry the contract's expiry date, notice period and policy as stored.
func TestNewRenewalDueEventCarriesTheDeadline(t *testing.T) {
	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	evt, err := newRenewalDueEvent(renewalDueContract(), now)
	require.NoError(t, err)

	require.Equal(t, eventtype.ContractRenewalDue.String(), evt.EventType())
	require.Equal(t, "did:example:contract-1", evt.GetDID())

	raw, err := json.Marshal(evt)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "did:example:contract-1", payload["did"])
	require.EqualValues(t, 3, payload["contract_version"])
	require.Equal(t, contractstate.Signed.String(), payload["state"])
	require.Equal(t, "2026-11-20T12:00:00Z", payload["exp_date"])
	require.EqualValues(t, 30, payload["exp_notice_period"])
	require.Equal(t, expirationpolicy.Renewal.String(), payload["exp_policy"])
	require.Equal(t, "2026-10-21T08:00:00Z", payload["occurred_at"])
	require.Equal(t, "did:example:peer", payload["responsible"].(map[string]any)["counterparty"])
}

func TestNewRenewalDueEventWithoutPolicy(t *testing.T) {
	c := renewalDueContract()
	c.ExpPolicy = nil
	evt, err := newRenewalDueEvent(c, time.Now())
	require.NoError(t, err)
	require.Nil(t, evt.ExpPolicy)
}

func TestNewRenewalDueEventRejectsIncompleteContracts(t *testing.T) {
	for name, mutate := range map[string]func(*database.ContractMetadata){
		"no expiry date":   func(c *database.ContractMetadata) { c.ExpDate = nil },
		"no notice period": func(c *database.ContractMetadata) { c.ExpNoticePeriod = nil },
		"unknown state":    func(c *database.ContractMetadata) { c.State = "BOGUS" },
		"unknown policy":   func(c *database.ContractMetadata) { p := "BOGUS"; c.ExpPolicy = &p },
	} {
		t.Run(name, func(t *testing.T) {
			c := renewalDueContract()
			mutate(&c)
			_, err := newRenewalDueEvent(c, time.Now())
			require.Error(t, err)
		})
	}
}
