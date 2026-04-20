"""OIDC token helpers for executable BDD scenarios.

Role token resolution order:
1) BDD_OIDC_TOKEN_<ROLE>
2) BDD_OIDC_ROLE_TOKENS_JSON
3) BDD_OIDC_TOKEN
4) Role-specific command: BDD_OIDC_TOKEN_CMD_<ROLE>
5) Generic command: BDD_OIDC_TOKEN_CMD
6) Optional exchange endpoint: BDD_OIDC_TOKEN_EXCHANGE_URL
"""

import json
import os
import re
import subprocess

import requests


def oidc_base_url() -> str:
    return os.getenv("BDD_OIDC_BASE_URL", "http://127.0.0.1:18080").rstrip("/")


def _role_token_env_key(role: str) -> str:
    role_key = re.sub(r"[^A-Za-z0-9]+", "_", role.upper()).strip("_")
    return f"BDD_OIDC_TOKEN_{role_key}"


def _role_token_cmd_env_key(role: str) -> str:
    role_key = re.sub(r"[^A-Za-z0-9]+", "_", role.upper()).strip("_")
    return f"BDD_OIDC_TOKEN_CMD_{role_key}"


def _token_from_json_map(role: str) -> str:
    raw = os.getenv("BDD_OIDC_ROLE_TOKENS_JSON", "").strip()
    if not raw:
        return ""
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError as err:
        raise AssertionError(f"Invalid BDD_OIDC_ROLE_TOKENS_JSON value: {err}") from err
    if not isinstance(payload, dict):
        raise AssertionError("BDD_OIDC_ROLE_TOKENS_JSON must be a JSON object mapping role names to tokens")
    token = payload.get(role, "")
    if not token:
        token = payload.get(_role_token_env_key(role), "")
    if token and not isinstance(token, str):
        raise AssertionError(f"Token entry for role '{role}' in BDD_OIDC_ROLE_TOKENS_JSON must be a string")
    return token or ""


def _run_token_command(command: str, role: str, client_id: str, username_prefix: str) -> str:
    env = dict(os.environ)
    env["BDD_OIDC_ROLE"] = role
    env["BDD_OIDC_CLIENT_ID"] = client_id
    env["BDD_OIDC_USERNAME_PREFIX"] = username_prefix
    env["BDD_OIDC_BASE_URL"] = oidc_base_url()

    result = subprocess.run(command, shell=True, check=False, capture_output=True, text=True, env=env)
    if result.returncode != 0:
        raise AssertionError(
            "OIDC token command failed for role "
            f"'{role}' (exit={result.returncode}): {result.stderr.strip() or result.stdout.strip()}"
        )

    token = result.stdout.strip().splitlines()[0] if result.stdout.strip() else ""
    if not token:
        raise AssertionError(f"OIDC token command produced no token output for role '{role}'")
    return token


def _fake_credential_for_role(role: str, username_prefix: str) -> dict:
    raw = os.getenv("BDD_OIDC_FAKE_CREDENTIALS_JSON", "").strip()
    if raw:
        try:
            payload = json.loads(raw)
        except json.JSONDecodeError as err:
            raise AssertionError(f"Invalid BDD_OIDC_FAKE_CREDENTIALS_JSON value: {err}") from err
        if not isinstance(payload, dict):
            raise AssertionError("BDD_OIDC_FAKE_CREDENTIALS_JSON must be a JSON object")
        role_payload = payload.get(role)
        if role_payload is not None:
            if not isinstance(role_payload, dict):
                raise AssertionError(f"Credential entry for role '{role}' must be a JSON object")
            return role_payload
    role_safe = re.sub(r"[^A-Za-z0-9]+", "-", role.lower()).strip("-")
    return {
        "sub": f"{username_prefix}-{role_safe}",
        "roles": [role],
        "credential_type": "bdd-fake",
        "issuer": "ocm-w-bdd",
    }


def _exchange_token_via_endpoint(role: str, client_id: str, username_prefix: str) -> str:
    exchange_url = os.getenv("BDD_OIDC_TOKEN_EXCHANGE_URL", "").strip()
    if not exchange_url:
        return ""

    payload = {
        "role": role,
        "client_id": client_id,
        "fake_credential": _fake_credential_for_role(role, username_prefix),
    }
    response = requests.post(exchange_url, json=payload, timeout=20)
    if response.status_code != 200:
        raise AssertionError(f"OIDC token exchange failed for role '{role}': {response.status_code} {response.text}")

    data = response.json()
    if not isinstance(data, dict):
        raise AssertionError("OIDC token exchange response must be a JSON object")

    token = data.get("access_token") or data.get("token")
    if not token or not isinstance(token, str):
        raise AssertionError("OIDC token exchange response missing access_token/token string")
    return token


def token_for_role(role: str, client_id: str, username_prefix: str = "bdd") -> str:
    """Return a role token from env vars, token command, or exchange endpoint."""
    direct = os.getenv(_role_token_env_key(role), "").strip()
    if direct:
        return direct

    mapped = _token_from_json_map(role)
    if mapped:
        return mapped

    fallback = os.getenv("BDD_OIDC_TOKEN", "").strip()
    if fallback:
        return fallback

    role_cmd = os.getenv(_role_token_cmd_env_key(role), "").strip()
    if role_cmd:
        return _run_token_command(role_cmd, role, client_id, username_prefix)

    generic_cmd = os.getenv("BDD_OIDC_TOKEN_CMD", "").strip()
    if generic_cmd:
        return _run_token_command(generic_cmd, role, client_id, username_prefix)

    endpoint_token = _exchange_token_via_endpoint(role, client_id, username_prefix)
    if endpoint_token:
        return endpoint_token

    raise AssertionError(
        "No OIDC token source configured for role "
        f"'{role}'. Set {_role_token_env_key(role)} or BDD_OIDC_ROLE_TOKENS_JSON, "
        "or configure BDD_OIDC_TOKEN_CMD / BDD_OIDC_TOKEN_EXCHANGE_URL."
    )