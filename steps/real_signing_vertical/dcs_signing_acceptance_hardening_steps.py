"""BDD steps for the ADR-20 signing-acceptance-hardening negative scenarios
(features/22_real_signing_vertical/signing_acceptance_hardening.feature):
cert-subject to PID name mismatch, invalid JAdES, revoked PID, a level below
the contract's required one, and a replayed callback. The happy path (nonce
binding, byte pinning) is exercised by oid4vp_document_retrieval.feature and
the "correct request nonce"/"incorrect request nonce" scenarios in
real_signing_vertical.feature; this pack covers what those don't.
"""

from __future__ import annotations

import base64
import os

import requests as _requests
from behave import given, then, when

from steps.support.api_client import signature_request_leaf_url, signature_request_publish_url, post_json
from steps.support.services.auth_service import AuthService
from steps.support.services.contract_service import ContractService
from steps.template_management.contract_state_machine_steps import _advance_to_approved

from steps.real_signing_vertical.dcs_real_signing_vertical_steps import (
    CEREMONY_AUD,
    _build_pid_presentation,
    _complete_ceremony_via_presentation,
    _run_full_ceremony,
)


def _publish(context, name):
    ceremony_id = context.ceremony_ids[name]
    signer_h = AuthService.get_headers_for_roles(["Contract Signer"])
    resp = post_json(
        context, signature_request_publish_url(context, ceremony_id),
        {"ceremony_id": ceremony_id, "credential_type": "AES"}, headers=signer_h,
    )
    assert resp.status_code == 200, f"publish failed for ceremony {ceremony_id!r}: {resp.status_code} {resp.text}"
    context.published_ceremony = resp.json()
    return ceremony_id


@given('contract "{name}" is APPROVED and has completed a signing ceremony requiring "{level}" for signatory "{signatory_name}"')
def step_given_approved_ceremony_requiring_level(context, name, level, signatory_name):
    party_did = ContractService._local_peer_did(context)
    ContractService._create_contract_in_draft_with_signature_field(
        context, name, party_did, required_credential_type=level,
    )
    _advance_to_approved(context, name)
    _run_full_ceremony(context, name, field_name=party_did, signatory_name=signatory_name)


@when('the signer publishes the OID4VP signing request for contract "{name}"')
def step_when_publish_for_hardening(context, name):
    _publish(context, name)


@when('the wallet signs contract "{name}" as "{signatory}" with a certificate naming "{given_name}" "{family_name}"')
def step_when_wallet_signs_with_mismatched_cert(context, name, signatory, given_name, family_name):
    """Sign via document retrieval with a certificate that deliberately does
    NOT match the ceremony's verified PID (ADR-20 item 4) — a fresh `user`
    label so its cached key/cert never collides with the matching identity's."""
    AuthService._ensure_dcs_wallet_importable()
    from dcs_wallet.oid4vp_signing import sign_via_document_retrieval  # noqa: PLC0415

    ceremony_id = context.ceremony_ids[name]
    request_uri = signature_request_leaf_url(context, ceremony_id, "object")
    dss_url = os.getenv("BDD_DSS_URL", "http://localhost:18099")
    try:
        context.wallet_callback_response = sign_via_document_retrieval(
            request_uri=request_uri, user=signatory, dss_url=dss_url,
            keys_dir=AuthService.resolve_wallet_keys_dir(),
            given_name=given_name, family_name=family_name,
        )
        context.requests_response = None
    except Exception as exc:  # the wallet CLI raises SystemExit-shaped errors on a non-200 callback
        context.wallet_callback_response = None
        context.signing_error = str(exc)


@when('the wallet signs contract "{name}" as "{signatory}" with an ordinary AES signature')
def step_when_wallet_signs_ordinary_aes(context, name, signatory):
    """A correctly cert↔PID-matched, cryptographically valid AES signature —
    the dev CA is not on any qualified trusted list, so this can never
    satisfy a QES-required field (ADR-20 item 5)."""
    AuthService._ensure_dcs_wallet_importable()
    from dcs_wallet.oid4vp_signing import sign_via_document_retrieval  # noqa: PLC0415

    ceremony_id = context.ceremony_ids[name]
    request_uri = signature_request_leaf_url(context, ceremony_id, "object")
    dss_url = os.getenv("BDD_DSS_URL", "http://localhost:18099")
    try:
        context.wallet_callback_response = sign_via_document_retrieval(
            request_uri=request_uri, user=signatory, dss_url=dss_url,
            keys_dir=AuthService.resolve_wallet_keys_dir(),
        )
        context.requests_response = None
    except Exception as exc:
        context.wallet_callback_response = None
        context.requests_response = None
        context.signing_error = str(exc)


@then('the ceremony callback for contract "{name}" rejects the signature')
def step_then_callback_rejects(context, name):
    resp = getattr(context, "requests_response", None)
    if resp is not None:
        assert resp.status_code >= 400, f"expected rejection, got {resp.status_code}: {resp.text}"
        return
    err = getattr(context, "signing_error", None)
    callback = getattr(context, "wallet_callback_response", None)
    assert err or (callback is not None and callback.get("status") != "SIGNED"), (
        f"expected the ceremony callback to reject the signature, got callback={callback!r} err={err!r}"
    )


