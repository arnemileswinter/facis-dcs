# Check the credential status of a signature

## Purpose

The Signature Compliance Viewer shows the current status of the power of
attorney credential presented at signing. When the credential has several
status entries, one valid entry is not enough to show it as valid.

## Roles

You need an authorized login for the Compliance Viewer and a signed contract
that the viewer shows.

## Steps

1. Open the **Signature Compliance Viewer** and select the contract.
2. Open **Compliance Checks**.
3. Under **Signature level detail**, read the **Credential status** column for
   the signature and any error reason shown below it.

## Status values

- **VALID:** all status entries are valid and the check accepts the
  credential. This says nothing about the other signature checks.
- **REVOKED:** at least one status entry marks the credential as revoked.
- **SUSPENDED:** at least one status entry marks the credential as suspended,
  and none marks it as revoked.
- **UNKNOWN:** the status cannot be determined, for example because a status
  list cannot be reached, the credential is missing, or the trust check fails.
  Read the reason shown. UNKNOWN does not confirm validity.

When several entries are not valid, the viewer shows the most severe one:
REVOKED, then SUSPENDED, then any other state.
