# ADR-18: DCS talks to the Federated Catalogue as one privileged service account

## Status

Accepted (2026-07-24).

## Context

DCS publishes contract templates as assets to the Federated Catalogue (FC)
via `backend/internal/templaterepository/command/publish.go`, authenticating
with a single Keycloak `client_credentials` service account
(`federated-catalogue`, see `deployment/helm/templates/fc-realm-provision-job.yaml`).

fc-service-server's own authorization model (`SessionUtils.checkParticipantAccess`,
called from `AssetService` on every asset create/update/delete) requires the
caller's JWT `participant_id` claim to match the asset's `issuer` field,
unless the caller holds `ADMIN_ALL` or the catalogue-admin role (`Ro-MU-CA`).

The asset's `issuer` is `HolderDID` — the **per-user** DID from the request
session (`middleware.GetHolderDID(ctx)`), not a fixed DCS-instance identity.
DCS's outbound calls to FC, however, all go through the one static service
account, which has no per-request identity to place in a `participant_id`
claim. So the two can never match, and `checkParticipantAccess` denies every
publish unless the service account is given blanket rights.

Two more precise alternatives were considered and rejected as
disproportionate to the problem:

- **OAuth2 token exchange (RFC 8693) / on-behalf-of.** DCS's service account
  exchanges for a token asserting a specific `participant_id` per request.
  Correctly scoped, but requires an FC Participant to exist per HolderDID
  (bootstrapped by *something* with elevated rights anyway) plus Keycloak
  token-exchange permissions and claim-mapping wiring — real infrastructure,
  not a config change.
- **Per-user "Connect Catalogue" OAuth flow.** The user is redirected to
  Keycloak, consents, and DCS stores a per-user refresh token to call FC as
  that user going forward — the standard "connect your account" pattern.
  This is the more correct end state (see Future direction below) but is a
  real feature: consent UI, per-user token storage/refresh, and it still
  needs an FC Participant to exist for that holder before the first connect
  succeeds.

## Decision

Grant DCS's own FC service account every functional `federated-catalogue`
client role (all of them except `uma_protection`, which is Keycloak-internal),
including `ADMIN_ALL` — see `fc-realm-provision-job.yaml`'s role-mapping step.

This is deliberately blunt. It is acceptable because DCS and FC are
co-deployed as one unit (this Helm chart owns both — see
[ADR-5](adr-5-xfsc-component-posture.md)); nothing external to this
deployment ever authenticates as this service account, and no DCS end user's
credentials or scope are involved. The trust boundary is "this DCS instance
trusts its own co-deployed FC instance completely," which already holds for
every other component in this chart.

## Consequences

- Every contract template DCS publishes to its own FC instance is
  effectively published with catalogue-admin rights, regardless of which
  user authored it. There is no per-holder authorization inside FC itself
  for these calls — DCS's own RBAC (which already gated the publish action
  before it ever reached FC) is the only access control that applies.
- If DCS ever needs to expose *another party's* FC instance (cross-tenant,
  not co-deployed), this decision must be revisited — a foreign FC operator
  should not need to hand out `ADMIN_ALL` to trust DCS.

## Future direction

The "Connect Catalogue" per-user OAuth flow above is the intended correct
design if/when per-holder catalogue attribution matters (e.g. a
multi-tenant FC not co-deployed with DCS): user clicks "Connect Catalogue" →
redirected to Keycloak login/consent ("DCS asks permission to: publish on
your behalf") → bounces back with a code → DCS exchanges it and stores a
refresh token for that user → subsequent FC calls for that user carry their
own `participant_id`, and `checkParticipantAccess` enforces ownership
naturally with no elevated service-account rights needed. Not built; noted
here so the current blunt fix isn't mistaken for the intended end state.
