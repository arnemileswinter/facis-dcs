#!/usr/bin/env bash
set -euo pipefail

: "${KUBECTL_BIN:?KUBECTL_BIN is required}"
: "${K8S_NAMESPACE:?K8S_NAMESPACE is required}"
: "${SERVICE_NAME:?SERVICE_NAME is required}"
: "${PORT_MAPPING:?PORT_MAPPING is required}"

OWNER_PID="${OWNER_PID:-$PPID}"
MAX_CONSECUTIVE_FAILURES="${MAX_CONSECUTIVE_FAILURES:-30}"
child_pid=""
cleanup() {
  if [[ -n "${child_pid}" ]]; then
    kill "${child_pid}" >/dev/null 2>&1 || true
    wait "${child_pid}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT
trap 'exit 143' TERM HUP
trap 'exit 130' INT

owner_is_alive() {
  kill -0 "${OWNER_PID}" >/dev/null 2>&1
}

# kubectl port-forward selects a backing pod even when the target is a
# Service. A rollout therefore ends that process; restart it until the BDD
# runner terminates this supervisor during cleanup. The owner check is also
# required: an abruptly cancelled Codex/shell tool may not execute the runner's
# EXIT trap, in which case a plain infinite loop would be reparented and live
# forever against a deleted cluster.
consecutive_failures=0
while owner_is_alive; do
  started_at="$(date +%s)"
  "${KUBECTL_BIN}" -n "${K8S_NAMESPACE}" port-forward "svc/${SERVICE_NAME}" "${PORT_MAPPING}" &
  child_pid=$!

  while kill -0 "${child_pid}" >/dev/null 2>&1; do
    if ! owner_is_alive; then
      exit 0
    fi
    sleep 1
  done
  wait "${child_pid}" || true
  child_pid=""

  runtime=$(( $(date +%s) - started_at ))
  if (( runtime >= 10 )); then
    consecutive_failures=0
  else
    consecutive_failures=$((consecutive_failures + 1))
  fi
  if (( consecutive_failures >= MAX_CONSECUTIVE_FAILURES )); then
    echo "port-forward supervisor: giving up after ${consecutive_failures} consecutive failures for svc/${SERVICE_NAME}" >&2
    exit 1
  fi

  backoff=$((1 << (consecutive_failures < 5 ? consecutive_failures : 5)))
  sleep "${backoff}"
done
