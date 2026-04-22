package design

import (
	. "goa.design/goa/v3/dsl"
)

// Auth Service — OIDC/Keycloak authentication endpoints.
// All methods use NoSecurity() since they handle the authentication flow itself.
var _ = Service("Auth", func() {
	Description("Authentication endpoints for OIDC/Keycloak login, callback, and token refresh.")

	Method("login", func() {
		Description("Returns the OIDC authorization URL for initiating the login flow.")
		NoSecurity()
		Result(func() {
			Attribute("auth_url", String, "OIDC authorization URL")
			Required("auth_url")
		})
		HTTP(func() {
			GET("/auth/login")
			Response(StatusOK)
		})
	})

	Method("callback", func() {
		Description("Handles the OIDC callback, exchanges authorization code for tokens, sets refresh token cookie, and redirects to /auth/success.")
		NoSecurity()
		Payload(func() {
			Attribute("code", String, "Authorization code from OIDC provider")
			Attribute("state", String, "Opaque state returned by OIDC provider")
			Required("code")
		})
		Result(func() {
			Attribute("location", String, "Redirect location")
			Required("location")
		})
		HTTP(func() {
			GET("/auth/callback")
			Param("code")
			Param("state")
			Response(StatusFound, func() {
				Header("location:Location")
			})
		})
	})

	Method("refresh", func() {
		Description("Exchanges a refresh token (from HttpOnly cookie) for a new access token.")
		NoSecurity()
		Result(func() {
			Attribute("access_token", String, "JWT access token")
			Attribute("token_type", String, "Token type (Bearer)")
			Attribute("expires_in", Int, "Token expiry in seconds")
			Required("access_token", "token_type", "expires_in")
		})
		HTTP(func() {
			POST("/auth/refresh")
			Response(StatusOK)
		})
	})

	Method("logout", func() {
		Description("Returns the OIDC logout URL for initiating the logout flow.")
		NoSecurity()
		Result(func() {
			Attribute("logout_url", String, "OIDC logout URL")
			Required("logout_url")
		})
		HTTP(func() {
			GET("/auth/logout")
			Response(StatusOK)
		})
	})

	Method("logoutComplete", func() {
		Description("logout callback. Clears refresh token cookie and redirects to home.")
		NoSecurity()
		Result(func() {
			Attribute("location", String, "Redirect location")
			Required("location")
		})
		HTTP(func() {
			GET("/auth/logout-complete")
			Response(StatusFound, func() {
				Header("location:Location")
			})
		})
	})

	Method("presentationRequest", func() {
		Description("Generates an OpenID4VP presentation request URI for cross-device wallet-based authentication.")
		NoSecurity()
		Payload(func() {
			Attribute("login_challenge", String, "Hydra login challenge to complete after VP verification")
		})
		Result(func() {
			Attribute("presentation_uri", String, "OpenID4VP presentation request URI")
			Required("presentation_uri")
		})
		HTTP(func() {
			GET("/auth/presentation-request")
			Param("login_challenge")
			Response(StatusOK)
		})
	})

	Method("presentationCallback", func() {
		Description("Receives a Verifiable Presentation from a wallet and establishes an authenticated session.")
		NoSecurity()
		Payload(func() {
			Attribute("vp_token", String, "Verifiable Presentation JWT")
			Attribute("presentation_submission", String, "JSON presentation submission metadata")
			Attribute("state", String, "OpenID4VP state parameter used for request/response correlation")
			Attribute("request_id", String, "Legacy correlation identifier (deprecated; use state)")
			Required("vp_token")
		})
		Result(func() {
			Attribute("location", String, "Redirect location")
			Required("location")
		})
		HTTP(func() {
			POST("/auth/presentation-callback")
			Param("vp_token")
			Param("presentation_submission")
			Param("state")
			Param("request_id")
			Response(StatusFound, func() {
				Header("location:Location")
			})
		})
	})

	Method("presentationStatus", func() {
		Description("Returns whether a cross-device presentation request has completed verification.")
		NoSecurity()
		Payload(func() {
			Attribute("request_id", String, "Presentation request correlation identifier")
			Required("request_id")
		})
		Result(func() {
			Attribute("completed", Boolean, "Whether presentation was completed")
			Attribute("location", String, "Redirect location after completion")
			Required("completed")
		})
		HTTP(func() {
			GET("/auth/presentation-status/{request_id}")
			Response(StatusOK)
		})
	})

	Method("consent", func() {
		Description("Accepts a Hydra consent challenge for the first-party DCS client and returns the redirect URL to the OIDC callback.")
		NoSecurity()
		Payload(func() {
			Attribute("consent_challenge", String, "Hydra consent challenge from the /ui/?consent_challenge= redirect")
			Required("consent_challenge")
		})
		Result(func() {
			Attribute("location", String, "Redirect location (Hydra-issued)")
			Required("location")
		})
		HTTP(func() {
			GET("/auth/consent")
			Param("consent_challenge")
			Response(StatusFound, func() {
				Header("location:Location")
			})
		})
	})
})
