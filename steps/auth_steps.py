"""Authentication and scenario setup steps for executable BDD scenarios."""

import json
import os
import re
import base64

from behave import given

from support.template_utils import template_env_key


def _set_headers_for_role(context, role: str, username_prefix: str = "bdd"):
    client_id = "dcs-client"
    role_safe = re.sub(r"[^A-Za-z0-9]+", "-", role.lower()).strip("-")
    username = f"{username_prefix}-{role_safe}"

    token = create_custom_jwt(client_id, username, role)
    context.headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }


def create_custom_jwt(client_id, username, role):
    header = {"alg": "none"}
    payload = {"sub": username, "iss": "https://auth.eclipse.org/auth/realms/community", "azp": client_id, "resource_access": {"dcs-client": {"roles": [role]}}, "exp": 9999999999}

    encoded_header = base64.urlsafe_b64encode(json.dumps(header).encode()).decode().rstrip("=")
    encoded_payload = base64.urlsafe_b64encode(json.dumps(payload).encode()).decode().rstrip("=")

    token = f"{encoded_header}.{encoded_payload}."
    return token

if __name__ == "__main__":
    class MockContext:
        "simple dict offering dot member access and settable from outside"
        def __getattr__(self, key):
            return self.__dict__.get(key)
        def __setattr__(self, key, value):
            self.__dict__[key] = value
    ctx = MockContext()
    _set_headers_for_role(ctx, "Template Creator")
    print(ctx.headers["Authorization"])

@given('I am authenticated with role "{role}"')
def step_given_authenticated_with_role(context, role):
    _set_headers_for_role(context, role)


@given('a system service is authenticated via API with role "{role}"')
def step_given_authenticated_service_with_role(context, role):
    _set_headers_for_role(context, role, username_prefix="bdd-service")


@given("a system service is authenticated via API")
def step_given_authenticated_service(context):
    token = os.getenv("BDD_DCS_TOKEN")
    assert token, "BDD_DCS_TOKEN must be set for authenticated API scenarios"
    context.headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }


@given("a system service provides an invalid API key")
def step_given_invalid_api_key(context):
    context.headers = {
        "Authorization": "Bearer invalid-token",
        "Content-Type": "application/json",
    }


@given('template "{template_name}" is available')
def step_given_template_available(context, template_name):
    env_key = template_env_key(template_name)
    template_did = os.getenv(env_key)
    if not template_did:
        from template_workflow_steps import (  # noqa: PLC0415
            _create_approved_template,
            _store_named,
        )

        did, updated_at = _create_approved_template(context)
        template_did = did
        _store_named(context, template_name, did, updated_at)
    if not hasattr(context, "template_dids"):
        context.template_dids = {}
    context.template_dids[template_name] = template_did


@given("the service provides contract data in the request payload")
def step_given_payload_data(context):
    context.contract_payload_extra = {"source": "bdd"}