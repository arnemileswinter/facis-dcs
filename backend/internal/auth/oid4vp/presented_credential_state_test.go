package oid4vp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/auth/oid4vp/status"
)

// The compliance viewer reads the live status of the Power of Attorney a
// signatory presented. The same stored presentation must follow the list:
// valid before the issuer revokes it, revoked after.
func TestPresentedCredentialState_FollowsTheLiveStatusList(t *testing.T) {
	list := newXFSCStatusList(t, 16, testPoAIssuer)
	issuerKey, holderKey := newECKey(t), newECKey(t)
	poa := mintPoA(t, issuerKey, holderKey, testPoAIssuer, testPoAParty, list, nil)
	trust := peerTrust(t, issuerKey, []Purpose{PurposePeer}, []string{testPoAParty}, list)

	state, err := PresentedCredentialState(context.Background(), poa.Presentation, trust, PurposePeer)
	require.NoError(t, err)
	assert.Equal(t, status.StateValid, state)

	list.Revoke(testPoAStatusIndex)

	state, err = PresentedCredentialState(context.Background(), poa.Presentation, trust, PurposePeer)
	require.NoError(t, err)
	assert.Equal(t, status.StateInvalid, state)
}

// A list that cannot be fetched gives no state. The caller reports UNKNOWN; a
// default of "valid" would hide a revocation for as long as the outage lasts.
func TestPresentedCredentialState_UnreachableListIsAnError(t *testing.T) {
	list := newXFSCStatusList(t, 16, testPoAIssuer)
	issuerKey, holderKey := newECKey(t), newECKey(t)
	poa := mintPoA(t, issuerKey, holderKey, testPoAIssuer, testPoAParty, list, map[string]any{
		"status": map[string]any{"status_list": map[string]any{"uri": "http://127.0.0.1:1/status-list/1", "idx": 1}},
	})
	trust := peerTrust(t, issuerKey, []Purpose{PurposePeer}, []string{testPoAParty}, list)

	state, err := PresentedCredentialState(context.Background(), poa.Presentation, trust, PurposePeer)
	require.Error(t, err)
	assert.Empty(t, state)
}

// The status entry is the credential author's pointer, so it is followed only
// for a credential an issuer trusted for the purpose signed.
func TestPresentedCredentialState_UntrustedIssuerIsNotFollowed(t *testing.T) {
	list := newXFSCStatusList(t, 16, testPoAIssuer)
	issuerKey, holderKey := newECKey(t), newECKey(t)
	poa := mintPoA(t, issuerKey, holderKey, testPoAIssuer, testPoAParty, list, nil)
	trust := peerTrust(t, issuerKey, []Purpose{PurposeLogin}, []string{testPoAParty}, list)

	_, err := PresentedCredentialState(context.Background(), poa.Presentation, trust, PurposePeer)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not trusted")
}
