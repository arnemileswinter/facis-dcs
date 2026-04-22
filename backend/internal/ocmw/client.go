package ocmw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	wellknown "github.com/eclipse-xfsc/nats-message-library"
	"github.com/eclipse-xfsc/nats-message-library/common"
	issuermessaging "github.com/eclipse-xfsc/oid4-vci-issuer-service/pkg/messaging"
	"github.com/eclipse-xfsc/oid4-vci-vp-library/model/credential"
	"github.com/google/uuid"
)

const (
	defaultTenantID                  = "tenant_space"
	defaultCredentialConfigurationID = "DCSRoleCredential"
	registrationTTL                  = time.Minute
)

// Config controls how the DCS backend integrates with the OCM-W issuer stack.
type Config struct {
	NATSURL                   string
	PublicIssuerURL           string
	TenantID                  string
	CredentialConfigurationID string

	// Signer configuration used by the issue responder to produce signed
	// JWT verifiable credentials via the TSA signer service.
	SignerURL       string
	SignerNamespace string
	SignerGroup     string
	SignerKey       string
}

// IssuanceClient publishes issuer metadata and requests credential offers over
// the OCM-W NATS/CloudEvents contract.
type IssuanceClient struct {
	natsURL                   string
	publicIssuerURL           string
	tenantID                  string
	credentialConfigurationID string

	registrationMu sync.Mutex
	registeredAt   time.Time
}

// CredentialOfferRequest asks the OCM-W issuer stack to create a pre-authorized
// credential offer for a specific holder DID and DCS role.
type CredentialOfferRequest struct {
	HolderDID      string                 `json:"holderDid"`
	CredentialType string                 `json:"credentialType"`
	Claims         map[string]interface{} `json:"claims"`
}

// CredentialOfferResponse contains the offer link that should be handed to the wallet.
type CredentialOfferResponse struct {
	OfferURI string                 `json:"offerUri"`
	Raw      map[string]interface{} `json:"raw"`
}

// NewIssuanceClient creates a client pointed at the OCM-W NATS interface.
func NewIssuanceClient(cfg Config) *IssuanceClient {
	tenantID := strings.TrimSpace(cfg.TenantID)
	if tenantID == "" {
		tenantID = defaultTenantID
	}

	configurationID := strings.TrimSpace(cfg.CredentialConfigurationID)
	if configurationID == "" {
		configurationID = defaultCredentialConfigurationID
	}

	return &IssuanceClient{
		natsURL:                   strings.TrimSpace(cfg.NATSURL),
		publicIssuerURL:           strings.TrimRight(strings.TrimSpace(cfg.PublicIssuerURL), "/"),
		tenantID:                  tenantID,
		credentialConfigurationID: configurationID,
	}
}

// CreateOffer asks the issuance service to produce an OID4VCI pre-authorized
// credential offer for the configured credential type.
func (c *IssuanceClient) CreateOffer(ctx context.Context, req CredentialOfferRequest) (*CredentialOfferResponse, error) {
	if strings.TrimSpace(c.natsURL) == "" {
		return nil, fmt.Errorf("OCM-W NATS URL is not configured")
	}
	if strings.TrimSpace(c.publicIssuerURL) == "" {
		return nil, fmt.Errorf("OCM-W public issuer URL is not configured")
	}

	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		resp, err := c.createOfferOnce(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// First-time bootstrap can race with OCM-W service startup/registration.
		if ctx.Err() != nil || attempt == 5 {
			break
		}
		time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
	}

	return nil, lastErr
}

