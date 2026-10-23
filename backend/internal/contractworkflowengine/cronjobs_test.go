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

// DCS-FR-CWE-10: the expiry alert carries the configured policy, so the
// subscriber knows which workflow to start.
func TestNewExpiredEventCarriesThePolicy(t *testing.T) {
	now := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	evt, err := newExpiredEvent(renewalDueContract(), now)
	require.NoError(t, err)

	require.Equal(t, eventtype.ContractExpired.String(), evt.EventType())
	require.Equal(t, "did:example:contract-1", evt.DID)
	require.Equal(t, contractstate.Signed, evt.State)
	require.NotNil(t, evt.ExpPolicy)
	require.Equal(t, expirationpolicy.Renewal, *evt.ExpPolicy)
	require.Equal(t, now, evt.OccurredAt)
}

// A contract without a policy still expires and still alerts.
func TestNewExpiredEventWithoutPolicy(t *testing.T) {
	c := renewalDueContract()
	c.ExpPolicy = nil
	evt, err := newExpiredEvent(c, time.Now())
	require.NoError(t, err)
	require.Nil(t, evt.ExpPolicy)

	payload, err := json.Marshal(evt)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	require.Contains(t, decoded, "exp_policy")
	require.Nil(t, decoded["exp_policy"])
}

func TestNewExpiredEventRejectsAnUnknownPolicy(t *testing.T) {
	c := renewalDueContract()
	policy := "extend"
	c.ExpPolicy = &policy
	_, err := newExpiredEvent(c, time.Now())
	require.Error(t, err)
}