@when('a raw invalid JAdES is submitted for the signed document on contract "{name}"')
def step_when_submit_invalid_jades(context, name):
    """Sign the PDF properly (a valid PAdES), then post it back with a
    garbage signatureObject[0] — the JAdES must be rejected on its own merits,
    independent of the PAdES being genuinely valid (ADR-20 item 6)."""
    AuthService._ensure_dcs_wallet_importable()
    from dcs_wallet.remote_signer import sign_pdf  # noqa: PLC0415

    ceremony_id = context.ceremony_ids[name]
    document_uri = signature_request_leaf_url(context, ceremony_id, "document")
    to_be_signed = _requests.get(document_uri, timeout=context.http_timeout_seconds).content
    signatory = context.pid_presentations[name]["given_name"]
    signed_pdf = sign_pdf(to_be_signed, user=signatory, dss_url=os.getenv("BDD_DSS_URL", "http://localhost:18099"), keys_dir=AuthService.resolve_wallet_keys_dir())

    callback_uri = signature_request_leaf_url(context, ceremony_id, "callback")
    context.requests_response = _requests.post(
        callback_uri,
        data={
            "documentWithSignature[0]": base64.b64encode(signed_pdf).decode(),
            "signatureObject[0]": "not-a-valid-jades-at-all",
        },
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        timeout=context.http_timeout_seconds,
    )


@when('the wallet signs contract "{name}" as "{signatory}" and the same callback is replayed')
def step_when_sign_and_replay(context, name, signatory):
    AuthService._ensure_dcs_wallet_importable()
    from dcs_wallet.oid4vp_signing import sign_via_document_retrieval  # noqa: PLC0415

    ceremony_id = context.ceremony_ids[name]
    request_uri = signature_request_leaf_url(context, ceremony_id, "object")
    dss_url = os.getenv("BDD_DSS_URL", "http://localhost:18099")
    first = sign_via_document_retrieval(
        request_uri=request_uri, user=signatory, dss_url=dss_url,
        keys_dir=AuthService.resolve_wallet_keys_dir(),
    )
    assert first.get("status") == "SIGNED", f"first ceremony completion did not reach SIGNED: {first}"
    context.wallet_callback_response = first

    # Replay: fetch the (still-served, unexpired) document and resubmit a
    # freshly-signed copy to the SAME callback — the ceremony is already
    # consumed, so this must be rejected regardless of the signature's own
    # validity (ADR-20 item 3 atomic consumption).
    document_uri = signature_request_leaf_url(context, ceremony_id, "document")
    to_be_signed = _requests.get(document_uri, timeout=context.http_timeout_seconds).content
    from dcs_wallet.remote_signer import sign_pdf  # noqa: PLC0415

    replay_pdf = sign_pdf(to_be_signed, user=signatory, dss_url=dss_url, keys_dir=AuthService.resolve_wallet_keys_dir())
    callback_uri = signature_request_leaf_url(context, ceremony_id, "callback")
    context.requests_response = _requests.post(
        callback_uri,
        data={"documentWithSignature[0]": base64.b64encode(replay_pdf).decode()},
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        timeout=context.http_timeout_seconds,
    )


@when('the PID for contract "{name}"\'s ceremony is revoked before it is presented')
def step_when_pid_revoked_before_presentation(context, name):
    """Build a PID presentation, revoke ITS status-list index, then present it
    — the presentation is otherwise perfectly valid, isolating the status
    check (ADR-20/SM-18: VerifyPID's status check runs unconditionally now
    that EUDIPLO, which omitted status, is removed)."""
    AuthService._ensure_dcs_wallet_importable()
    from dcs_wallet.credential import decode_jwt_payload  # noqa: PLC0415
    from dcs_wallet.sdjwt import split_sd_jwt  # noqa: PLC0415
    from dcs_wallet.status_list import credential_status_from_claims, revoke_status_index  # noqa: PLC0415

    ceremony_id = context.ceremony_ids[name]
    nonce = context.pid_presentations[name]["nonce"]
    given_name = context.pid_presentations[name]["given_name"]
    family_name = context.pid_presentations[name]["family_name"]

    presentation, issuer_jwt, _disclosures, subject_did = _build_pid_presentation(
        given_name=given_name, family_name=family_name, aud=CEREMONY_AUD, nonce=nonce,
    )
    claims = decode_jwt_payload(split_sd_jwt(presentation)[0])
    idx_uri = credential_status_from_claims(claims)
    assert idx_uri, f"self-issued PID carries no status claim to revoke: {claims}"
    idx, uri = idx_uri
    revoke_status_index(idx, service_base=uri.split("/v1/")[0], tenant=uri.split("/tenants/")[1].split("/")[0])

    field_name = ContractService._local_peer_did(context)
    context.requests_response = _complete_ceremony_via_presentation(
        context, ceremony_id, presentation, subject_did, given_name, family_name,
        poa_organization=field_name, nonce=nonce,
    )


@then('the ceremony presentation for contract "{name}" is rejected')
def step_then_presentation_rejected(context, name):
    resp = context.requests_response
    assert resp.status_code >= 400, f"expected the presentation to be rejected, got {resp.status_code}: {resp.text}"