func (c *IssuanceClient) createOfferOnce(ctx context.Context, req CredentialOfferRequest) (*CredentialOfferResponse, error) {

	if err := c.ensureRegistered(ctx); err != nil {
		return nil, err
	}

	offerReq := issuermessaging.OfferingURLReq{
		Request: common.Request{
			TenantId:  c.tenantID,
			RequestId: uuid.NewString(),
		},
		Params: issuermessaging.AuthorizationReq{
			Subject: req.HolderDID,
			CredentialConfigurations: []credential.CredentialConfigurationIdentifier{{
				Id: c.credentialConfigurationID,
			}},
			GrantType: "urn:ietf:params:oauth:grant-type:pre-authorized_code",
			Nonce:     uuid.NewString(),
		},
	}

	data, err := json.Marshal(offerReq)
	if err != nil {
		return nil, fmt.Errorf("marshal credential offer request: %w", err)
	}

	client, err := c.newRequestClient(issuermessaging.TopicOffering)
	if err != nil {
		return nil, fmt.Errorf("create issuance request client: %w", err)
	}
	defer client.Close()

	event, err := cloudeventprovider.NewEvent("digital-contracting-service", issuermessaging.EventTypeOffering, data)
	if err != nil {
		return nil, fmt.Errorf("build issuance request event: %w", err)
	}

	respEvent, err := client.RequestCtx(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("issuance service request failed: %w", err)
	}
	if respEvent == nil {
		return nil, fmt.Errorf("issuance service returned no response")
	}

	var offerResp issuermessaging.OfferingURLResp
	if err := json.Unmarshal(respEvent.Data(), &offerResp); err != nil {
		return nil, fmt.Errorf("unmarshal issuance response: %w", err)
	}
	if offerResp.Error != nil {
		return nil, fmt.Errorf("issuance service returned %d: %s", offerResp.Error.Status, offerResp.Error.Msg)
	}

	offerURI := offerResp.CredentialOffer.CredentialOffer
	if offerURI == "" {
		offerURI = offerResp.CredentialOffer.CredentialOfferUri
	}
	if offerURI == "" {
		return nil, fmt.Errorf("issuance service response did not contain an offer URI")
	}
	offerURI = normalizeOfferURIIssuer(offerURI, c.publicCredentialIssuer())

	raw := map[string]interface{}{
		"offerUri":   offerURI,
		"offer":      offerResp.CredentialOffer,
		"code":       offerResp.Code,
		"subject":    offerResp.Subject,
		"tenant_id":  offerResp.TenantId,
		"request_id": offerResp.RequestId,
	}

	return &CredentialOfferResponse{
		OfferURI: offerURI,
		Raw:      raw,
	}, nil
}

func (c *IssuanceClient) ensureRegistered(ctx context.Context) error {
	c.registrationMu.Lock()
	defer c.registrationMu.Unlock()

	if time.Since(c.registeredAt) < registrationTTL {
		return nil
	}

	if err := c.publishIssuerRegistration(ctx); err != nil {
		return err
	}
	if err := c.publishCredentialRegistration(ctx); err != nil {
		return err
	}
	if err := c.waitForRegistration(ctx); err != nil {
		return err
	}

	c.registeredAt = time.Now()
	return nil
}

func (c *IssuanceClient) publishIssuerRegistration(ctx context.Context) error {
	issuer := wellknown.IssuerRegistration{
		Request: common.Request{
			TenantId:  c.tenantID,
			RequestId: uuid.NewString(),
		},
		Issuer: credential.IssuerMetadata{
			CredentialIssuer:                  c.publicCredentialIssuer(),
			CredentialEndpoint:                c.publicCredentialEndpoint(),
			CredentialConfigurationsSupported: map[string]credential.CredentialConfiguration{},
		},
	}

	return c.publishRegistration(ctx, wellknown.TopicIssuerRegistration, wellknown.EventTypeIssuerRegistration, issuer)
}

func (c *IssuanceClient) publishCredentialRegistration(ctx context.Context) error {
	registration := wellknown.CredentialRegistration{
		Request: common.Request{
			TenantId:  c.tenantID,
			RequestId: uuid.NewString(),
		},
		Issuer:          c.publicCredentialIssuer(),
		ConfigurationId: c.credentialConfigurationID,
		CredentialConfiguration: credential.CredentialConfiguration{
			Format:                               "jwt_vc_json",
			Scope:                                c.credentialConfigurationID,
			CryptographicBindingMethodsSupported: []string{"did:key", "did:web", "did:jwk", "jwk"},
			CredentialSigningAlgValuesSupported:  []string{"ES256", "RS256", "PS256"},
			ProofTypesSupported: map[credential.ProofVariant]credential.ProofType{
				credential.ProofVariant("jwt"): {
					ProofSigningAlgValuesSupported: []string{"ES256", "RS256", "PS256"},
				},
			},
			CredentialDefinition: credential.CredentialDefinition{
				Context: []string{"https://www.w3.org/2018/credentials/v1"},
				Type:    []string{"VerifiableCredential", c.credentialConfigurationID},
				CredentialSubject: map[string]credential.CredentialSubject{
					"id":   {},
					"role": {},
				},
			},
			Subject: c.issueSubject(),
			Display: []credential.LocalizedCredential{{
				Name:            "DCS Role Credential",
				Locale:          "en-US",
				BackgroundColor: "#0F172A",
				TextColor:       "#F8FAFC",
			}},
			Schema: map[string]interface{}{
				"$schema": "https://json-schema.org/draft/2020-12/schema",
				"type":    "object",
				"properties": map[string]interface{}{
					"id": map[string]interface{}{"type": "string"},
					// `role` may be a single string or an array of strings.
					// The bootstrap admin credential issues all roles as an
					// array; per-role issuance uses a single string.
					"role": map[string]interface{}{
						"oneOf": []map[string]interface{}{
							{"type": "string"},
							{"type": "array", "items": map[string]interface{}{"type": "string"}},
						},
					},
				},
				"required": []string{"id", "role"},
			},
		},
	}

	return c.publishRegistration(ctx, wellknown.TopicIssuerRegistration, wellknown.EventTypeIssuerCredentialRegistration, registration)
}

