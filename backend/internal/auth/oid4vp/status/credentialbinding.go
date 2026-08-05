package status

import "strings"

// RequireCredentialIssuer binds a status list to the credential whose status it
// carries: the list must be issued by the same issuer as that credential.
//
// Without it, a list says only that SOME issuer this deployment trusts published
// it. Any of them could then publish a list at another issuer's URI and un-revoke
// that issuer's credentials — the signature verifies, the subject matches the
// reference URI, and nothing has asked whose revocation statement it actually is.
// That is the outstanding defect ADR-34 recorded against this package, and it is
// this project's verification logic rather than a property of the stand-in
// issuers.
//
// It is also what lets a deployment hold more than one CA trust list (ADR-35)
// without the status-list path quietly widening them back into one: the verifier
// is reached without a purpose in hand, so the anchors it holds are the union,
// and this is the check that stops a list anchored for one purpose speaking for
// a credential admitted under another.
//
// A list that declares no issuer at all is left to the binding its own format
// provides: a standard CWT status list need carry no `iss`, and its key is
// resolved by a lookup already scoped to the reference URI, so it cannot be
// signed by a key registered for some other list. Refusing it here would reject
// a conformant list over a claim the spec does not require.
//
// A CREDENTIAL that names no issuer is refused, because one cannot exist —
// resolution needs `iss` to find a key at all, so its absence means the caller
// verified the credential some other way and there is nothing to bind to.
func RequireCredentialIssuer(credential VerifiedCredential, listIssuer string) error {
	listIssuer = strings.TrimSpace(listIssuer)
	if listIssuer == "" {
		return nil
	}

	credentialIssuer := strings.TrimSpace(CredentialIssuer(credential))

	if credentialIssuer == "" || credentialIssuer != listIssuer {
		return ErrStatusListIssuerMismatch
	}
	return nil
}

// CredentialIssuer is the issuer a verified credential names, in either spelling
// this codebase carries: `iss` on an SD-JWT, `issuer` on a W3C VC — where it is
// a bare string or an object with an `id`.
func CredentialIssuer(credential VerifiedCredential) string {
	if credential.Claims == nil {
		return ""
	}
	if iss, ok := credential.Claims["iss"].(string); ok && strings.TrimSpace(iss) != "" {
		return iss
	}
	switch issuer := credential.Claims["issuer"].(type) {
	case string:
		return issuer
	case map[string]any:
		id, _ := issuer["id"].(string)
		return id
	}
	return ""
}
