#!/usr/bin/env bash
# Render checks for the log store (DCS-NFR-SEC-09): retention, TLS and access
# only through the collector and Grafana. Needs helm, python3 with PyYAML.
# Run: bash deployment/helm/tests/log_store_test.sh
set -euo pipefail

CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ACK=(--set federatedCatalogue.remote.acknowledgeAdminAllTrustBoundary=true)
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

render() { helm template dcs "$CHART_DIR" -n dcs "$@"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

# Prints one field of one rendered object: obj <file> <kind> <name> <python expression on x>
obj() {
  python3 - "$@" <<'EOF'
import sys, yaml
f, kind, name, expr = sys.argv[1:5]
for x in yaml.safe_load_all(open(f)):
    if x and x['kind'] == kind and x['metadata']['name'] == name:
        print(eval(expr))
        break
else:
    sys.exit('no %s %s' % (kind, name))
EOF
}
B=dcs-digital-contracting-service
render "${ACK[@]}" | grep -q 'name: dcs-loki' && fail "log store rendered while logging.enabled is false"
for profile in prod acceptance; do
  render -f "$CHART_DIR/values.$profile.yml" "${ACK[@]}" >"$TMP/$profile.yaml"
  p="$TMP/$profile.yaml"
  [[ "$(obj "$p" StatefulSet dcs-loki "x['spec']['replicas']")" == 1 ]] || fail "$profile has no Loki"
  cfg="$(obj "$p" ConfigMap dcs-loki "x['data']['loki.yaml']")"
  grep -q 'retention_period: 2160h' <<<"$cfg" || fail "$profile: Loki without retention period"
  grep -q 'retention_enabled: true' <<<"$cfg" || fail "$profile: Loki compactor does not delete expired logs"
  grep -q 'tls_min_version: VersionTLS13' <<<"$cfg" || fail "$profile: Loki without TLS 1.3"
  [[ "$(obj "$p" Service dcs-loki "x['spec']['type']")" == ClusterIP ]] || fail "$profile: Loki exposed outside the cluster"
  [[ "$(obj "$p" NetworkPolicy dcs-loki "[f.get('podSelector', {}).get('matchLabels', {}).get('app.kubernetes.io/name') for r in x['spec']['ingress'] for f in r['from']]")" == "['log-collector', 'grafana']" ]] \
    || fail "$profile: Loki reachable by pods other than the collector and Grafana"
  ds="$(obj "$p" ConfigMap $B-loki-datasource "x['metadata']['labels']['grafana_datasource'] + ' ' + x['data']['loki-datasource.yaml']")"
  grep -q '^1 ' <<<"$ds" || fail "$profile: data source not labelled for the Grafana sidecar"
  grep -q 'url: https://dcs-loki.dcs.svc:3100' <<<"$ds" || fail "$profile: Grafana reads Loki without TLS"
  obj "$p" Secret dcs-internal-tls "__import__('base64').b64decode(x['data']['ca.crt']).decode().strip()" >"$TMP/pca"
  python3 -c "import sys,yaml; d=yaml.safe_load(sys.stdin.read().split(' ',1)[1]); print(d['datasources'][0]['secureJsonData']['tlsCACert'].strip())" <<<"$ds" >"$TMP/dca"
  cmp -s "$TMP/pca" "$TMP/dca" || fail "$profile: Grafana data source trusts another CA than the server certificate's"
  alloy="$(obj "$p" ConfigMap dcs-log-collector "x['data']['config.alloy']")"
  grep -q 'url = "https://dcs-loki:3100/loki/api/v1/push"' <<<"$alloy" || fail "$profile: collector pushes without TLS"
  grep -q 'names = \["dcs"\]' <<<"$alloy" || fail "$profile: collector not limited to the release namespace"
  [[ "$(obj "$p" RoleBinding dcs-log-collector "x['roleRef']['kind']")" == Role ]] || fail "$profile: collector has cluster-wide rights"
done
render -f "$CHART_DIR/values.prod.yml" "${ACK[@]}" --set logging.retention=720h --show-only charts/logging/templates/loki-config.yaml \
  | grep -q 'retention_period: 720h' || fail "logging.retention is not used"

echo 'log store render tests passed'
