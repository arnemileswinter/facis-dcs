#!/usr/bin/env bash
# Render checks for in-cluster TLS (DCS-NFR-SEC-14, DCS-NFR-SEC-01) and the
# generated PostgreSQL password (DCS-NFR-SEC-12).
# Needs helm, openssl and python3 with PyYAML.
# Run: bash deployment/helm/tests/in_cluster_security_test.sh
set -euo pipefail

CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ACK=(--set federatedCatalogue.remote.acknowledgeAdminAllTrustBoundary=true)
ALL=(--set postgresql.enabled=true --set nats.enabled=true --set ipfs.enabled=true --set dss.enabled=true
  --set orce.enabled=true --set global.archiveAuditToken=t --set backup.enabled=true)
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
env_of() { # env_of <file> <kind> <name> <container> <env name>
  obj "$1" "$2" "$3" "next(((e.get('value') or str(e.get('valueFrom'))) for c in (x['spec']['template'] if 'template' in x['spec'] else x['spec']['jobTemplate']['spec']['template'])['spec']['containers'] if c['name'] == '$4' for e in c.get('env', []) if e['name'] == '$5'), None)"
}
B=dcs-digital-contracting-service
C=digital-contracting-service

# ---- DCS-NFR-SEC-12: the co-deployed PostgreSQL has no fixed password -------
render "${ALL[@]}" >"$TMP/a.yaml"
render "${ALL[@]}" >"$TMP/b.yaml"
pw_a="$(obj "$TMP/a.yaml" Secret dcs-postgresql "x['stringData']['postgres-password']")"
pw_b="$(obj "$TMP/b.yaml" Secret dcs-postgresql "x['stringData']['postgres-password']")"
[[ "$pw_a" != dcs && ${#pw_a} -ge 32 ]] || fail "default postgres password is fixed or short: $pw_a"
[[ "$pw_a" != "$pw_b" ]] || fail "default postgres password is not random"
grep -q "password=" <<<"$(env_of "$TMP/a.yaml" Deployment $B $C DATABASE_URL)" && fail "generated password in DATABASE_URL"
for w in "Deployment $B $C" "CronJob $B-db-backup pg-dump" "CronJob $B-pac-monitor pac-monitor"; do
  read -r kind name ctr <<<"$w"
  pg="$(env_of "$TMP/a.yaml" "$kind" "$name" "$ctr" PGPASSWORD)"
  grep -q "'name': 'dcs-postgresql', 'key': 'postgres-password'" <<<"$pg" || fail "$name gets no PGPASSWORD from the postgresql Secret: $pg"
done
# An explicit password (values.bdd.yml) is used as given.
render -f "$CHART_DIR/values.bdd.yml" >"$TMP/bdd.yaml"
[[ "$(obj "$TMP/bdd.yaml" Secret dcs-postgresql "x['stringData']['postgres-password']")" == dcs ]] || fail "explicit postgres password not used"
[[ "$(env_of "$TMP/bdd.yaml" Deployment $B $C DATABASE_URL)" == "host=dcs-postgresql port=5432 user=dcs password=dcs dbname=dcs sslmode=verify-full sslrootcert=/etc/dcs-internal-tls/ca.crt" ]] \
  || fail "BDD DATABASE_URL changed: $(env_of "$TMP/bdd.yaml" Deployment $B $C DATABASE_URL)"
[[ "$(env_of "$TMP/bdd.yaml" Deployment $B $C PGPASSWORD)" == None ]] || fail "PGPASSWORD with an explicit password"

# ---- DCS-NFR-SEC-14 / DCS-NFR-SEC-01: TLS for in-cluster traffic, on by default --
f="$TMP/a.yaml"
obj "$f" Secret dcs-internal-tls "__import__('base64').b64decode(x['data']['ca.crt']).decode()" >"$TMP/ca.crt"
obj "$f" Secret dcs-internal-tls "__import__('base64').b64decode(x['data']['tls.crt']).decode()" >"$TMP/tls.crt"
obj "$f" Secret dcs-internal-ca "__import__('base64').b64decode(x['data']['ca.crt']).decode()" >"$TMP/ca2.crt"
cmp -s "$TMP/ca.crt" "$TMP/ca2.crt" || fail "server Secret carries another CA"
openssl verify -CAfile "$TMP/ca.crt" "$TMP/tls.crt" >/dev/null || fail "server certificate not issued by the internal CA"
openssl x509 -in "$TMP/tls.crt" -noout -text | grep -q 'id-ecPublicKey' || fail "server key is not ECDSA"
sans="$(openssl x509 -in "$TMP/tls.crt" -noout -ext subjectAltName)"
for h in dcs-postgresql dcs-nats dcs-ipfs dcs-dss dcs-orce dcs-loki dcs-hydra $B $B-pdf-core dcs-postgresql.dcs.svc.cluster.local; do
  grep -q "DNS:$h\(,\|$\)" <<<"$sans" || fail "server certificate does not name $h"
done

# PostgreSQL: TLS 1.3, TCP only with TLS; the backend verifies the server name.
pg_args="$(obj "$f" Deployment dcs-postgresql "x['spec']['template']['spec']['containers'][0]['args'][0]")"
for a in "ssl=on" "ssl_min_protocol_version=TLSv1.3" "ssl_cert_file=/etc/dcs-internal-tls/tls.crt" "hba_file=/etc/postgresql-hba/pg_hba.conf"; do
  grep -q -- "-c $a" <<<"$pg_args" || fail "postgresql runs without -c $a"
done
hba="$(obj "$f" ConfigMap dcs-postgresql-hba "x['data']['pg_hba.conf']")"
grep -q '^hostssl all all all scram-sha-256$' <<<"$hba" || fail "pg_hba has no hostssl rule"
grep -q '^host ' <<<"$hba" && fail "pg_hba accepts TCP without TLS"
[[ "$(env_of "$f" Deployment $B $C DATABASE_URL)" == "host=dcs-postgresql port=5432 user=dcs dbname=dcs sslmode=verify-full sslrootcert=/etc/dcs-internal-tls/ca.crt" ]] \
  || fail "DATABASE_URL does not verify the server: $(env_of "$f" Deployment $B $C DATABASE_URL)"

# NATS: TLS 1.3 required.
grep -q 'min_version: "1.3"' <<<"$(obj "$f" ConfigMap dcs-nats "x['data']['nats.conf']")" || fail "NATS without TLS 1.3 minimum"
[[ "$(obj "$f" Deployment dcs-nats "x['spec']['template']['spec']['containers'][0]['args']")" == "['-js', '-c', '/etc/nats/nats.conf']" ]] \
  || fail "NATS does not load its TLS config"
[[ "$(env_of "$f" Deployment $B $C NATS_URL)" == tls://dcs-nats:4222 ]] || fail "NATS_URL is not tls://"

# HTTP components: TLS proxy in each pod, the backend calls it and trusts the CA.
# The backend itself has one for in-cluster callers (ORCE).
for d in dcs-ipfs dcs-dss dcs-orce $B-pdf-core $B; do
  proxy="$(obj "$f" Deployment "$d" "next((c['args'][0] for c in x['spec']['template']['spec']['containers'] if c['name'] == 'tls-proxy'), None)")"
  grep -q 'ssl_protocols TLSv1.3;' <<<"$proxy" || fail "$d has no TLS 1.3 proxy"
  [[ "$(obj "$f" Service "$d" "[p['port'] for p in x['spec']['ports'] if p['name'] == 'https']")" == "[8443]" ]] || fail "service $d has no https port"
done
[[ "$(env_of "$f" Deployment $B $C IPFS_MFS_BASE_URL)" == https://dcs-ipfs:8443 ]] || fail "IPFS called without TLS"
[[ "$(env_of "$f" Deployment $B $C DSS_URL)" == https://dcs-dss:8443 ]] || fail "DSS called without TLS"
[[ "$(env_of "$f" Deployment $B $C PDF_CORE_URL)" == https://$B-pdf-core:8443 ]] || fail "pdf-core called without TLS"
for v in ORCE_ARCHIVE_NOTARY_URL ORCE_ARCHIVE_AUDIT_LOG_URL PAC_AUDIT_EXECUTOR_URL PAC_WORKFLOW_GATE_EXECUTOR_URL; do
  [[ "$(env_of "$f" Deployment $B $C $v)" == https://dcs-orce:8443/* ]] || fail "$v without TLS"
done
[[ "$(obj "$f" Service $B "[p['port'] for p in x['spec']['ports'] if p['name'] == 'http']")" == "[8991]" ]] || fail "backend http port (ingress, probes) removed"
# ORCE calls the backend and Hydra over https and trusts the internal CA.
[[ "$(env_of "$f" Deployment dcs-orce orce NODE_EXTRA_CA_CERTS)" == /etc/dcs-internal-tls/ca.crt ]] || fail "ORCE does not trust the internal CA"
obj "$f" Deployment dcs-orce "next(c for c in x['spec']['template']['spec']['containers'] if c['name'] == 'orce')['volumeMounts']" \
  | grep -q "'name': 'internal-ca', 'mountPath': '/etc/dcs-internal-tls'" || fail "ORCE does not mount the internal CA"
# The ontology and context IRIs stay the service identity.
[[ "$(env_of "$f" Deployment $B $C PDF_CORE_CONTEXT_IRI)" == http://$B-pdf-core:8080/ontology/dcs-pdf-core ]] || fail "context IRI changed"
[[ "$(env_of "$f" Deployment $B $C SSL_CERT_DIR)" == /etc/ssl/certs:/etc/dcs-internal-tls ]] || fail "backend does not trust the internal CA"
for w in "Deployment $B" "CronJob $B-db-backup" "CronJob $B-pac-monitor"; do
  read -r kind name <<<"$w"
  obj "$f" "$kind" "$name" "(x['spec']['template'] if 'template' in x['spec'] else x['spec']['jobTemplate']['spec']['template'])['spec']['containers'][0]['volumeMounts']" \
    | grep -q "'name': 'internal-ca', 'mountPath': '/etc/dcs-internal-tls'" || fail "$name does not mount the internal CA"
done

# A later upgrade keeps the CA: the template reads it back from the cluster.
grep -q 'lookup "v1" "Secret" .Release.Namespace (printf "%s-internal-ca" .Release.Name)' "$CHART_DIR/templates/_helpers.tpl" \
  || fail "internal CA not read back on upgrade"

# Off: no certificates and the plain endpoints.
render "${ALL[@]}" --set global.internalTLS.enabled=false >"$TMP/off.yaml"
grep -q 'internal-tls' "$TMP/off.yaml" && fail "internal TLS rendered while global.internalTLS.enabled=false"
[[ "$(env_of "$TMP/off.yaml" Deployment $B $C NATS_URL)" == nats://dcs-nats:4222 ]] || fail "NATS_URL without TLS changed"
[[ "$(env_of "$TMP/off.yaml" Deployment $B $C IPFS_MFS_BASE_URL)" == http://dcs-ipfs:5001 ]] || fail "IPFS URL without TLS changed"
grep -q 'name: tls-proxy' "$TMP/off.yaml" && fail "TLS proxy rendered while global.internalTLS.enabled=false"

# The kind CI (BDD and e2e: values.bdd.yml, values.bdd2.yml) runs the TLS paths
# of prod, with no XFSC statuslist-service.
for prof in "bdd dcs" "bdd2 dcs2"; do
  read -r v r <<<"$prof"
  helm template "$r" "$CHART_DIR" -n dcs -f "$CHART_DIR/values.$v.yml" >"$TMP/ci-$v.yaml"
  f="$TMP/ci-$v.yaml"
  rb="$r-digital-contracting-service"
  obj "$f" Secret "$r-internal-tls" "x['type']" >/dev/null || fail "values.$v.yml: no internal TLS"
  [[ "$(env_of "$f" Deployment "$rb" $C DATABASE_URL)" == *"sslmode=verify-full sslrootcert=/etc/dcs-internal-tls/ca.crt" ]] || fail "values.$v.yml: database without TLS"
  [[ "$(env_of "$f" Deployment "$rb" $C NATS_URL)" == "tls://$r-nats:4222" ]] || fail "values.$v.yml: NATS without TLS"
  [[ "$(env_of "$f" Deployment "$rb" $C IPFS_MFS_BASE_URL)" == "https://$r-ipfs:8443" ]] || fail "values.$v.yml: IPFS without TLS"
  [[ "$(env_of "$f" Deployment "$rb" $C PDF_CORE_URL)" == "https://$rb-pdf-core:8443" ]] || fail "values.$v.yml: pdf-core without TLS"
  [[ "$(env_of "$f" Deployment "$r-hydra" hydra DSN)" == *"@$r-postgresql:5432/hydra?sslmode=verify-full&sslrootcert=/etc/dcs-internal-tls/ca.crt" ]] \
    || fail "values.$v.yml: Hydra reaches the database without TLS"
  grep -q 'name: statuslist-service' "$f" && fail "values.$v.yml deploys the XFSC statuslist-service"
  # Hydra admin API: TLS 1.3 proxy in the pod, called over https by the backend
  # and the client registration job.
  grep -q 'ssl_protocols TLSv1.3;' <<<"$(obj "$f" Deployment "$r-hydra" "next((c['args'][0] for c in x['spec']['template']['spec']['containers'] if c['name'] == 'tls-proxy'), None)")" \
    || fail "values.$v.yml: Hydra has no TLS 1.3 proxy"
  [[ "$(obj "$f" Service "$r-hydra" "[p['port'] for p in x['spec']['ports'] if p['name'] == 'https']")" == "[8443]" ]] || fail "values.$v.yml: Hydra service has no https port"
  [[ "$(env_of "$f" Deployment "$rb" $C HYDRA_ADMIN_URL)" == "https://$r-hydra:8443" ]] || fail "values.$v.yml: backend calls the Hydra admin API without TLS"
  [[ "$(env_of "$f" Job "$r-hydra-clients" register-clients HYDRA_ADMIN)" == "https://$r-hydra:8443" ]] || fail "values.$v.yml: client registration without TLS"
  [[ "$(env_of "$f" Job "$r-hydra-clients" register-clients CURL_CA_BUNDLE)" == /etc/dcs-internal-tls/ca.crt ]] || fail "values.$v.yml: client registration does not verify Hydra"
  proxy="$(obj "$f" Deployment "$rb" "next((c['readinessProbe']['httpGet']['path'] for c in x['spec']['template']['spec']['containers'] if c['name'] == 'statuslist-localhost-proxy'), None)")"
  [[ "$proxy" == /issuer/status-list/1 ]] || fail "values.$v.yml: localhost proxy readiness depends on $proxy"
done
for v in DSS_URL PAC_AUDIT_EXECUTOR_URL PAC_WORKFLOW_GATE_EXECUTOR_URL TSA_URL DCS_TRUST_PDP_URL ORCE_ARCHIVE_NOTARY_URL; do
  [[ "$(env_of "$TMP/ci-bdd.yaml" Deployment $B $C $v)" == https://dcs-*:8443* ]] || fail "values.bdd.yml: $v without TLS"
done
[[ "$(env_of "$TMP/ci-bdd.yaml" Deployment dcs-orce orce DCS_URL)" == https://$B:8443/* ]] || fail "values.bdd.yml: ORCE calls the backend without TLS"

# The issuer release renders the orce chart on its own, without the parent.
helm template dcs-issuer "$CHART_DIR/charts/orce" -f "$CHART_DIR/values.issuer-base.yml" -f "$CHART_DIR/values.issuer.bdd.yml" \
  | grep -q 'tls-proxy' && fail "standalone orce chart renders the TLS proxy"

echo 'in-cluster security render tests passed'
