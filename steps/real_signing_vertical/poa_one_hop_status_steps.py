"""Acceptance steps for the one-hop PoA and status-list baseline."""

from __future__ import annotations

import base64
import concurrent.futures
import json
import os
import subprocess
import time
from datetime import datetime, timedelta, timezone
from urllib.parse import urlsplit

import requests
from behave import then, when
from cryptography import x509
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import NameOID

from steps.real_signing_vertical.dcs_real_signing_vertical_steps import (
    POA_QUERY_ID,
    _build_poa_presentation,
    _complete_ceremony_via_presentation,
    ceremony_aud,
)
from steps.support.api_client import (
    contract_terminate_url,
    contract_peer_provenance_url,
    pac_audit_url,
    post_json,
    signature_request_leaf_url,
)
from steps.support.services.auth_service import AuthService
from steps.support.services.contract_service import ContractService
from steps.support.services.pdf_service import PDFService


def _b64u(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode()


def _decode_jwt_part(token: str, part: int) -> dict:
    raw = token.split(".")[part]
    return json.loads(base64.urlsafe_b64decode(raw + "=" * (-len(raw) % 4)))


def _sign_poa(
    context,
    *,
    organization: str,
    nonce: str,
    status_claim,
    issuer_private=None,
    issuer_did="did:web:dev.example:issuer:poa",
    issuer_cert_der=None,
) -> str:
    AuthService._ensure_dcs_wallet_importable()
    from dcs_wallet.issuer import sign_credential_sd_jwt, sign_credential_sd_jwt_x5c, sign_key_binding_jwt
    from dcs_wallet.keys import cnf_jwk, did_jwk_from_public_jwk, public_jwk
    from dcs_wallet.sdjwt import join_sd_jwt, split_sd_jwt

    keys = AuthService.load_wallet_keys()
    issuer_private = issuer_private or keys.issuer_private
    holder_public = public_jwk(keys.wallet_private)
    subject_did = did_jwk_from_public_jwk(holder_public)
    claims = {
        "iss": issuer_did,
        "sub": subject_did,
        "vct": "urn:dcs:poa:v1",
        "iat": int(time.time()) - 60,
        "exp": int(time.time()) + 3600,
        "cnf": {"jwk": cnf_jwk(holder_public)},
    }
    if isinstance(status_claim, dict) and "__credentialStatus" in status_claim:
        claims["credentialStatus"] = status_claim["__credentialStatus"]
    elif status_claim is not None:
        claims["status"] = status_claim
    kwargs = {
        "visible_claims": claims,
        "selective_claims": {"organization": organization, "roles": ["Contract Signer"]},
        "issuer_private": issuer_private,
    }
    issued = (
        sign_credential_sd_jwt_x5c(**kwargs, issuer_cert_der=issuer_cert_der)
        if issuer_cert_der is not None
        else sign_credential_sd_jwt(**kwargs)
    )
    issuer_jwt, disclosures, _ = split_sd_jwt(issued)
    kb = sign_key_binding_jwt(
        issuer_jwt=issuer_jwt,
        disclosures=disclosures,
        wallet_private=keys.wallet_private,
        aud=ceremony_aud(context),
        nonce=nonce,
    )
    return join_sd_jwt(issuer_jwt, disclosures, kb)


def _post_poa(context, presentation: str):
    c = context.poa_ceremony
    return _complete_ceremony_via_presentation(
        context,
        c["id"],
        c["presentation"],
        c["subject_did"],
        "PoA Signatory",
        "BDD-Testperson",
        poa_organization=None,
        nonce=c["nonce"],
    ) if presentation == "" else requests.post(
        signature_request_leaf_url(context, c["id"], "callback"),
        data={
            "state": c["id"],
            "vp_token": json.dumps({
                "eudi_pid_credential": [c["presentation"]],
                POA_QUERY_ID: [presentation],
            }, separators=(",", ":")),
        },
        headers={"Content-Type": "application/x-www-form-urlencoded"},
        timeout=context.http_timeout_seconds,
    )


@then(
    'the accepted PoA evidence for contract "{name}" is holder-bound, active, '
    "organization-matched, and has one issuer leaf directly under the Dev Root"
)
def step_then_accepted_poa_evidence(context, name):
    did, _ = ContractService._contract_data(context, name)
    cur = context.db.cursor()
    cur.execute(
        "SELECT vp_token, signer_did, poa_organization FROM signature_ceremonies "
        "WHERE contract_did = %s ORDER BY created_at DESC LIMIT 1",
        (did,),
    )
    row = cur.fetchone()
    cur.close()
    assert row and row[0], "accepted ceremony did not retain its PoA presentation"
    envelope = json.loads(row[0])
    poa = (envelope.get(POA_QUERY_ID) or [None])[0]
    assert poa, f"accepted ceremony retained no {POA_QUERY_ID} evidence"
    issuer_jwt, *_ = poa.split("~")
    header = _decode_jwt_part(issuer_jwt, 0)
    claims = _decode_jwt_part(issuer_jwt, 1)
    assert claims.get("vct") == "urn:dcs:poa:v1"
    assert claims.get("sub") == row[1], "PoA subject is not the verified ceremony holder"
    assert row[2] == ContractService._local_peer_did(context), "PoA organization is not the signed party"
    status = claims.get("status", {}).get("status_list", {})
    assert status.get("uri") and status.get("idx") is not None, "PoA has no live status reference"
    from dcs_wallet.status_list import bit_is_revoked, encoded_list_from_payload, fetch_status_list_payload
    payload = fetch_status_list_payload(status["uri"])
    assert not bit_is_revoked(encoded_list_from_payload(payload), int(status["idx"])), "accepted PoA is revoked"
    chain = header.get("x5c")
    assert isinstance(chain, list) and len(chain) == 2, (
        "accepted PoA does not carry issuer-leaf + Dev-Root x5c evidence"
    )
    leaf = x509.load_der_x509_certificate(base64.b64decode(chain[0]))
    root = x509.load_der_x509_certificate(base64.b64decode(chain[1]))
    assert leaf.issuer == root.subject and root.issuer == root.subject


@when("the ceremony presentation is completed with a holder-bound PoA from an untrusted root")
def step_when_untrusted_root_poa(context):
    root_key = ec.generate_private_key(ec.SECP256R1())
    leaf_key = ec.generate_private_key(ec.SECP256R1())
    now = datetime.now(timezone.utc)
    root_subject = x509.Name(
        [x509.NameAttribute(NameOID.COMMON_NAME, "BDD Untrusted PoA Root")]
    )
    leaf_subject = x509.Name(
        [x509.NameAttribute(NameOID.COMMON_NAME, "BDD Untrusted PoA Issuer Leaf")]
    )
    root_cert = (
        x509.CertificateBuilder()
        .subject_name(root_subject)
        .issuer_name(root_subject)
        .public_key(root_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=1))
        .not_valid_after(now + timedelta(hours=1))
        .add_extension(x509.BasicConstraints(ca=True, path_length=0), critical=True)
        .sign(root_key, hashes.SHA256())
    )
    leaf_cert = (
        x509.CertificateBuilder()
        .subject_name(leaf_subject)
        .issuer_name(root_cert.subject)
        .public_key(leaf_key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(now - timedelta(minutes=1))
        .not_valid_after(now + timedelta(hours=1))
        .add_extension(x509.BasicConstraints(ca=False, path_length=None), critical=True)
        .sign(root_key, hashes.SHA256())
    )
    numbers = leaf_key.private_numbers()
    pub = numbers.public_numbers
    jwk = {
        "kty": "EC", "crv": "P-256",
        "x": _b64u(pub.x.to_bytes(32, "big")),
        "y": _b64u(pub.y.to_bytes(32, "big")),
        "d": _b64u(numbers.private_value.to_bytes(32, "big")),
    }
    from dcs_wallet.status_list import BDD_CREDENTIAL_TENANT, build_credential_status
    c = context.poa_ceremony
    status = build_credential_status(
        sub=c["subject_did"], organization=c["party_did"], roles=["Contract Signer"],
        service_base=os.environ["STATUSLIST_SERVICE_URL"], tenant=BDD_CREDENTIAL_TENANT,
    )
    poa = _sign_poa(
        context, organization=c["party_did"], nonce=c["nonce"], status_claim=status,
        issuer_private=jwk, issuer_did="did:web:untrusted.example:issuer:poa",
        issuer_cert_der=[
            leaf_cert.public_bytes(serialization.Encoding.DER),
            root_cert.public_bytes(serialization.Encoding.DER),
        ],
    )
    issuer_jwt = poa.split("~", 1)[0]
    chain = _decode_jwt_part(issuer_jwt, 0).get("x5c")
    assert isinstance(chain, list) and len(chain) == 2, chain
    parsed_leaf = x509.load_der_x509_certificate(base64.b64decode(chain[0]))
    parsed_root = x509.load_der_x509_certificate(base64.b64decode(chain[1]))
    assert parsed_leaf.issuer == parsed_root.subject
    assert not parsed_leaf.extensions.get_extension_for_class(
        x509.BasicConstraints
    ).value.ca
    assert parsed_root.extensions.get_extension_for_class(
        x509.BasicConstraints
    ).value.ca
    parsed_leaf.verify_directly_issued_by(parsed_root)
    parsed_root.verify_directly_issued_by(parsed_root)
    context.untrusted_poa_chain = (parsed_leaf, parsed_root)
    context.status_probe_started = time.monotonic()
    context.requests_response = _post_poa(context, poa)


@then("the signing request is rejected because the Power of Attorney trust chain is untrusted")
def step_then_untrusted_chain_rejected(context):
    resp = context.requests_response
    assert resp.status_code >= 400, f"untrusted PoA root was accepted: {resp.status_code} {resp.text}"
    assert any(word in resp.text.lower() for word in ("trust", "issuer", "x5c", "certificate"))


@then("the rejected PoA carried a formally valid leaf-to-self-signed-root x5c chain")
def step_then_untrusted_chain_was_formally_valid(context):
    leaf, root = context.untrusted_poa_chain
    assert leaf.issuer == root.subject and root.issuer == root.subject
    assert not leaf.extensions.get_extension_for_class(x509.BasicConstraints).value.ca
    assert root.extensions.get_extension_for_class(x509.BasicConstraints).value.ca
    leaf.verify_directly_issued_by(root)
    root.verify_directly_issued_by(root)


@when('the ceremony presentation uses a PoA whose XFSC status is changed to "{state}"')
def step_when_poa_status_changed(context, state):
    from dcs_wallet.credential import decode_jwt_payload
    from dcs_wallet.sdjwt import split_sd_jwt
    from dcs_wallet.status_list import credential_status_from_claims, revoke_status_index
    c = context.poa_ceremony
    poa = _build_poa_presentation(
        organization=c["party_did"],
        roles=["Contract Signer"],
        aud=ceremony_aud(context),
        nonce=c["nonce"],
    )
    claims = decode_jwt_payload(split_sd_jwt(poa)[0])
    idx, uri = credential_status_from_claims(claims)
    revoke_status_index(
        idx, service_base=uri.split("/v1/")[0],
        tenant=uri.split("/tenants/")[1].split("/")[0],
    )
    context.status_probe_started = time.monotonic()
    context.status_probe_state = state
    context.requests_response = _post_poa(context, poa)


@then("the signing request is rejected for live credential status within 5 minutes")
def step_then_status_rejected_in_time(context):
    resp = context.requests_response
    assert resp.status_code >= 400, f"status-invalid PoA was accepted: {resp.status_code} {resp.text}"
    assert time.monotonic() - context.status_probe_started <= 300
    assert any(word in resp.text.lower() for word in ("status", "revoked", "suspended"))


@then("the XFSC binary status is normalized as revoked rather than active")
def step_then_xfsc_binary_revoked(context):
    assert "revoked" in context.requests_response.text.lower()
    assert "active" not in context.requests_response.text.lower()


def _signed_w3c_status_uri(context, state: str, signature_defect: str = "") -> tuple[str, str]:
    """Publish a genuinely signed W3C status-list VC to the existing shared
    IPFS test seam. The DCS fetches it through the in-cluster IPFS gateway;
    service naming derives from the harness' HELM_RELEASE, not a hard-coded
    external base URL."""
    import gzip
    import jwt
    from steps.support.tamper_seam import ipfs_add_bytes
    from dcs_wallet.issuer import _jwt_private_key

    purpose = "suspension" if state == "suspended" else "revocation"
    bitstring = bytearray(16384)
    if state != "active":
        # W3C Bitstring Status List uses MSB-first packing.
        bitstring[0] = 0x80
    encoded = "u" + _b64u(gzip.compress(bytes(bitstring)))
    issuer = (
        "did:web:untrusted.example:status"
        if signature_defect == "untrusted signature"
        else "did:web:dev.example:issuer:poa"
    )
    claims = {
        "iss": issuer,
        "iat": int(time.time()) - 60,
        "exp": int(time.time()) + 3600,
        "type": ["VerifiableCredential", "BitstringStatusListCredential"],
        "credentialSubject": {
            "id": "urn:bdd:w3c-status-list#list",
            "type": "BitstringStatusList",
            "statusPurpose": purpose,
            "encodedList": encoded,
        },
    }
    key = _jwt_private_key(AuthService.load_wallet_keys().issuer_private)
    if signature_defect == "bad signature":
        numbers = ec.generate_private_key(ec.SECP256R1()).private_numbers()
        pub = numbers.public_numbers
        key = _jwt_private_key({
            "kty": "EC", "crv": "P-256",
            "x": _b64u(pub.x.to_bytes(32, "big")),
            "y": _b64u(pub.y.to_bytes(32, "big")),
            "d": _b64u(numbers.private_value.to_bytes(32, "big")),
        })
    token = jwt.encode(
        claims,
        key,
        algorithm="ES256",
        headers={"typ": "vc+jwt", "alg": "ES256"},
    )
    cid = ipfs_add_bytes(token.encode())
    release = os.getenv("HELM_RELEASE", "dcs").strip()
    port = os.getenv("BDD_IPFS_GATEWAY_PORT", "8080").strip()
    return f"http://{release}-ipfs:{port}/ipfs/{cid}", purpose


@when('the ceremony presentation uses a PoA with signed W3C status "{state}"')
def step_when_signed_w3c_status(context, state):
    c = context.poa_ceremony
    uri, purpose = _signed_w3c_status_uri(context, state)
    status = {
        "__credentialStatus": {
            "id": f"{uri}#0",
            "type": "BitstringStatusListEntry",
            "statusPurpose": purpose,
            "statusListIndex": "0",
            "statusListCredential": uri,
        }
    }
    poa = _sign_poa(
        context,
        organization=c["party_did"],
        nonce=c["nonce"],
        status_claim=status,
    )
    context.w3c_expected_state = state
    context.requests_response = _post_poa(context, poa)


@then('the W3C PoA status is normalized to decision "{decision}"')
def step_then_w3c_status_decision(context, decision):
    resp = context.requests_response
    if decision == "accepted":
        assert resp.status_code == 200, f"active signed W3C status was rejected: {resp.status_code} {resp.text}"
        return
    assert resp.status_code >= 400, (
        f"{context.w3c_expected_state} signed W3C status was accepted: {resp.status_code} {resp.text}"
    )
    expected = "suspended" if context.w3c_expected_state == "suspended" else "revoked"
    assert expected in resp.text.lower(), (
        f"W3C {context.w3c_expected_state} was not normalized traceably: {resp.text}"
    )


@then("the ceremony remains unverified and no PoA authority is persisted")
def step_then_ceremony_unverified(context):
    cur = context.db.cursor()
    cur.execute(
        "SELECT status, poa_organization FROM signature_ceremonies WHERE id = %s",
        (context.poa_ceremony["id"],),
    )
    row = cur.fetchone()
    cur.close()
    assert row and str(row[0]).upper() == "PENDING" and not row[1], row


@when('the ceremony presentation uses otherwise valid PoA evidence with status defect "{defect}"')
def step_when_status_defect(context, defect):
    c = context.poa_ceremony
    if defect == "missing reference":
        status = None
    elif defect == "bad reference":
        status = {
            "__credentialStatus": {
                "type": "BitstringStatusListEntry",
                "statusPurpose": "revocation",
                "statusListIndex": "not-an-index",
                "statusListCredential": "urn:bdd:bad-reference",
            }
        }
    elif defect in ("bad signature", "untrusted signature"):
        uri, purpose = _signed_w3c_status_uri(context, "active", signature_defect=defect)
        status = {
            "__credentialStatus": {
                "type": "BitstringStatusListEntry",
                "statusPurpose": purpose,
                "statusListIndex": "0",
                "statusListCredential": uri,
            }
        }
    elif defect == "unknown mechanism":
        status = {"not_a_status_list_reference": {"idx": 0, "uri": "urn:unsupported:bdd"}}
    elif defect == "bad encoding":
        status = {
            "status_list": {
                "idx": 0,
                "uri": f"{context.base_url}/did.json",
            }
        }
    elif defect == "unavailable":
        status = {"status_list": {"idx": 0, "uri": "http://127.0.0.1:1/bdd-unavailable-status"}}
    else:
        raise AssertionError(f"unsupported status defect: {defect}")
    poa = _sign_poa(
        context,
        organization=c["party_did"],
        nonce=c["nonce"],
        status_claim=status,
    )
    context.requests_response = _post_poa(context, poa)


@then("the signing request is rejected for a traceable status-evidence reason")
def step_then_status_defect_rejected(context):
    resp = context.requests_response
    assert resp.status_code >= 400, f"unusable status evidence was accepted: {resp.status_code} {resp.text}"
    body = resp.text.lower()
    assert any(marker in body for marker in ("status", "reference", "mechanism", "retriev", "decod")), (
        f"rejection does not identify the status-evidence gate: {resp.status_code} {resp.text}"
    )


def _provenance_on_b(context):
    headers = AuthService.get_headers_for_roles(["Contract Manager"], api_base=context.base_url_b)
    original = context.base_url
    context.base_url = context.base_url_b
    try:
        url = contract_peer_provenance_url(context)
    finally:
        context.base_url = original
    return requests.get(url, params={"did": context.cross_instance_contract_did},
                        headers=headers, timeout=context.http_timeout_seconds)


def _tamper_compact_signature(compact: str) -> str:
    parts = compact.split(".")
    assert len(parts) == 3 and parts[2], f"not a signed compact JWT: {compact[:80]}"
    parts[2] = ("A" if parts[2][0] != "A" else "B") + parts[2][1:]
    return ".".join(parts)


def _peer_poa_evidence(context, did: str, defect: str) -> dict | None:
    if defect == "missing":
        return None

    # This negative-path packet is built without starting a ceremony. Derive
    # the audience from instance A's configured public host, using the same
    # x509_san_dns client-id scheme as the real request object.
    instance_a_host = str(urlsplit(context.base_url_a).hostname or "").strip()
    assert instance_a_host, f"instance A base URL has no host: {context.base_url_a!r}"
    audience = f"x509_san_dns:{instance_a_host}"
    nonce = f"bdd-peer-poa-{time.time_ns()}"
    presentation = _build_poa_presentation(
        organization=context.peer_did_a,
        roles=["Contract Signer"],
        aud=audience,
        nonce=nonce,
    )
    pieces = [piece for piece in presentation.split("~") if piece]
    assert len(pieces) >= 2, "PoA presentation lacks issuer and key-binding JWTs"
    issuer_claims = _decode_jwt_part(pieces[0], 1)
    kb_claims = _decode_jwt_part(pieces[-1], 1)
    holder_did = issuer_claims.get("sub")
    assert holder_did, f"PoA presentation has no holder subject: {issuer_claims}"
    assert kb_claims.get("nonce") == nonce and kb_claims.get("aud") == audience

    evidence = {
        "presentation": presentation,
        "nonce": nonce,
        "aud": audience,
        "vct": "urn:dcs:poa:v1",
        "contract_id": did,
        "field_name": context.peer_did_a,
        "holder_did": holder_did,
        "organization": context.peer_did_a,
    }
    if defect == "invalid signature":
        tampered = list(pieces)
        tampered[0] = _tamper_compact_signature(tampered[0])
        evidence["presentation"] = "~".join(tampered)
    elif defect == "wrong nonce":
        evidence["nonce"] = nonce + "-transport-tampered"
    elif defect == "wrong audience":
        evidence["aud"] = audience + "-transport-tampered"
    elif defect == "wrong organization":
        evidence["organization"] = context.peer_did_b
    elif defect != "valid":
        raise AssertionError(f"unsupported peer PoA evidence defect: {defect}")
    return evidence


def _send_peer_acceptance_with_poa_defect(context, defect: str):
    """Build the packet independently of the normal A signing ship.

    The offer has already established the contract and CEK on B. Every
    variant sends a genuine PAdES PDF, A's genuine instance-key JAdES over
    the exact contract document, and A's genuine did:web challenge-response
    signature to B's real PostPdf endpoint. Except for ``missing``, it also
    sends a valid, active, holder-bound PoA and changes exactly the named
    receiver-side evidence gate.
    """
    import uuid

    from dcs_wallet.remote_signer import sign_pdf
    from steps.peer_trust.dcs_peer_trust_steps import (
        _as_instance,
        _canonical_jades_payload,
        _jades_sign_as_own_instance,
        _own_identity,
    )
    from steps.template_management.contract_state_machine_steps import (
        _sign_secret_value_with_dev_key,
    )
    from steps.support.api_client import (
        contract_peer_pdf_url,
        contract_retrieve_by_id_url,
        get_with_headers,
    )
    from steps.support.services.pdf_service import PDFService

    did = context.cross_instance_contract_did
    with _as_instance(context, context.base_url_a):
        manager_h = AuthService.get_headers_for_roles(
            ["Contract Manager"], api_base=context.base_url_a
        )
        retrieve = get_with_headers(
            context, contract_retrieve_by_id_url(context, did), headers=manager_h
        )
        assert retrieve.status_code == 200, (
            f"could not retrieve approved contract on A: "
            f"{retrieve.status_code} {retrieve.text}"
        )
        contract = retrieve.json()
        contract_document = contract.get("contract_data")
        contract_version = int(contract.get("contract_version") or 0)
        assert isinstance(contract_document, dict) and contract_version > 0, (
            f"A's contract response lacks document/version: {contract}"
        )

        exported = PDFService.export_contract_pdf(context, did, headers=manager_h)
        assert exported.status_code == 200 and exported.content.startswith(b"%PDF-"), (
            f"could not export a genuine PDF on A: "
            f"{exported.status_code} {exported.text}"
        )
        signed_pdf = sign_pdf(
            exported.content,
            user="MissingPoaPeerAcceptance",
            dss_url=os.getenv("BDD_DSS_URL", "http://localhost:18099"),
            field=context.peer_did_a,
            keys_dir=AuthService.resolve_wallet_keys_dir(),
        )

        jades_payload = _canonical_jades_payload(
            did, contract_version, contract_document
        )
        jades_signature = _jades_sign_as_own_instance(context, jades_payload)
        peer_did, token_dir = _own_identity(context)
        assert peer_did == context.peer_did_a, (
            f"packet signer {peer_did} is not instance A {context.peer_did_a}"
        )
        secret_value = str(uuid.uuid4())
        secret_hash = base64.b64encode(
            _sign_secret_value_with_dev_key(token_dir, secret_value)
        ).decode()
        poa_evidence = _peer_poa_evidence(context, did, defect)

    with _as_instance(context, context.base_url_b):
        payload = {
            "from_peer_did": context.peer_did_a,
            "contract_iri": did,
            "pdf": base64.b64encode(signed_pdf).decode(),
            "jades_signature": jades_signature,
            "contract_state": "SIGNED",
            "secret_value": secret_value,
            "secret_hash": secret_hash,
        }
        if poa_evidence is not None:
            payload["poa_evidence"] = poa_evidence
        response = post_json(
            context, contract_peer_pdf_url(context), payload, headers={}
        )
    context.peer_poa_negative_defect = defect
    context.peer_poa_negative_response = response
    context.missing_poa_peer_response = response
    context.requests_response = response


@when("instance A sends an otherwise valid peer-signed acceptance without transferable PoA evidence")
def step_when_peer_acceptance_omits_poa_evidence(context):
    _send_peer_acceptance_with_poa_defect(context, "missing")


@when(
    'instance A sends an otherwise valid peer-signed acceptance whose transferable '
    'PoA evidence has defect "{defect}"'
)
def step_when_peer_acceptance_has_invalid_poa_evidence(context, defect):
    _send_peer_acceptance_with_poa_defect(context, defect)


@then(
    "instance B stores the revalidated urn:dcs:poa:v1 evidence with exactly "
    "instance A's original ceremony nonce and audience"
)
def step_then_peer_poa_context_persisted(context):
    deadline = time.monotonic() + 45
    response = None
    while time.monotonic() < deadline:
        response = _provenance_on_b(context)
        if response.status_code == 200:
            break
        time.sleep(1)
    assert response is not None and response.status_code == 200, (
        f"instance B did not store signed sync provenance: {response.status_code if response else 'n/a'} "
        f"{response.text if response else ''}"
    )
    body = response.json()
    evidence = body.get("poa_evidence")
    assert isinstance(evidence, dict), f"sync provenance has no revalidated poa_evidence: {body}"
    assert evidence.get("vct") == "urn:dcs:poa:v1", evidence

    cur = context.db.cursor()
    cur.execute(
        "SELECT vp_token FROM signature_ceremonies "
        "WHERE contract_did = %s ORDER BY created_at DESC LIMIT 1",
        (context.cross_instance_contract_did,),
    )
    row = cur.fetchone()
    cur.close()
    assert row and row[0], "instance A retained no original ceremony presentation"
    envelope = json.loads(row[0])
    original_poa = (envelope.get(POA_QUERY_ID) or [None])[0]
    assert original_poa, f"instance A ceremony has no {POA_QUERY_ID} presentation"
    original_parts = [part for part in original_poa.split("~") if part]
    original_kb = _decode_jwt_part(original_parts[-1], 1)
    original_nonce = original_kb.get("nonce")
    original_audience = original_kb.get("aud")
    assert original_nonce and original_audience, original_kb
    assert evidence.get("nonce") == original_nonce, (
        f"B stored nonce {evidence.get('nonce')!r}, not A ceremony nonce "
        f"{original_nonce!r}"
    )
    assert evidence.get("aud") == original_audience, (
        f"B stored audience {evidence.get('aud')!r}, not A ceremony audience "
        f"{original_audience!r}"
    )
    assert evidence.get("revalidated_at"), f"PoA evidence has no receiver-side revalidation timestamp: {evidence}"


@then(
    "instance B rejects the signed acceptance before storing sync provenance "
    "because transferable PoA evidence is missing"
)
def step_then_peer_missing_poa_rejected_before_persistence(context):
    inbound = context.missing_poa_peer_response
    assert inbound.status_code >= 400, (
        "instance B accepted an otherwise valid peer-signed acceptance with no "
        f"transferable urn:dcs:poa:v1 evidence: {inbound.status_code} {inbound.text}"
    )
    body = inbound.text.lower()
    assert "power of attorney" in body or "poa" in body, (
        f"missing-evidence rejection does not name the PoA gate: {inbound.text}"
    )

    provenance = _provenance_on_b(context)
    assert provenance.status_code == 404, (
        "instance B persisted sync provenance before enforcing the transferable "
        f"PoA gate: {provenance.status_code} {provenance.text}"
    )


@then(
    'instance B rejects the peer-signed acceptance before storing sync provenance '
    'at the "{defect}" Power of Attorney gate'
)
def step_then_peer_invalid_poa_rejected_before_persistence(context, defect):
    assert context.peer_poa_negative_defect == defect
    inbound = context.peer_poa_negative_response
    assert inbound.status_code >= 400, (
        f"instance B accepted peer PoA evidence with {defect}: "
        f"{inbound.status_code} {inbound.text}"
    )
    body = inbound.text.lower()
    assert "power of attorney" in body or "poa" in body, (
        f"{defect} rejection does not name the PoA gate: {inbound.text}"
    )
    provenance = _provenance_on_b(context)
    assert provenance.status_code == 404, (
        f"instance B persisted provenance before rejecting {defect}: "
        f"{provenance.status_code} {provenance.text}"
    )


@then("instance B records a traceable Power of Attorney finding for the rejected peer signature")
def step_then_peer_poa_finding(context):
    headers = AuthService.get_headers_for_roles(["Auditor"], api_base=context.base_url_b)
    with_base = context.base_url
    context.base_url = context.base_url_b
    try:
        response = post_json(
            context,
            pac_audit_url(context),
            {
                "scope": "CONTRACT",
                "resource_id": context.cross_instance_contract_did,
                "justification": "BDD rejected peer PoA evidence",
            },
            headers=headers,
        )
    finally:
        context.base_url = with_base
    assert response.status_code == 200, response.text
    assert "power of attorney" in response.text.lower(), (
        f"peer rejection left no traceable Power of Attorney finding: {response.text}"
    )


@then('a durable status-publication queue entry exists for contract "{name}" and lifecycle "{lifecycle}"')
def step_then_status_queue_entry(context, name, lifecycle):
    did, _ = ContractService._contract_data(context, name)
    cur = context.db.cursor()
    cur.execute(
        "SELECT table_name FROM information_schema.tables "
        "WHERE table_schema = 'public' AND table_name IN "
        "('status_publication_queue', 'contract_status_publication_queue')"
    )
    tables = [row[0] for row in cur.fetchall()]
    assert tables, (
        "no persistent status-publication queue exists; a transient lifecycle "
        "handler cannot guarantee retryable <=5 minute XFSC publication"
    )
    table = tables[0]
    cur.execute(
        f"SELECT status, created_at FROM {table} WHERE contract_did = %s "
        "ORDER BY created_at DESC LIMIT 1",
        (did,),
    )
    row = cur.fetchone()
    cur.close()
    assert row and str(row[0]).lower() == lifecycle.lower(), (
        f"no durable {lifecycle} publication intent for {did}: {row}"
    )


@when(
    'two concurrent manager requests enqueue the same "{lifecycle}" status '
    'for contract "{name}"'
)
def step_when_concurrent_same_status(context, lifecycle, name):
    assert lifecycle == "terminated", (
        f"this real lifecycle concurrency path supports terminated, got {lifecycle}"
    )
    did, updated_at = ContractService._contract_data(context, name)
    headers = AuthService.get_headers_for_roles(
        ["Contract Manager"], api_base=context.base_url
    )
    payload = {
        "did": did,
        "reason": "BDD duplicate desired-status delivery",
        "updated_at": updated_at,
    }

    # Hold only the row's UPDATE lock. Both real HTTP transactions can read
    # the same SIGNED state and pass their transition check, then wait at the
    # state update. Releasing the lock lets both reach EnqueueTx with the same
    # (contract_did, status), exercising its ON CONFLICT idempotence instead
    # of merely calling an already-terminal endpoint a second time.
    context.db.commit()
    lock = context.db.cursor()
    lock.execute("SELECT did FROM contracts WHERE did = %s FOR UPDATE", (did,))
    try:
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
            futures = [
                executor.submit(
                    requests.post,
                    contract_terminate_url(context),
                    json=payload,
                    headers=headers,
                    timeout=context.http_timeout_seconds,
                )
                for _ in range(2)
            ]
            time.sleep(1)
            context.db.commit()
            context.concurrent_status_responses = [
                future.result(timeout=context.http_timeout_seconds + 5)
                for future in futures
            ]
    except Exception:
        context.db.rollback()
        raise
    finally:
        lock.close()


@then(
    'both accepted lifecycle requests settle as exactly one logical "{lifecycle}" '
    'publication for contract "{name}"'
)
def step_then_one_logical_status_publication(context, lifecycle, name):
    responses = context.concurrent_status_responses
    assert len(responses) == 2 and all(resp.status_code == 200 for resp in responses), (
        "the duplicate-enqueue boundary was not reached by both lifecycle "
        f"transactions: {[(r.status_code, r.text) for r in responses]}"
    )
    did, _ = ContractService._contract_data(context, name)
    deadline = time.monotonic() + 45
    row = None
    while time.monotonic() < deadline:
        cur = context.db.cursor()
        cur.execute(
            "SELECT COUNT(*), MIN(status), MAX(attempt_count), "
            "COUNT(published_at) FROM status_publication_queue "
            "WHERE contract_did = %s AND LOWER(status) = LOWER(%s)",
            (did, lifecycle),
        )
        row = cur.fetchone()
        cur.close()
        if row and row[0] == 1 and row[3] == 1:
            break
        time.sleep(1)
    assert row and row[0] == 1, (
        f"repeated {lifecycle} enqueue created more than one logical row: {row}"
    )
    assert str(row[1]).lower() == lifecycle and row[2] == 1 and row[3] == 1, (
        f"one desired status caused duplicate or unsettled worker effects: {row}"
    )


@then('the deterministic XFSC status bit for contract "{name}" is revoked within 5 minutes')
def step_then_contract_status_bit_revoked(context, name):
    import hashlib
    import struct
    from dcs_wallet.status_list import bit_is_revoked, encoded_list_from_payload, fetch_status_list_payload, status_list_uri

    did, _ = ContractService._contract_data(context, name)
    index = struct.unpack(">I", hashlib.sha256(did.encode()).digest()[:4])[0] % 131072
    uri = status_list_uri(os.environ["STATUSLIST_SERVICE_URL"], tenant="default")
    deadline = time.monotonic() + 300
    while time.monotonic() < deadline:
        payload = fetch_status_list_payload(uri)
        if bit_is_revoked(encoded_list_from_payload(payload), index):
            return
        time.sleep(2)
    raise AssertionError(f"deterministic XFSC bit {index} for {did} was not revoked within 5 minutes")


@when('the Contract Manager exports and verifies contract "{name}" as PDF')
def step_when_manager_exports_and_verifies(context, name):
    did, _ = ContractService._contract_data(context, name)
    headers = AuthService.get_headers_for_roles(
        ["Contract Manager"], api_base=context.base_url
    )
    exported = PDFService.export_contract_pdf(context, did, headers=headers)
    assert exported.status_code == 200, (
        f"PDF export failed for {name}: {exported.status_code} {exported.text}"
    )
    context.requests_response = PDFService.verify_contract_pdf(
        context, did, headers=headers
    )


@when(
    'the Contract Manager exports contract "{name}" as PDF while XFSC is available'
)
def step_when_manager_exports_before_status_outage(context, name):
    did, _ = ContractService._contract_data(context, name)
    headers = AuthService.get_headers_for_roles(
        ["Contract Manager"], api_base=context.base_url
    )
    exported = PDFService.export_contract_pdf(context, did, headers=headers)
    assert exported.status_code == 200, (
        f"PDF export failed before XFSC outage: {exported.status_code} {exported.text}"
    )
    context.status_outage_verify_did = did
    context.status_outage_verify_headers = headers


def _kubectl_for_statuslist() -> list[str]:
    kubectl = os.getenv("BDD_KUBECTL") or os.getenv("KUBECTL_BIN", "kubectl")
    namespace = os.getenv("K8S_NAMESPACE")
    assert namespace, (
        "K8S_NAMESPACE is required to isolate and restore the genuine XFSC "
        "status-list deployment"
    )
    return [kubectl, "-n", namespace]


def _statuslist_deployment() -> str:
    release = os.getenv("HELM_RELEASE", "dcs").strip()
    proc = subprocess.run(
        _kubectl_for_statuslist()
        + [
            "get",
            "deployment",
            "-l",
            f"app.kubernetes.io/name=statuslist-service,app.kubernetes.io/instance={release}",
            "-o",
            "jsonpath={.items[*].metadata.name}",
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert proc.returncode == 0, proc.stderr
    names = proc.stdout.split()
    assert len(names) == 1, (
        f"expected one genuine XFSC deployment for release {release}, got {names}"
    )
    return names[0]


def _scale_statuslist(deployment: str, replicas: int):
    proc = subprocess.run(
        _kubectl_for_statuslist()
        + ["scale", f"deployment/{deployment}", f"--replicas={replicas}"],
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert proc.returncode == 0, (
        f"could not scale {deployment} to {replicas}: {proc.stderr}"
    )


@when("the genuine XFSC status service becomes unavailable only for PDF verification")
def step_when_status_service_unavailable_for_verify(context):
    deployment = _statuslist_deployment()
    proc = subprocess.run(
        _kubectl_for_statuslist()
        + [
            "get",
            f"deployment/{deployment}",
            "-o",
            "jsonpath={.spec.replicas}",
        ],
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert proc.returncode == 0, proc.stderr
    replicas = int(proc.stdout.strip() or "1")
    assert replicas > 0, f"XFSC deployment {deployment} was already unavailable"

    restored = False

    def restore():
        nonlocal restored
        if restored:
            return
        _scale_statuslist(deployment, replicas)
        rollout = subprocess.run(
            _kubectl_for_statuslist()
            + [
                "rollout",
                "status",
                f"deployment/{deployment}",
                "--timeout=180s",
            ],
            capture_output=True,
            text=True,
            timeout=210,
        )
        assert rollout.returncode == 0, (
            f"XFSC deployment did not recover: {rollout.stdout} {rollout.stderr}"
        )
        restored = True

    context.add_cleanup(restore)
    _scale_statuslist(deployment, 0)
    deadline = time.monotonic() + 60
    last_deployment_state = ""
    last_service_result = ""
    while time.monotonic() < deadline:
        state = subprocess.run(
            _kubectl_for_statuslist()
            + [
                "get",
                f"deployment/{deployment}",
                "-o",
                "jsonpath={.spec.replicas} {.status.readyReplicas} {.status.availableReplicas}",
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )
        assert state.returncode == 0, state.stderr
        last_deployment_state = state.stdout.strip()
        values = last_deployment_state.split()
        desired = int(values[0]) if values else -1
        ready = int(values[1]) if len(values) > 1 else 0
        available = int(values[2]) if len(values) > 2 else 0

        from dcs_wallet.status_list import (
            BDD_CREDENTIAL_TENANT,
            status_list_uri,
        )

        service_base = os.environ["STATUSLIST_SERVICE_URL"].rstrip("/")
        probe_uri = status_list_uri(
            service_base, tenant=BDD_CREDENTIAL_TENANT
        )
        service_unavailable = False
        try:
            probe = requests.get(probe_uri, timeout=2)
            last_service_result = f"HTTP {probe.status_code}"
            service_unavailable = not 200 <= probe.status_code < 300
        except requests.RequestException as exc:
            last_service_result = type(exc).__name__
            service_unavailable = True

        if desired == 0 and ready == 0 and available == 0 and service_unavailable:
            break
        time.sleep(1)
    else:
        raise AssertionError(
            f"XFSC deployment {deployment} did not become unavailable: "
            f"deployment state={last_deployment_state!r}, "
            f"service probe={last_service_result}"
        )

    context.requests_response = PDFService.verify_contract_pdf(
        context,
        context.status_outage_verify_did,
        headers=context.status_outage_verify_headers,
    )


@then(
    "the verifier never reports the contract active and fails closed on the "
    "unavailable live status"
)
def step_then_verifier_fails_closed_on_status_outage(context):
    response = context.requests_response
    if response.status_code >= 400:
        return
    assert response.status_code == 200, response.text
    body = response.json()
    live = str(body.get("status_list_status") or "").lower()
    assert live != "active", (
        f"verifier fabricated active while XFSC was unavailable: {body}"
    )
    explicit_failure = live in {"unavailable", "unknown", "error"} or body.get("match") is False
    assert explicit_failure, (
        "verifier returned a successful match without an explicit failed live-"
        f"status decision while XFSC was unavailable: {body}"
    )


@then('the verifier reports live status "{live}" separately from lifecycle banner "{banner}"')
def step_then_verifier_separates_status(context, live, banner):
    assert context.requests_response.status_code == 200, context.requests_response.text
    body = context.requests_response.json()
    assert str(body.get("status_list_status", "")).lower() == live.lower(), body
    assert str(body.get("lifecycle_status", "")).lower() == banner.lower(), body
    assert "status_list_status" in body and "lifecycle_status" in body
