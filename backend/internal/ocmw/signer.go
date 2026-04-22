package ocmw

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SignerConfig holds the runtime parameters for signing JWT credentials via
// the TSA signer service.
type SignerConfig struct {
	URL       string
	Namespace string
	Group     string
	Key       string
}

// signCredentialJWT builds and signs a compact JWS using the TSA signer's
// generic /v1/sign endpoint. The signer returns a raw ES256 signature
// (R||S, 64 bytes, base64-std encoded), which is re-encoded to base64url
// to produce a valid JWT compact serialization.
func signCredentialJWT(ctx context.Context, sc SignerConfig, issuer string, claims map[string]interface{}) (string, error) {
	if strings.TrimSpace(sc.URL) == "" {
		return "", fmt.Errorf("signer URL is not configured")
	}
	if strings.TrimSpace(sc.Key) == "" {
		return "", fmt.Errorf("signer key is not configured")
	}

	header := map[string]interface{}{
		"alg": "ES256",
		"typ": "JWT",
		"kid": issuer + "#" + sc.Key,
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal jwt header: %w", err)
	}
	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal jwt claims: %w", err)
	}

	signingInput := base64.RawURLEncoding.EncodeToString(headerBytes) + "." +
		base64.RawURLEncoding.EncodeToString(claimsBytes)

	reqBody := map[string]string{
		"namespace": sc.Namespace,
		"key":       sc.Key,
		"group":     sc.Group,
		"data":      base64.StdEncoding.EncodeToString([]byte(signingInput)),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal signer request: %w", err)
	}

	url := strings.TrimRight(sc.URL, "/") + "/v1/sign"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build signer request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("call signer: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("signer returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var signResp struct {
		Signature string `json:"signature"`
	}
	if err := json.Unmarshal(respBody, &signResp); err != nil {
		return "", fmt.Errorf("decode signer response: %w", err)
	}
	if signResp.Signature == "" {
		return "", fmt.Errorf("signer returned empty signature")
	}

	rawSig, err := base64.StdEncoding.DecodeString(signResp.Signature)
	if err != nil {
		return "", fmt.Errorf("decode signer signature: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(rawSig), nil
}
