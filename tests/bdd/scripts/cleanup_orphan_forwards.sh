#!/usr/bin/env bash
set -euo pipefail

: "${HARNESS_DIR:?HARNESS_DIR is required}"
: "${K8S_NAMESPACE:?K8S_NAMESPACE is required}"
: "${HELM_RELEASE:?HELM_RELEASE is required}"

supervisor_pattern="^bash ${HARNESS_DIR}/scripts/keep_port_forward\\.sh$"
mapfile -t supervisor_pids < <(pgrep -f -- "$supervisor_pattern" || true)

if (( ${#supervisor_pids[@]} > 0 )); then
  echo "Stopping ${#supervisor_pids[@]} stale BDD port-forward supervisor(s)"
  kill -TERM "${supervisor_pids[@]}" >/dev/null 2>&1 || true
  sleep 1
  for pid in "${supervisor_pids[@]}"; do
    if kill -0 "$pid" >/dev/null 2>&1; then
      # Supervisors from before the owner-liveness fix trapped TERM without
      # exiting. KILL is restricted to the exact repository script path.
      kill -KILL "$pid" >/dev/null 2>&1 || true
    fi
  done
fi

# A supervisor that itself had to be killed cannot reap its current kubectl
# child. Limit cleanup to the two restart-supervised services of this release
# and namespace; direct DB/DCS forwards are handled through run-specific PIDs.
kubectl_pattern="kubectl[^ ]*( [^ ]+)* -n ${K8S_NAMESPACE} port-forward svc/${HELM_RELEASE}-(dss|orce)( |$)"
mapfile -t kubectl_pids < <(pgrep -f -- "$kubectl_pattern" || true)
if (( ${#kubectl_pids[@]} > 0 )); then
  echo "Stopping ${#kubectl_pids[@]} stale kubectl port-forward process(es)"
  kill -TERM "${kubectl_pids[@]}" >/dev/null 2>&1 || true
fi
