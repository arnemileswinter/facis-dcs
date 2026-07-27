# ADR-26: On a signed contract, signature integrity outranks the C2PA hard binding

Status: Accepted (2026-07-27).

## Context

A C2PA manifest's `c2pa.hash.data` assertion binds the whole file. Anything
appended after the manifest is written changes the bytes it hashed, so the
binding no longer matches. **The manifest must therefore be last.**

A PAdES signature covers the document up to its own `/ByteRange`. It commits to
what the signer saw. **The signature must therefore be last.**

Both cannot be last, and a contract's signing flow needs both. The current
order is deliberate — `stampLifecycleForSigning` writes the lifecycle manifest
*before* PAdES signing, "so the signature commits to the PDF's final
lifecycle-bearing content, and the signed artefact never needs a
post-signature revision".

The observable consequence, on artifacts produced by the two-instance vertical:

| Stage | C2PA verdict |
|---|---|
| offer, counter, counter, settle (both instances) | **Valid** |
| signed, double-signed (both instances) | `assertion.dataHash.mismatch` |

The signature layer appends roughly 99 KB and three PDF revisions after the
last manifest, so that manifest's hard binding cannot match the file it now
lives in.

## Decision

**Signature integrity wins.** The signing order stays: provenance is written,
then the signature commits to it.

A contract's signature must verify cleanly in Acrobat and other external PDF
tools. That is what a counterparty, a court or an auditor actually checks, and
a signature reported as "valid, but the document has been modified since it was
signed" is materially worse than a hard binding that stops where the signature
starts. Appending a C2PA manifest after signing would buy a green C2PA verdict
at exactly that price, so it is rejected.

**C2PA provenance must hold through offer and negotiation**, where no PAdES
layer exists and no such conflict arises. Those artifacts validate, and a
regression there is a defect.

**A `dataHash` mismatch on a PAdES-signed contract is not tampering** and must
not be reported as such. It is the arithmetic consequence of this ADR. Anything
that reports on provenance — verification endpoints, the compliance views, test
assertions — distinguishes the two cases:

- mismatch on a document carrying a PAdES signature: expected, the binding
  covers the document up to the signature;
- mismatch on a document without one: a real integrity failure.

## Consequences

The signed artifact's provenance is still readable and still meaningful: the
manifest chain, its ingredients, the lifecycle assertions and their signatures
are all intact and verifiable. What cannot hold is the *whole-file* hash of the
last manifest.

Demonstrations may show C2PA validation through the negotiation and must not
claim a green C2PA verdict on the signed artifact. Saying why is a design
statement, not an apology: the signature covers the provenance, so the
provenance cannot also cover the signature.

`assertion.dataHash.mismatch` joins `signingCredential.untrusted` as a status
that is expected under stated conditions, so a validation gate has to be
condition-aware rather than treating any failure as fatal.

## Alternatives considered

**Append a C2PA update manifest after signing.** Restores the hard binding and
keeps the signature cryptographically valid, since an incremental update leaves
the signed byte range untouched. Rejected: PDF readers report the document as
modified after signing, which is precisely the outcome this ADR ranks lowest.

**Exclude the signature region from the hard binding.** C2PA data-hash
assertions carry exclusion ranges, and the compiler already emits them for the
manifest stream. Rejected as not currently reachable: the range is fixed when
the manifest is written, and signing happens externally (wallet plus DSS)
afterwards, so its size and offset are unknown at that point. It becomes
possible only if the signature occupies a pre-allocated, fixed-size region
filled in place rather than appended — a different signing pipeline, and the
avenue to revisit if both properties are ever required at once.

**Drop PAdES and rely on C2PA alone.** Rejected: C2PA is not an electronic
signature under eIDAS, and external tools cannot check it.
