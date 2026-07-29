# ADR-31: Issuer trust is scoped by purpose, bound to an organization, and resolved by a configurable mechanism

Status: accepted
Date: 2026-07-29

## Context

Trust configuration answered one question — "is this issuer's signature
acceptable?" — and every caller reused that single answer. `trust.json` listed
issuers with their JWKS, `IssuerTrusted(iss)` returned a boolean, and login,
counterparty PoA verification and PID verification all consulted it.

That conflates authenticity with authorization, and on a two-instance
federation it fails concretely. Both instances must trust the other's issuer,
because verifying a counterparty's Power of Attorney is the whole point of
peering. But the same entry then also permitted *login*: a PoA minted by
instance A's issuer produced a valid session on instance B. Worse, the
credential's `organization` claim becomes the session's participant identity
and the attribution recorded in the audit trail — so the holder acted **as
A's party inside B**, and the two parties the federation exists to distinguish
collapsed into one.

ADR-29 made each demo issuer stamp its own host as the organization, and a
subsequent fix required a login credential's organization to equal the
instance's own participant DID. That closes the observed hole but leaves the
underlying weakness: nothing binds an *issuer* to the organizations it may
speak for. Any trusted issuer can assert any organization, so the login check
depends on every trusted issuer being well-behaved rather than on a constraint
the verifier enforces.

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
| `peer` | its credentials may be verified when they arrive from a counterparty |
| `pid` | may attest the identity of a natural person (signing ceremonies, QES) |

Purposes are configuration, not policy baked into the code. **An operator
decides which issuers grant access to their deployment** — in production that is
plausibly several: a corporate IdP-backed issuer, a federation issuer, a
customer's own. The verifier enforces only that an issuer stays within the
purposes it was granted.

The demonstration configures the narrow case, because it is the one that makes
the two-party story legible:

```
on instance A:   issuer A → login, peer      issuer B → peer      PID issuer → pid
on instance B:   issuer B → login, peer      issuer A → peer      PID issuer → pid
```

A credential from issuer B therefore verifies on A when it arrives as a
counterparty's PoA, and is refused at A's login — because A's operator granted B
`peer` and not `login`, not because the code says so. A different deployment
granting `login` to five issuers is equally valid and needs no code change.

Peering is deliberately mutual: a signing ceremony verifies both sides' PoA, so
each instance grants `peer` to its counterparty's issuer *and* to its own.

### Organization binding

An issuer may only attest organizations listed in its own entry. A credential
whose `organization` is absent from the issuing entry's `organizations` is
refused regardless of purpose. This is what makes the login rule enforceable
rather than conventional: instance B does not merely check that a login
credential names B's organization, it checks that the issuer was entitled to
say so. An issuer with no `organizations` may attest none — the empty case
fails closed.

`pid` issuers are exempt: a PID attests a natural person, not an organization.

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

For the demo this is approximated honestly rather than faked: a PID issuer
distinct from either DCS issuer, with its **own** trust anchor, trusted only
for `pid`, and presenting its credentials with an `x5c` chain the way a real
EUDI PID does. That exercises the real code path — chain validation against
configured roots — instead of the bare-JWK shortcut, so swapping in a genuine
national or QTSP issuer is a configuration change.

Until such an issuer is deployed, `pid` has no trusted entry and the ceremony
refuses. That is the correct behaviour: a QES claim with no third-party
identity attestation is not a QES claim.

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
  on its own issuer's word, verifies its counterparty's PoA on that
  counterparty's issuer's word, and treats identity as something a third party
  attests.
- QES remains blocked on a real PID issuer. The architecture no longer hides
  that; it names it as a missing trusted entry.
