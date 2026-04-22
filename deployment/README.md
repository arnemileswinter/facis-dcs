[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../LICENSE)

# Digital Contracting Service

An automated orchestration workspace that deploys a [Digital Contracting Service](https://github.com/eclipse-xfsc/facis/tree/main/DCS) instance to a Kubernetes cluster.

---

## Overview

The Digital Contracting Service (DCS) provides an open-source platform for creating, signing, and managing contracts digitally.
Integrated with the European Digital Identity Wallet (EUDI), it guarantees that all digital transactions are secure, legally binding, and interoperable.

Key components:
- **Multi-Contract Signing** — multi-party contract execution within a single workflow
- **Automated Workflows** — contract generation, execution, and deployment
- **Lifecycle Management** — contract monitoring with renewal/expiration alerts
- **Signature Management** — signatures linked to verifiable digital identities
- **Secure Archiving** — tamper-evident archive compliant with retention policies
- **Machine Signing** — automated signing for high-volume transactions

---

## Helm Chart

The parent chart bundles `postgresql`, `hydra`, `nats`, `neo4j`, and `federated-catalogue` as optional sub-charts, each toggled via `<subchart>.enabled`.

For DCS itself, authentication is now treated as generic OIDC. Per the SRS, the recommended production direction is Hydra as the OIDC server, with the login/consent experience backed by OCM-W's OpenID4VC wallet flow so users authenticate with wallet-held credentials and role claims.

Hydra requires explicit login and consent URLs. In this repository, the intended setup is that DCS acts as the auth app and serves those routes via the frontend gateway. In local development, `values.dev.yml` points Hydra at `http://localhost:5173/hydra/login` and `http://localhost:5173/hydra/consent`, which Vite proxies to the local DCS backend.

When sub-charts are disabled, point DCS to external services via:
- `serviceDiscovery.postgresqlHost`
- `serviceDiscovery.hydraHost`
- `serviceDiscovery.natsHost`

Routing is configured with `route.basePath` (e.g. `/tenant-a/dcs`) or explicit `paths.api` / `paths.ui` overrides.

---

## Local Development

### Prerequisites
- [Rancher Desktop](https://rancherdesktop.io/) with Kubernetes enabled (provides `kubectl`, `helm`, and NodePort forwarding to `localhost`)
- Go with [air](https://github.com/air-verse/air) (`go install github.com/air-verse/air@latest`)
- Node.js 20+
- Goa **v3** – Installation: Follow the instructions on [Goa Quickstart](https://goa.design/docs/1-goa/quickstart/)


#### Initialize all dependencies
Run the following command in **backend** to initialize all needed dependencies:
```bash
go mod tidy
```

#### Generate Go code with Goa
Generate the required glue code under `gen/` with the Goa CLI:
```bash
goa gen digital-contracting-service/design
```

### 1. Deploy dependencies

```bash
helm dependency build ./deployment/helm
helm install dcs ./deployment/helm -f ./deployment/helm/values.dev.yml
```

This starts all dependencies as NodePort services forwarded to `localhost`.
With `values.dev.yml`, the parent DCS Kubernetes workload is disabled (`app.enabled: false`) because the backend is expected to run locally:

| Service              | Address                          |
|----------------------|----------------------------------|
| PostgreSQL           | `localhost:30432`                |
| Hydra Public         | `http://localhost:30080`         |
| Hydra Admin          | `http://localhost:30085`         |
| NATS                 | `nats://localhost:30422`         |
| Neo4j HTTP           | `http://localhost:30474`         |
| Neo4j Bolt           | `bolt://localhost:30687`         |

Hydra public OIDC endpoints, login, and consent are handled through the Vite gateway:
- `http://localhost:5173/.well-known/openid-configuration`
- `http://localhost:5173/oauth2/auth`
- `http://localhost:5173/hydra/login`
- `http://localhost:5173/hydra/consent`

To upgrade after chart changes:
```bash
helm upgrade dcs ./deployment/helm -f ./deployment/helm/values.dev.yml --server-side=false --force-replace
```

### 2. Run the backend

```bash
cp backend/.env.dev backend/.env
cd backend && air
```

The backend listens on `http://localhost:8991`.

### 3. Run the frontend

```bash
cd frontend/ClientApp
npm install
npm run dev
```

The Vite dev server starts at `http://localhost:5173` and proxies `/api` and `/hydra` requests to the backend automatically.

---

## BDD Tests

BDD scenarios live in `features/` at the project root. Tests are run against a full stack in an ephemeral [kind](https://kind.sigs.k8s.io/) cluster.

The BDD profile (`deployment/helm/values.bdd.yml`) seeds Hydra with a deterministic RSA JWK so tokens minted during test runs are signed by a stable key and validate consistently against the Hydra issuer JWKS.

### Prerequisites
- `kind` — `go install sigs.k8s.io/kind@v0.23.0` or see [kind releases](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- `kubectl` and `helm`
- Docker (to build the DCS image)
- Python 3.10+

### Run locally

```bash
# Build DCS image, spin up kind cluster, deploy via Helm, run all scenarios
make -C tests/bdd run_bdd_kind_ci
```

This single command:
1. Builds the DCS Docker image (`digital-contracting-service:bdd`)
2. Creates a kind cluster named `dcs-bdd`
3. Loads the image into the cluster
4. Deploys the full stack via `deployment/helm` with `values.bdd.yml`
5. Port-forwards DCS into the cluster network
6. Runs all `features/**/*.feature` scenarios with behave

Tear down the cluster afterwards:
```bash
make -C tests/bdd kind_delete
```

### Run against an already-deployed Helm release

If you have a release running (e.g. via Rancher Desktop):

```bash
make -C tests/bdd run_bdd_helm_dev \
  K8S_NAMESPACE=default \
  HELM_RELEASE=dcs
```

### CI

The `bdd-kind.yml` GitHub Actions workflow runs:

```yaml
make -C tests/bdd run_bdd_kind_ci
```

JUnit reports are published as check annotations and uploaded as workflow artifacts.

---

## Production Deployment

### OIDC Provider
- Use a properly secured external OIDC provider
- For wallet-backed role authentication, use Hydra together with an OCM-W-backed login/consent service so wallet presentation drives the OIDC login flow
- Configure valid redirect URIs in your client settings:
  - **Valid Redirect URIs**: `https://<domain>/<path>/api/auth/callback`
  - **Valid Post Logout Redirect URIs**: `https://<domain>/<path>/api/auth/logout-complete`
- Enable the authorization code flow for the DCS client

### TLS
- Use certificates from a trusted Certificate Authority
- Recommend [cert-manager](https://cert-manager.io/) for automatic renewal

### Values
Override the following at minimum:

```yaml
oidc:
  issuerURL: "https://hydra.example.com/"
  clientID: "dcs-client"
  redirectURI: "https://example.com/dcs/ui/"
  logoutRedirectURI: "https://example.com/dcs/ui/"

route:
  basePath: "/dcs"
```

---

## Known issues / post-install patches

### Wallet presentation flow (`credential-verification-service`)

The vendored upstream chart `credential-verification-service-1.0.2` from the
ocm-wstack OCI registry has a few quirks that affect the OID4VP presentation
flow used for wallet-based login. None of these are fixable from `values.yaml`
alone; apply them after `helm install` / `helm upgrade`.

1. **`clientUrlSchema` env var name mismatch (chart bug).**
   The chart template renders `config.externalPresentation.clientUrlSchema`
   into the env var `CREDENTIALVERIFICATION_CLIENTURLSCHEMA`, but the CV
   service binary actually reads it from
   `CREDENTIALVERIFICATION_EXTERNALPRESENTATION_CLIENTURLSCHEMA` (nested under
   the `ExternalPresentation` envconfig group). The chart-rendered value is
   therefore silently ignored and the service falls back to the default
   `https`, which makes CV crash on startup in local dev.

   The umbrella chart works around this with a post-install/post-upgrade
   hook (`templates/cv-env-patch-job.yaml`) that patches the deployment
   with the correct env var. Configurable under `cvEnvPatch:` in
   [values.yaml](helm/values.yaml); set `cvEnvPatch.enabled: false` to opt
   out, or override `cvEnvPatch.clientUrlSchema` for non-dev deployments.

2. **`response_uri` host comes from the incoming `Host` header.**
   CV builds the JWT `response_uri` claim as
   `<clientUrlSchema>://<incoming Host>/<publicBasePath>/<id>`. The DCS
   backend therefore overrides the outgoing `Host` header on its CV
   `/presentation/request` call so the wallet sees the public gateway host
   instead of the CV NodePort. Configure via `PRESENTATION_PUBLIC_HOST` in
   the backend env (see [backend/.env.dev](../backend/.env.dev)). For local
   dev this is `localhost:5173`; the Vite dev server proxies
   `/api/presentation/proof/...` to the CV NodePort.

   The `publicBasePath` advertised in `response_uri` defaults to
   `/api/presentation/proof`, but the **actual** CV proof-receive handler
   is mounted at `/v1/tenants/{tenantId}/presentation/proof/{id}`. The
   chart does not expose `publicBasePath` as a value, so the dev gateway
   must rewrite the path on its way to CV. The Vite dev proxy in
   `frontend/ClientApp/vite.config.ts` does this rewrite. Production
   ingress configurations need an equivalent rewrite rule.

3. **`client_id` claim is taken verbatim from the `x-did` header.**
   CV does not derive a verifier DID itself. The DCS backend sends the value
   of `OCM_W_PRESENTATION_DID` as `x-did`, which CV puts into the JWT
   `client_id` claim. If the env var is empty the wallet will reject the
   request with `missing client_id`.

4. **`enbaled` typo in chart values.**
   The chart's `externalPresentation.enbaled` key is misspelled in the
   upstream template. Keep the same misspelling in `values.dev.yml` so the
   rendered env value stays a string (see the inline comment there).

---

## License

Apache License 2.0. See [LICENSE](../LICENSE).
