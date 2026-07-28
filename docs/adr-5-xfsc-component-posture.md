# ADR-5: XFSC component posture — what is retained, what is substituted

## Status

Accepted. Revised 2026-07-27 to record the retained Federated Catalogue's
supported graph-store and deployment lifecycle.

## Context

DCS is built against the XFSC (GAIA-X Federated Secure Computing) stack.
Appendix D describes the OCM W-Stack deployment, whose "essential crypto
parts are defined in the Crypto Provider Service (formerly TSA Signer
Service)." Appendix E describes the Status List Service, deployable
standalone or together with the Crypto Provider Service. IR-SI-02 requires
Node-RED-webhook-compatible orchestration integration. Not every XFSC
component fits DCS's actual key-custody decision (ADR-1), and this ADR
records, component by component, which are used as specified and which are
substituted.

## Decision

| XFSC component | Posture |
|---|---|
| Federated Catalogue | Retained. DCS integrates with its asset, schema, query and verification interfaces (`backend/internal/templatecatalogueintegration/client/client.go:100-105`, `backend/internal/templatecatalogueintegration/client/client.go:108-183`). The supported co-deployment uses Fuseki as its graph store; Neo4j/n10s is no longer a runtime dependency (`deployment/helm/charts/federated-catalogue/templates/deployment.yaml:41-53`). |
| XFSC Status List Service | Retained, integrated as specified — credential revocation status (`backend/internal/auth/oid4vp/status_list*.go`) and C2PA lifecycle status publication both resolve to it. |
| ORCE (orchestration engine) | Retained; IR-SI-02's Node-RED-webhook compatibility is satisfied by the shipped example flow (`deployment/helm/charts/orce/flows/contract-target-flow.json`). |
| Crypto Provider Service (Appendix D's "essential crypto parts") | **Substituted** by PKCS#11/HSM key custody (ADR-1). IR-HI-01's "standardized interfaces" wording is read as satisfied by PKCS#11 itself being the standardized interface — DCS does not require the specific Crypto Provider Service REST API to satisfy that requirement, since PKCS#11 already is one. |
| OCM W-Stack (issuance/verification/retrieval/well-known) | Not used for VC signing — see [adr-ocmw-vc-signing.md](adr-ocmw-vc-signing.md) for the detailed evaluation of why the OCM-W protocol (OID4VCI wallet-pull) does not fit DCS's synchronous in-process signing requirement. Remains the natural integration point if DCS later issues credentials *to* wallet holders (a different feature than what it does today). |

### Federated Catalogue deployment lifecycle

The retained catalogue is deployed from the DCS Helm chart with Fuseki,
PostgreSQL and Keycloak. Fuseki was selected over the earlier Neo4j/n10s
combination because FC startup invoked `n10s.graphconfig.show`, while the n10s
plugin was incompatible with the drifting Neo4j patch image. The result was a
terminal crash loop that could never become ready. The current chart selects
`GRAPHSTORE_IMPL=fuseki`, points FC at the Fuseki dataset and disables the
irrelevant Spring Neo4j health indicator
(`deployment/helm/charts/federated-catalogue/templates/deployment.yaml:41-53`).
The Fuseki image is pinned to the FC revision used by the service
(`deployment/helm/charts/fuseki/values.yaml:1-13`).

Readiness is a lifecycle, not a fixed delay:

1. Keycloak reports its native OIDC discovery endpoint ready
   (`deployment/helm/charts/keycloak/templates/deployment.yaml:114-122`).
2. Realm provisioning waits on the native Deployment condition and runs once,
   with no Job retry (`deployment/helm/templates/fc-realm-provision-job.yaml:41-70`).
3. The backend observes both terminal Job conditions. `Complete` permits
   startup; `Failed` returns the Kubernetes reason and fails the init container
   (`deployment/helm/templates/deployment.yaml:69-123`).
4. FC's own readiness probe uses `/actuator/health`
   (`deployment/helm/charts/federated-catalogue/templates/deployment.yaml:124-134`).
5. DCS performs one semantic `/verification` call and one schema
   synchronization before exposing `/readyz` as ready
   (`backend/internal/templatecatalogueintegration/client/client.go:108-183`,
   `backend/cmd/dcs/main.go:497-507`,
   `backend/cmd/dcs/readiness.go:19-28`).

This sequence deliberately contains no FC warm-up request, schema-sync retry or
blanket readiness sleep. A terminal response is returned immediately to the
deployment entry point, which prints the relevant Deployment, Job, Pod and log
diagnostics (`deployment/helm/deploy.sh:90-101`,
`deployment/helm/deploy.sh:119-147`).

## Consequences

- The substitution is documented rather than silently made, so a reviewer
  checking Appendix D compliance finds the reasoning here instead of
  discovering an unexplained gap.
- The swap-back path (re-adopting the Crypto Provider Service instead of
  PKCS#11 directly) stays open: DCS's signer interfaces (`VCSigner`,
  the HSM `crypto.Signer`) are narrow enough that a Crypto-Provider-backed
  implementation could be substituted without touching call sites.
- A development installation that still contains the obsolete Neo4j/n10s
  catalogue is replaced by the current Fuseki-based chart; catalogue data from
  the unreleased development stack is not migrated. This is explicitly covered
  by the lifecycle acceptance scenario
  (`features/25_federated_catalogue_deployment_lifecycle/federated_catalogue_deployment_lifecycle.feature:45-52`).
- Reproducibility depends on the committed Helm dependency lock and immutable
  FC workload images. The deployment path rebuilds dependencies from
  `Chart.lock` without refreshing it (`deployment/helm/deploy.sh:103-107`), and
  the lifecycle contract verifies deterministic rendering plus digest-pinned FC
  images (`deployment/helm/tests/federated_catalogue_lifecycle_test.sh:16-29`,
  `deployment/helm/tests/federated_catalogue_lifecycle_test.sh:59-87`).
