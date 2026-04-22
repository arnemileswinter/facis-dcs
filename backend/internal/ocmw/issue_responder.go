package ocmw

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"digital-contracting-service/internal/base/datatype/userrole"

	cev2 "github.com/cloudevents/sdk-go/v2/event"
	cloudeventprovider "github.com/eclipse-xfsc/cloud-event-provider"
	"github.com/eclipse-xfsc/nats-message-library/common"
	issuermessaging "github.com/eclipse-xfsc/oid4-vci-issuer-service/pkg/messaging"
)

// StartIssueResponder starts a NATS request/reply consumer for
// <subject>.issue used by the OCM-W issuance service.
func StartIssueResponder(ctx context.Context, cfg Config) error {
	tenantID := strings.TrimSpace(cfg.TenantID)
	if tenantID == "" {
		tenantID = defaultTenantID
	}

	configurationID := strings.TrimSpace(cfg.CredentialConfigurationID)
	if configurationID == "" {
		configurationID = defaultCredentialConfigurationID
	}

	natsURL := strings.TrimSpace(cfg.NATSURL)
	if natsURL == "" {
		return fmt.Errorf("OCM-W NATS URL is not configured")
	}

	publicIssuerURL := strings.TrimRight(strings.TrimSpace(cfg.PublicIssuerURL), "/")
	if publicIssuerURL == "" {
		return fmt.Errorf("OCM-W public issuer URL is not configured")
	}

	subject := strings.ToLower("digital-contracting-service."+tenantID+"."+configurationID) + ".issue"

	client, err := cloudeventprovider.New(
		cloudeventprovider.Config{
			Protocol: cloudeventprovider.ProtocolTypeNats,
			Settings: cloudeventprovider.NatsConfig{Url: natsURL},
		},
		cloudeventprovider.ConnectionTypeRep,
		subject,
	)
	if err != nil {
		return fmt.Errorf("create issue responder client: %w", err)
	}

	go func() {
		<-ctx.Done()
		_ = client.Close()
	}()

	go func() {
		_ = client.ReplyCtx(ctx, func(replyCtx context.Context, event cev2.Event) (*cev2.Event, error) {
			return buildIssueResponse(event, publicIssuerURL, tenantID, configurationID)
		})
	}()

	return nil
}

func buildIssueResponse(event cev2.Event, publicIssuerURL, tenantID, configurationID string) (*cev2.Event, error) {
	var req issuermessaging.IssuanceModuleReq
	if err := json.Unmarshal(event.Data(), &req); err != nil {
		payload := issuermessaging.IssuanceModuleRep{
			Reply: common.Reply{
				Error: &common.Error{Status: 400, Msg: "invalid issue request"},
			},
		}
		return marshalIssueReply(payload)
	}

	now := time.Now().UTC()
	issuedAt := now.Unix()
	notBefore := issuedAt
	expiresAt := now.Add(24 * time.Hour).Unix()

	// The signer requires credentialSubject.id to be a URI. The OCM-W
	// `Subject` is an opaque digest, so derive the holder DID from the
	// proof JWT's `kid` header (set by the wallet to the binding DID).
	subjectID := holderDIDFromProof(req.Holder)
	if subjectID == "" {
		subjectID = req.Subject
	}

	credential := map[string]interface{}{
		"iss": publicIssuerURL + "/v1/tenants/" + tenantID,
		"sub": subjectID,
		"iat": issuedAt,
		"nbf": notBefore,
		"exp": expiresAt,
		"vc": map[string]interface{}{
			"@context": []interface{}{
				"https://www.w3.org/2018/credentials/v1",
				map[string]interface{}{
					"@version":          1.1,
					"@protected":        true,
					"dcs":               "https://facis-dcs.local/vocab#",
					"DCSRoleCredential": "dcs:DCSRoleCredential",
					"role":              "dcs:role",
				},
			},
			"type": []string{"VerifiableCredential", configurationID},
			"credentialSubject": map[string]interface{}{
				"id":   subjectID,
				"role": allRoleNames(),
			},
		},
	}

	credentialJWT, err := buildUnsignedJWT(credential)
	if err != nil {
		payload := issuermessaging.IssuanceModuleRep{
			Reply: common.Reply{
				TenantId:  req.TenantId,
				RequestId: req.RequestId,
				GroupId:   req.GroupId,
				Error:     &common.Error{Status: 500, Msg: "failed to build credential"},
			},
		}
		return marshalIssueReply(payload)
	}

	payload := issuermessaging.IssuanceModuleRep{
		Reply: common.Reply{
			TenantId:  req.TenantId,
			RequestId: req.RequestId,
			GroupId:   req.GroupId,
		},
		Credential: credentialJWT,
		Format:     "jwt_vc_json",
	}

	return marshalIssueReply(payload)
}

func marshalIssueReply(payload issuermessaging.IssuanceModuleRep) (*cev2.Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	replyEvent, err := cloudeventprovider.NewEvent("digital-contracting-service", "credential.issue.reply.v1", data)
	if err != nil {
		return nil, err
	}

	return &replyEvent, nil
}

// allRoleNames returns every defined UserRole as strings. The bootstrap
// admin credential carries them all so the first user can fully operate the
// system. Once per-subject role assignment is wired in, this should be
// replaced with a lookup keyed on the holder DID.
func allRoleNames() []string {
	all := userrole.All()
	names := make([]string, 0, len(all))
	for _, r := range all {
		names = append(names, r.String())
	}
	return names
}

func buildUnsignedJWT(claims map[string]interface{}) (string, error) {
	header := map[string]interface{}{"alg": "none", "typ": "JWT"}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := base64.RawURLEncoding.EncodeToString(headerBytes)
	encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)

	return encodedHeader + "." + encodedClaims + ".", nil
}

// holderDIDFromProof extracts the holder DID from a wallet proof JWT.
// The wallet sets the JWT header `kid` to <DID>#<key-id>; the DID alone
// (without the fragment) is a valid URI suitable for credentialSubject.id.
func holderDIDFromProof(proofJWT string) string {
	proofJWT = strings.TrimSpace(proofJWT)
	if proofJWT == "" {
		return ""
	}
	parts := strings.Split(proofJWT, ".")
	if len(parts) < 2 {
		return ""
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ""
	}
	var header struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return ""
	}
	kid := strings.TrimSpace(header.Kid)
	if kid == "" {
		return ""
	}
	if i := strings.Index(kid, "#"); i >= 0 {
		kid = kid[:i]
	}
	return kid
}
