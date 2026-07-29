# ADR-31: Issuer trust is scoped by purpose, bound to an organization, and resolved by a configurable mechanism

Status: accepted
Date: 2026-07-29

## Context

Trust configuration answered one question — "is this issuer's signature
acceptable?" — and every caller reused that single answer. `trust.json` listed
issuers with their JWKS, `IssuerTrusted(iss)` returned a boolean, and login,
counterparty PoA verification and PID verification all consulted it.

That conflates authenticity with authorization, and on a two-instance
federation it fails concretely: a PoA minted by instance A's issuer produced a
valid session on instance B. Worse, the
credential's `organization` claim becomes the session's participant identity
and the attribution recorded in the audit trail — so the holder acted **as
A's party inside B**, and the two parties the federation exists to distinguish
collapsed into one.

ADR-29 made each demo issuer stamp its own host as the organization, and a
first attempt at a fix required a login credential's organization to equal the
instance's own participant DID. That was the wrong invariant: it encodes "one
instance, one organization", which the BDD suite disproves immediately — its
identities belong to Acme Corp, TechVendor Inc and a per-scenario organization,
all on the same deployment. It also left the underlying weakness untouched:
nothing bound an *issuer* to the organizations it may speak for, so any trusted
issuer could assert any party and the check depended on every issuer being
well-behaved.

Two further requirements land on the same structure:

- **QES needs a PID issuer**, and a PID is meaningfully a *third party's*
  attestation of a natural person — not something the relying party issues to
  itself. There is no PID issuer today.
- **The production issuance and trust model is not yet known.** Deployments may
  resolve issuer keys through x5c chains, bare JWKS, `did:jwk`, `did:web`,
  `did:ebsi`, or something not yet on the list. Hard-coding today's two
  mechanisms would force a code change for each new one.

## Decision

**Trust is declared per purpose, per issuer, and bound to organizations.**

```json
{
  "vcts": ["urn:dcs:poa:v1", "urn:eudi:pid:de:1"],
  "issuers": {
    "https://dcs-ionos.facis.cloud/issuer": {
      "purposes": ["login", "peer"],
      "organizations": ["did:web:dcs-ionos.facis.cloud"],
      "mechanism": "jwks",
      "jwks": { "keys": [ ... ] }
    },
    "https://dcs-osc.facis.cloud/issuer": {
      "purposes": ["peer"],
      "organizations": ["did:web:dcs-osc.facis.cloud"],
      "mechanism": "jwks",
      "jwks": { "keys": [ ... ] }
    }
  }
}
```

### Purposes

| purpose | meaning |
|---|---|
| `login` | may grant a session on **this** instance |
| `peer` | its credentials may be verified in a signing ceremony |
| `pid` | may attest the identity of a natural person (signing ceremonies, QES) |

Purposes are configuration, not policy baked into the code. **An operator
decides which issuers grant access to their deployment** — in production that is
plausibly several: a corporate IdP-backed issuer, a federation issuer, a
customer's own. The verifier enforces only that an issuer stays within the
purposes it was granted.

The demonstration configures the narrow case, because it is the one that makes
the two-party story legible:

```
on instance A:   issuer A → login, peer      PID issuer → pid
on instance B:   issuer B → login, peer      PID issuer → pid
```

A credential from issuer B is refused at A's login because A's operator granted
it nothing — not because the code says so. A different deployment granting
`login` to five issuers is equally valid and needs no code change.

**What `peer` actually gates today.** It is the purpose used when a signing
ceremony verifies the Power of Attorney presented at it. That is the *local*
signatory's PoA: no code path verifies a counterparty's PoA, so the mutual
binding this ADR was written to describe does not exist yet.

Saying otherwise is not harmless. If an operator followed an instruction to
grant `peer` to a counterparty's issuer, a holder of that counterparty's
credentials could satisfy the ceremony here. Until a counterparty-PoA path
exists, `peer` should be granted to the issuers whose PoAs may authorize a
signature **on this instance** — in the demonstration, its own.

