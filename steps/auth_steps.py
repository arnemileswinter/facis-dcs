"""Authentication and scenario setup steps for executable BDD scenarios."""

import os

from behave import given

from support.oidc_provider_client import token_for_role
from support.template_utils import template_env_key


def _set_headers_for_role(context, role: str, username_prefix: str = "bdd"):
    client_id = os.getenv("BDD_OIDC_CLIENT_ID", "digital-contracting-service")
    token = token_for_role(role=role, client_id=client_id, username_prefix=username_prefix)
    context.headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }


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