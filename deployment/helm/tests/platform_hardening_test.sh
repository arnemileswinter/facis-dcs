#!/usr/bin/env bash
# Render checks for the deployment hardening of the chart defaults and of the
# prod and acceptance profiles. Run: bash deployment/helm/tests/platform_hardening_test.sh
set -euo pipefail

CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# prod and acceptance refuse to render without these operator acknowledgements.
ACK=(--set federatedCatalogue.remote.acknowledgeAdminAllTrustBoundary=true)

render() { helm template dcs "$CHART_DIR" -n dcs "$@"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

# DCS-NFR-SEC-12: no fixed default credentials.
pin_a="$(render --show-only templates/hsm-pin-secret.yaml | grep 'PKCS11_PIN:')"
pin_b="$(render --show-only templates/hsm-pin-secret.yaml | grep 'PKCS11_PIN:')"
grep -q '"1234"' <<<"$pin_a" && fail "default PKCS#11 PIN is 1234"
[[ "$pin_a" != "$pin_b" ]] || fail "default PKCS#11 PIN is not random"
so_pin="$(render --show-only templates/hsm-provision-job.yaml | grep 'SO_PIN=')"
grep -q '"12345678"' <<<"$so_pin" && fail "default SO PIN is 12345678"
grep -q 'SO_PIN=""' <<<"$so_pin" && fail "default SO PIN is empty"
grep -rq 'YourStrongPassword' "$CHART_DIR/values.yaml" "$CHART_DIR/prom-values.yaml" && fail "fixed Grafana admin password in the chart values"
# An explicit PIN (values.bdd.yml) is used as given.
render -f "$CHART_DIR/values.bdd.yml" --show-only templates/hsm-pin-secret.yaml | grep -q 'PKCS11_PIN: "1234"' \
  || fail "explicit pkcs11.pin is not used"

# Signing needs a DSS to validate against (ADR-12): prod and acceptance get DSS_URL.
for profile in prod acceptance; do
  render -f "$CHART_DIR/values.$profile.yml" "${ACK[@]}" --show-only templates/deployment.yaml | grep -q 'name: DSS_URL' \
    || fail "values.$profile.yml gives the backend no DSS_URL"
done

# DCS-IR-CI-01 / DCS-NFR-SEC-01: TLS 1.3 minimum by default where Traefik terminates TLS.
traefik_prod="$(render -f "$CHART_DIR/values.prod.yml" "${ACK[@]}" --api-versions traefik.io/v1alpha1/TLSOption)"
grep -q 'kind: TLSOption' <<<"$traefik_prod" || fail "no TLSOption for a TLS ingress on Traefik"
grep -q 'minVersion: "VersionTLS13"' <<<"$traefik_prod" || fail "TLSOption minimum is not TLS 1.3"
grep -q 'router.tls.options: "dcs-dcs-digital-contracting-service-tls13@kubernetescrd"' <<<"$traefik_prod" \
  || fail "the ingress does not name the TLSOption"
# No TLSOption where Traefik is absent (the CRD would not exist) or the ingress serves no TLS (BDD).
render -f "$CHART_DIR/values.prod.yml" "${ACK[@]}" | grep -q 'kind: TLSOption' && fail "TLSOption without the Traefik CRD"
render -f "$CHART_DIR/values.bdd.yml" | grep -q 'router.tls.options' && fail "TLS option on a plain-HTTP ingress"
render "${ACK[@]}" --api-versions traefik.io/v1alpha1/TLSOption --set ingress.enabled=true \
  --set 'ingress.hosts[0].host=a.example' --set 'ingress.tls[0].hosts[0]=a.example' --set 'ingress.tls[0].secretName=t' \
  --set 'ingress.tlsOptions.cipherSuites[0]=TLS_AES_128_GCM_SHA256' >/dev/null || fail "TLSOption with cipherSuites does not render"

# DCS-IR-CI-09 / DCS-NFR-BR-06: access tokens live 5 minutes in prod and
# acceptance; the chart default keeps the Hydra default.
for profile in prod acceptance; do
  render -f "$CHART_DIR/values.$profile.yml" "${ACK[@]}" --show-only charts/hydra/templates/deployment.yaml \
    | grep -A1 'name: TTL_ACCESS_TOKEN$' | grep -q 'value: "5m"' || fail "values.$profile.yml: Hydra access-token TTL is not 5m"
done
render -f "$CHART_DIR/values.bdd.yml" --show-only charts/hydra/templates/deployment.yaml | grep -q TTL_ACCESS_TOKEN \
  && fail "access-token TTL set without hydra.config.accessTokenTTL"

# DCS-FR-CSA-14 / DCS-FR-CSA-19: archive retention policy of 10 years in prod
# and acceptance; none by default.
for profile in prod acceptance; do
  render -f "$CHART_DIR/values.$profile.yml" "${ACK[@]}" --show-only templates/deployment.yaml \
    | grep -A1 'name: DCS_ARCHIVE_RETENTION_YEARS$' | grep -q 'value: "10"' || fail "values.$profile.yml: no 10-year archive retention"
done
render "${ACK[@]}" --show-only templates/deployment.yaml | grep -q DCS_ARCHIVE_RETENTION_YEARS \
  && fail "archive retention set without archive.retentionYears"

# DCS-NFR-PER-03: liveness probe and, with more than one replica, a PodDisruptionBudget.
prod="$(render -f "$CHART_DIR/values.prod.yml" "${ACK[@]}")"
grep -A4 'livenessProbe:' <<<"$prod" | grep -q 'path: /livez' || fail "no liveness probe on /livez"
grep -A4 'startupProbe:' <<<"$prod" | grep -q 'path: /livez' || fail "no startup probe on /livez"
grep -q 'kind: PodDisruptionBudget' <<<"$prod" || fail "no PodDisruptionBudget for 3 replicas"
render "${ACK[@]}" | grep -q 'kind: PodDisruptionBudget' && fail "PodDisruptionBudget for a single replica blocks drains"

# Acceptance runs 2 backend pods on the ReadWriteOnce SoftHSM2 token volume: they
# must share one node, and the budget must let a drain evict both.
render -f "$CHART_DIR/values.acceptance.yml" "${ACK[@]}" | python3 -c '
import sys, yaml
docs = [d for d in yaml.safe_load_all(sys.stdin) if d]
dep = next(d for d in docs if d["kind"] == "Deployment" and d["metadata"]["name"] == "dcs-digital-contracting-service")
pod = dep["spec"]["template"]
aff = pod["spec"].get("affinity") or {}
terms = (aff.get("podAffinity") or {}).get("requiredDuringSchedulingIgnoredDuringExecution") or []
ok = any(t.get("topologyKey") == "kubernetes.io/hostname"
         and all(pod["metadata"]["labels"].get(k) == v for k, v in t["labelSelector"]["matchLabels"].items())
         for t in terms)
assert ok, "acceptance backend pods are not required on the node of the other backend pods"
assert "podAntiAffinity" not in aff, "acceptance backend pods carry an anti-affinity"
assert any(c["name"] == "hsm-token" and c["persistentVolumeClaim"] for c in pod["spec"]["volumes"] if "persistentVolumeClaim" in c), "no shared token volume"
job = next(d for d in docs if d["kind"] == "Job" and d["metadata"]["name"] == "dcs-digital-contracting-service-hsm-provision")
assert job["spec"]["template"]["spec"].get("affinity") == aff, "the provisioning Job can land on another node than the token volume"
pdb = next(d for d in docs if d["kind"] == "PodDisruptionBudget")
assert pdb["spec"]["maxUnavailable"] >= dep["spec"]["replicas"], "PodDisruptionBudget blocks the drain of the shared node"
' || fail "acceptance backend pods are not pinned to the node of their token volume"

# DCS-NFR-SF-03: scheduled database backup in prod and acceptance, off by default.
for profile in prod acceptance; do
  backup="$(render -f "$CHART_DIR/values.$profile.yml" "${ACK[@]}" --show-only templates/backup-cronjob.yaml 2>/dev/null || true)"
  grep -q 'kind: CronJob' <<<"$backup" || fail "values.$profile.yml has no backup CronJob"
  grep -q 'pg_dump --format=custom --dbname="$DATABASE_URL"' <<<"$backup" || fail "backup does not dump DATABASE_URL"
  grep -A3 'name: DATABASE_URL' <<<"$backup" | grep -q "name: \"dcs-$profile-db-credentials\"" \
    || fail "backup does not read the $profile database Secret"
done
render "${ACK[@]}" | grep -q 'db-backup' && fail "backup rendered while backup.enabled is false"

# DCS-NFR-SEC-04: configIntegrity.pins reach the backend as DCS_CONFIG_SHA256_PINS.
a=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
b=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
pins="$(render -f "$CHART_DIR/values.bdd.yml" --set "configIntegrity.pins.x5c-trust-anchors-poa=$a" \
  --set "configIntegrity.pins.did-document=$b" --show-only templates/deployment.yaml | grep -A1 'name: DCS_CONFIG_SHA256_PINS$')"
grep -q "value: \"did-document=$b,x5c-trust-anchors-poa=$a\"" <<<"$pins" || fail "configIntegrity.pins not rendered: $pins"
render -f "$CHART_DIR/values.bdd.yml" --show-only templates/deployment.yaml | grep -q DCS_CONFIG_SHA256_PINS \
  && fail "empty pins rendered"

# DCS-NFR-SEC-11: alert rules where the Prometheus Operator runs.
rules="$(render "${ACK[@]}" --set backup.enabled=true --api-versions monitoring.coreos.com/v1/PrometheusRule --show-only templates/prometheusrule.yaml)"
for alert in DCSBackendDown DCSServerErrors DCSAuthFailureSpike DCSSlowResponses DCSBackendRestarting DCSBackupFailed; do
  grep -q "alert: $alert" <<<"$rules" || fail "no $alert alert rule"
done
grep -q 'expr: up{namespace="dcs", service="dcs-digital-contracting-service-metrics"} == 0' <<<"$rules" \
  || fail "DCSBackendDown does not select the backend metrics service"
render "${ACK[@]}" | grep -q 'kind: PrometheusRule' && fail "PrometheusRule without the CRD"

echo 'platform hardening render tests passed'
