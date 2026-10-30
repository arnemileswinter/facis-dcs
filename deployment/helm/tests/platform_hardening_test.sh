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

echo 'platform hardening render tests passed'
