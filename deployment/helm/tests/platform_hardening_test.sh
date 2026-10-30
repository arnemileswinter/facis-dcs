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

# DCS-NFR-PER-03: liveness probe and, with more than one replica, a PodDisruptionBudget.
prod="$(render -f "$CHART_DIR/values.prod.yml" "${ACK[@]}")"
grep -A4 'livenessProbe:' <<<"$prod" | grep -q 'path: /livez' || fail "no liveness probe on /livez"
grep -A4 'startupProbe:' <<<"$prod" | grep -q 'path: /livez' || fail "no startup probe on /livez"
grep -q 'kind: PodDisruptionBudget' <<<"$prod" || fail "no PodDisruptionBudget for 3 replicas"
render "${ACK[@]}" | grep -q 'kind: PodDisruptionBudget' && fail "PodDisruptionBudget for a single replica blocks drains"

# DCS-NFR-SF-03: scheduled database backup in prod and acceptance, off by default.
for profile in prod acceptance; do
  backup="$(render -f "$CHART_DIR/values.$profile.yml" "${ACK[@]}" --show-only templates/backup-cronjob.yaml 2>/dev/null || true)"
  grep -q 'kind: CronJob' <<<"$backup" || fail "values.$profile.yml has no backup CronJob"
  grep -q 'pg_dump --format=custom --dbname="$DATABASE_URL"' <<<"$backup" || fail "backup does not dump DATABASE_URL"
  grep -A3 'name: DATABASE_URL' <<<"$backup" | grep -q "name: \"dcs-$profile-db-credentials\"" \
    || fail "backup does not read the $profile database Secret"
done
render "${ACK[@]}" | grep -q 'db-backup' && fail "backup rendered while backup.enabled is false"

echo 'platform hardening render tests passed'
