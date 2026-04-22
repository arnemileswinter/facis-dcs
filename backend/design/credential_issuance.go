package design

import . "goa.design/goa/v3/dsl"

var IssueRoleCredentialRequest = Type("IssueRoleCredentialRequest", func() {
	Description("Request to trigger an OID4VCI credential offer for a DCS role.")

	Token("token", String, "JWT token")

	// The wallet holder's decentralized identifier.
	Attribute("holder_did", String, "DID of the credential recipient wallet.", func() {
		Example("did:web:participant.example.com")
	})
	// The DCS role to embed in the credential.
	Attribute("role", String, "DCS role to assign (must match a declared scope).", func() {
		Example("Contract Creator")
	})

	Required("holder_did", "role")
})

var IssueRoleCredentialResponse = Type("IssueRoleCredentialResponse", func() {
	Description("OID4VCI credential offer returned by the issuance service.")

	// URI the wallet uses to retrieve the credential offer.
	Attribute("offer_uri", String, "OID4VCI credential offer URI to hand to the wallet.")
	// Raw credential offer object as returned by the issuance service.
	Attribute("offer", MapOf(String, Any), "Credential offer object.")

	Required("offer_uri")
})

// CredentialIssuance provides an admin endpoint that triggers an OID4VCI
// credential offer via the OCM-W issuance service. The resulting offer URI
// is handed to the target wallet holder out of band.
var _ = Service("credential_issuance", func() {
	Description("Admin API for issuing role Verifiable Credentials via the OCM-W issuance service.")

	Security(JWTAuth)

	HTTP(func() {
		Path("/credential")
	})

	// IssueRoleCredential triggers an OID4VCI pre-authorized code credential
	// offer for a given DID and role. Returns the offer_uri the wallet must
	// fetch to receive the credential.
	Method("issue_role_credential", func() {
		Description("Create an OID4VCI credential offer for the given DID and DCS role. Restricted to System Administrator.")
		Security(JWTAuth, func() {
			Scope("System Administrator")
		})

		Payload(IssueRoleCredentialRequest)
		Result(IssueRoleCredentialResponse)

		HTTP(func() {
			POST("/offer")
			Response(StatusCreated)
		})
	})
})