func (c *IssuanceClient) publishRegistration(ctx context.Context, topic string, eventType string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal registration payload: %w", err)
	}

	client, err := cloudeventprovider.New(
		cloudeventprovider.Config{
			Protocol: cloudeventprovider.ProtocolTypeNats,
			Settings: cloudeventprovider.NatsConfig{Url: c.natsURL},
		},
		cloudeventprovider.ConnectionTypePub,
		topic,
	)
	if err != nil {
		return fmt.Errorf("create registration publisher: %w", err)
	}
	defer client.Close()

	event, err := cloudeventprovider.NewEvent("digital-contracting-service", eventType, data)
	if err != nil {
		return fmt.Errorf("build registration event: %w", err)
	}

	if err := client.PubCtx(ctx, event); err != nil {
		return fmt.Errorf("publish registration event: %w", err)
	}

	return nil
}

func (c *IssuanceClient) waitForRegistration(ctx context.Context) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		metadata, err := c.getIssuerMetadata(ctx)
		if err == nil {
			if _, ok := metadata.CredentialConfigurationsSupported[c.credentialConfigurationID]; ok {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for issuer registration: %w", ctx.Err())
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for issuer registration")
		case <-ticker.C:
		}
	}
}

func (c *IssuanceClient) getIssuerMetadata(ctx context.Context) (*credential.IssuerMetadata, error) {
	req := wellknown.GetIssuerMetadataReq{
		Request: common.Request{
			TenantId:  c.tenantID,
			RequestId: uuid.NewString(),
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal issuer metadata request: %w", err)
	}

	client, err := c.newRequestClient(wellknown.TopicGetIssuerMetadata)
	if err != nil {
		return nil, fmt.Errorf("create well-known request client: %w", err)
	}
	defer client.Close()

	event, err := cloudeventprovider.NewEvent("digital-contracting-service", wellknown.EventTypeGetIssuerMetadata, data)
	if err != nil {
		return nil, fmt.Errorf("build well-known event: %w", err)
	}

	respEvent, err := client.RequestCtx(ctx, event)
	if err != nil {
		return nil, fmt.Errorf("request issuer metadata: %w", err)
	}
	if respEvent == nil {
		return nil, fmt.Errorf("well-known service returned no response")
	}

	var reply wellknown.GetIssuerMetadataReply
	if err := json.Unmarshal(respEvent.Data(), &reply); err != nil {
		return nil, fmt.Errorf("unmarshal issuer metadata: %w", err)
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("well-known service returned %d: %s", reply.Error.Status, reply.Error.Msg)
	}
	if reply.Issuer == nil {
		return nil, fmt.Errorf("well-known service returned empty issuer metadata")
	}

	return reply.Issuer, nil
}

func (c *IssuanceClient) newRequestClient(topic string) (*cloudeventprovider.CloudEventProviderClient, error) {
	return cloudeventprovider.New(
		cloudeventprovider.Config{
			Protocol: cloudeventprovider.ProtocolTypeNats,
			Settings: cloudeventprovider.NatsConfig{Url: c.natsURL},
		},
		cloudeventprovider.ConnectionTypeReq,
		topic,
	)
}

func (c *IssuanceClient) publicCredentialIssuer() string {
	return c.publicIssuerURL + "/v1/tenants/" + c.tenantID
}

func (c *IssuanceClient) publicCredentialEndpoint() string {
	return c.publicCredentialIssuer() + "/credential"
}

func (c *IssuanceClient) issueSubject() string {
	return strings.ToLower("digital-contracting-service." + c.tenantID + "." + c.credentialConfigurationID)
}

// normalizeOfferURIIssuer keeps the offer's credential_issuer aligned with the
// configured public issuer URL. This ensures externally visible offers use the
// ingress/public route even when upstream metadata is stale.
func normalizeOfferURIIssuer(offerURI, credentialIssuer string) string {
	if strings.TrimSpace(offerURI) == "" || strings.TrimSpace(credentialIssuer) == "" {
		return offerURI
	}

	base, rawQuery, found := strings.Cut(offerURI, "?")
	if !found {
		return offerURI
	}

	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return offerURI
	}

	encodedOffer := query.Get("credential_offer")
	if encodedOffer == "" {
		return offerURI
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(encodedOffer), &payload); err != nil {
		return offerURI
	}

	payload["credential_issuer"] = strings.TrimRight(strings.TrimSpace(credentialIssuer), "/")

	updatedPayload, err := json.Marshal(payload)
	if err != nil {
		return offerURI
	}

	query.Set("credential_offer", string(updatedPayload))
	return base + "?" + query.Encode()
}
