package deployevent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"digital-contracting-service/internal/contractworkflowengine/command"
	"digital-contracting-service/internal/processauditandcompliance/workflowgate"
)

type recordingDeployer struct {
	commands []command.DeployCmd
	notReady error
}

func (r *recordingDeployer) Deployable(context.Context, string, string) error {
	return r.notReady
}

func (r *recordingDeployer) Handle(_ context.Context, cmd command.DeployCmd) (*command.DeployResult, error) {
	r.commands = append(r.commands, cmd)
	return &command.DeployResult{DID: cmd.DID}, nil
}

func appliedSignatureEvent(t *testing.T, did string) json.RawMessage {
	t.Helper()
	return json.RawMessage(`{"did":"` + did + `"}`)
}

// Both deployment paths run the same multi-signer gate, and that gate reads
// which declared slot is this instance's own from LocalPeer. Left empty here,
// every slot of a federated contract looked local and the counterparty's — held
// only in the counterparty's database — was demanded from ours, so a fully
// countersigned contract could never auto-deploy at all.
func TestAutoDeployPassesThisInstancesOwnPeerIdentity(t *testing.T) {
	deployer := &recordingDeployer{}
	sub := &Subscriber{Deployer: deployer, LocalPeer: "did:web:dcs-a.localhost"}

	require.NoError(t, sub.handle(context.Background(), appliedSignatureEvent(t, "did:web:example#contract")))

	require.Len(t, deployer.commands, 1)
	require.Equal(t, "did:web:dcs-a.localhost", deployer.commands[0].LocalPeer)
	require.Equal(t, "did:web:example#contract", deployer.commands[0].DID)
	require.Equal(t, "system:auto-deploy", deployer.commands[0].RequestedBy)
}

func TestAutoDeployIgnoresAnEventWithoutAContract(t *testing.T) {
	deployer := &recordingDeployer{}
	sub := &Subscriber{Deployer: deployer, LocalPeer: "did:web:dcs-a.localhost"}

	require.NoError(t, sub.handle(context.Background(), appliedSignatureEvent(t, "")))
	require.Empty(t, deployer.commands)
}

// After the first of two signatures the contract may not deploy yet. The
// deployment workflow gate must not be dispatched then: an open gate run
// blocks the deployment once the last signature arrives.
func TestAutoDeployDispatchesNoGateForAPartiallySignedContract(t *testing.T) {
	deployer := &recordingDeployer{notReady: command.ErrSigningIncomplete}
	// An unconfigured coordinator answers every Execute with a BlockedError.
	sub := &Subscriber{Deployer: deployer, Gate: &workflowgate.Coordinator{}, LocalPeer: "did:web:dcs-a.localhost"}

	err := sub.handle(context.Background(), appliedSignatureEvent(t, "did:web:example#contract"))

	require.ErrorIs(t, err, command.ErrSigningIncomplete)
	var blocked *workflowgate.BlockedError
	require.NotErrorAs(t, err, &blocked, "the gate ran for a contract that may not deploy")
	require.Empty(t, deployer.commands)
}