**Peers are not enumerated, and `peer_dynamic` ships off.** `peer_dynamic` lets
a did:web-resolvable issuer be verified without an entry, with its key taken from
its own DID document, bounded by the identifier: an issuer at `did:web:X:issuer`
may attest `did:web:X` and nothing else. It was added because listing every
counterparty issuer is an allowlist wearing federation's clothes — whether this
instance deals with a peer at all is already decided fail-closed by the ADR-19
trust gate (the peer's self-signed agreement credential must verify against its
own `did.json` and carry this instance's federation rules hash) and by the local
policy endpoint (`DCS_TRUST_PDP_URL`).

That reasoning is about *membership*, and it does not transfer to `peer`. Given
what `peer` gates today, trusting an unlisted issuer there does not admit a
counterparty — it lets a party authorize a signature on this instance off a
document it publishes about itself, which is self-attestation with extra steps.
The two questions are separate: the gate decides who we deal with, the trust
entry decides whose attestation may authorize a signature here. So the default is
`false`, and it stays false until a counterparty-PoA path exists to give the
setting a meaning that matches its name.

The flag also turns a credential's `iss` into a server-side fetch. Resolution
therefore refuses redirects and will not dial loopback, link-local or multicast
addresses — the cloud metadata endpoint is not a DID registry —, and
`OID4VP_RESOLVER_ALLOWED_HOSTS` narrows it to a named set where a deployment
wants that. Private ranges stay reachable because an in-cluster peer or ORCE
resolver genuinely lives there.

`login` is likewise deliberately **not** dynamic. Who may obtain a session on
this deployment is local policy, and an operator states it explicitly.

### Organization binding

An issuer may only attest organizations listed in its own entry. A credential
whose `organization` is absent from that list is refused regardless of purpose,
so a trusted issuer cannot speak for a party nobody granted it.

**An instance hosts many organizations.** The organization claim is a party
identifier, not the deployment's identity — a single instance legitimately
serves Acme and TechVendor at once. So the rule is about which issuer may name
which party; it is *not* "the organization must be this instance". That
formulation only looks right when a deployment happens to host one party, and it
breaks every multi-tenant one.

Where the issuer *is* the tenant registry for its deployment, enumerating its
organizations in trust configuration would mean editing that file on every
onboarding. Such an issuer declares the explicit wildcard `"*"`. It must be
written out: treating an absent list as "any" is how an issuer silently gains
the right to speak for a party nobody granted it.

`pid` issuers are exempt from the requirement entirely: a PID attests a natural
person, not an organization.

### Revocation

**Every accepted credential is checked against its status list, on every path
and for every purpose.** Login, counterparty PoA, PID: no exception, and no
mechanism opts out — an x5c chain that validates says the issuer signed it, not
that the issuer still stands behind it.

A credential whose status list cannot be reached is refused, not admitted with a
warning. An unreachable revocation list is an unknown revocation state, and a
verifier that treats unknown as valid has no revocation.

### Mechanism

Each issuer declares how its verification key is resolved:

| mechanism | resolution |
|---|---|
| `jwks` | keys bundled in the entry, matched by `kid` (or single-key) |
| `x5c` | certificate chain in the credential header, verified to configured roots |
| `did:jwk` | key decoded from the issuer identifier itself |
| `did:web` | key fetched from the issuer's DID document over HTTPS |
| `orce` | delegated to a configured ORCE flow endpoint |

A mechanism that is declared but not compiled in is **refused at load**, not at
first use: a deployment learns its trust configuration is unsupported when it
starts, not when a wallet arrives. `did:ebsi` and any future scheme reach the
verifier through `orce` without a code change — the flow returns the key, and
ORCE is where a deployment's registry-specific resolution belongs.

### PID issuance

A PID issuer is a **third party** to the relying party. The DCS must not issue
the identity credential it later accepts as proof of who signed; that is the
relying party attesting to itself, and no signature over it means anything
about the signatory.

For the demo this is approximated honestly rather than faked: a PID issuer that
is a separate release with its **own** key and its **own** DID
(`did:web:<host>:pid-issuer`), serving both instances the way a national or QTSP
issuer would, and trusted by each for `pid` and nothing else. It issues
`urn:eudi:pid:de:1` describing a person — given name, family name, date of birth
— and carries neither roles nor an organization, because authority to act for a
party is what a PoA is for and an identity document must not grant permissions.

It presents its **certificate chain** (`x5c`), issued by the CA its own PKI flow
provisions, exactly as a real EUDI PID does — and each DCS trusts it with
`mechanism: x5c`, verifying that chain against anchors configured at
`OID4VP_X5C_TRUST_ANCHORS_PATH`. The dev CA is therefore load-bearing rather
than decorative: it had been provisioned and published all along while nothing
verified against it, because the credential asserted identity through `did:web`
instead.

That makes this a stepping stone rather than a mock to be thrown away: a genuine
national or QTSP issuer differs by an anchor and a trust entry, not by code, and
the demo already exercises the chain-validation path a real PID depends on.

What it is not: a real identity proofing process. Nobody checks that the person
is who the form says. The demo shows the *shape* — a third party attests, the
relying party verifies — and the substance arrives with a real issuer.

## Consequences

- A compromised counterparty issuer can no longer mint a session on this
  instance, nor speak for an organization it was not entitled to.
- The trust file gains structure, and every existing file must be migrated:
  a bare `{jwks}` entry no longer loads. This is deliberate — silently
  defaulting an unscoped entry to "all purposes" would reintroduce exactly the
  conflation this ADR removes.
- Adding a resolution mechanism is a configuration change when it can be
  expressed as an ORCE flow, and a small resolver otherwise. Neither requires
  touching the verification path.
- The demo can state a defensible trust story: each instance grants access only
  on its own issuer's word, and treats identity as something a third party
  attests.
- Mutual Power-of-Attorney binding across instances is **not** implemented. The
  purpose that would carry it exists; the verification path does not. Naming
  that here is the point — the earlier version of this ADR described the
  binding as though it were built, which would have led an operator to grant a
  counterparty's issuer the purpose that authorizes a signature locally.
- QES remains blocked on identity proofing. There is now a third-party PID
  issuer and the chain-validation path it exercises is the real one, but nobody
  verifies that the person is who the form says. The architecture no longer
  hides that.
