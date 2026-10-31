"""DCS restart (DCS-NFR-SF-01): the DCS deployment is restarted mid-flow and a
contract created before the restart is read and continued afterwards. All
contract state is in Postgres and IPFS, so the new pod needs no recovery.

The restart follows the ORCE restart step
(steps/audit_compliance/external_checkpoint_sink_steps.py step_worker_restarts).
The harness forwards and the HSM exec seam resolve the pod at each call
(tests/bdd/scripts/run_bdd_helm.sh), so they survive the new pod.
"""

import os
import subprocess
import time

import requests
from behave import when

from steps.support.services.contract_service import ContractService


def _backend_pods(kubectl, namespace, release):
    out = subprocess.run(
        [
            kubectl, "-n", namespace, "get", "pod",
            "-l", f"app.kubernetes.io/component=backend,app.kubernetes.io/instance={release}",
            "-o", "jsonpath={.items[*].metadata.name}",
        ],
        capture_output=True, text=True, timeout=30,
    )
    assert out.returncode == 0, f"Could not list DCS pods: {out.stderr}"
    return set(out.stdout.split())


@when("the DCS deployment restarts")
def step_dcs_restarts(context):
    namespace = os.getenv("K8S_NAMESPACE", "dcs-bdd")
    deployment = os.getenv("DCS_DEPLOYMENT", "dcs-digital-contracting-service")
    release = os.getenv("HELM_RELEASE", "dcs")
    kubectl = os.getenv("KUBECTL_BIN", "kubectl")

    before = _backend_pods(kubectl, namespace, release)
    restarted = subprocess.run(
        [kubectl, "-n", namespace, "rollout", "restart", f"deployment/{deployment}"],
        capture_output=True, text=True, timeout=30,
    )
    assert restarted.returncode == 0, (
        f"Could not restart DCS deployment {deployment!r}: {restarted.stderr}"
    )
    ready = subprocess.run(
        [kubectl, "-n", namespace, "rollout", "status", f"deployment/{deployment}", "--timeout=300s"],
        capture_output=True, text=True, timeout=310,
    )
    assert ready.returncode == 0, f"DCS did not become ready after restart: {ready.stderr}"
    new_pods = _backend_pods(kubectl, namespace, release) - before
    assert new_pods, f"The restart created no new DCS pod (before: {sorted(before)})"
    context.dcs_restart_pods = new_pods

    # The ingress follows the new endpoints shortly after the rollout ends.
    health = f"{context.base_url}/auth/login"
    deadline = time.monotonic() + 120
    status = None
    while time.monotonic() < deadline:
        try:
            status = requests.post(health, json={}, timeout=10).status_code
        except requests.RequestException:
            status = None
        if status is not None and status < 500 and status != 404:
            break
        time.sleep(2)
    else:
        raise AssertionError(f"DCS did not answer at {health} after the restart (last HTTP {status})")
    _wait_for_harness_forwards()


def _wait_for_harness_forwards():
    """A kubectl port-forward to the replaced pod fails the first connection
    after the rollout and then exits; keep_port_forward.sh starts a new one to
    the new pod. Use up that first failure here, so the next scenario that
    calls a forward gets a response."""
    for origin, path in (
        (os.getenv("BDD_DCS_INTERNAL_ORIGIN", ""), "/readyz"),
        (os.getenv("BDD_DCS_METRICS_ORIGIN", ""), "/metrics"),
    ):
        origin = origin.strip().rstrip("/")
        if not origin:
            continue
        deadline = time.monotonic() + 60
        while True:
            try:
                requests.get(origin + path, timeout=10)
                break
            except requests.RequestException as exc:
                if time.monotonic() >= deadline:
                    raise AssertionError(f"Harness forward {origin} did not reconnect after the restart: {exc}") from exc
                time.sleep(2)


@when('the creator submits contract "{name}" for review after the restart')
def step_submit_after_restart(context, name):
    # DRAFT -> NEGOTIATION -> SUBMITTED, both submits on the new pod.
    ContractService._prepare_contract_under_review(context, name)

