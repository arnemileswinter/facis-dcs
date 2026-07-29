package oid4vp

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/rego"
)

// trustPolicySource is the default authorization policy. It is the rules that
// used to be `if` statements spread across this file — which issuer may do what,
// on whose behalf — expressed as data a deployment can read, diff and test with
// `opa test` rather than infer from control flow.
//
// A deployment may replace it via OID4VP_TRUST_POLICY_PATH. Nothing
// cryptographic is delegated: chain validation, signature and holder binding,
// and revocation stay in Go. Only the authorization decision is policy.
//
//go:embed policy/trust.rego
var trustPolicySource string

// trustDecision is one evaluation of the policy.
type trustDecision struct {
	Trusted   bool     `json:"trusted"`
	MayAttest bool     `json:"may_attest"`
	Reasons   []string `json:"reasons"`
}

// The policy module does not depend on any one configuration, so it is compiled
// once for the process. The trust document travels as input rather than as a
// bound data document: bound data would be a snapshot taken at first
// evaluation, and a TrustConfig whose fields were changed afterwards would go
// on being judged against the document it had at startup — a difference nothing
// would report.
var (
	preparedTrustPolicy rego.PreparedEvalQuery
	trustPolicyOnce     sync.Once
	trustPolicyErr      error
)

// policyModule returns the policy this deployment evaluates: an operator's file
// when one is configured, otherwise the embedded default.
func policyModule() (string, string, error) {
	path := strings.TrimSpace(os.Getenv("OID4VP_TRUST_POLICY_PATH"))
	if path == "" {
		return "policy/trust.rego", trustPolicySource, nil
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read trust policy %s: %w", path, err)
	}
	return path, string(source), nil
}

func prepareTrustPolicy() (rego.PreparedEvalQuery, error) {
	trustPolicyOnce.Do(func() {
		name, source, err := policyModule()
		if err != nil {
			trustPolicyErr = err
			return
		}
		query, err := rego.New(
			rego.Query("data.dcs.trust"),
			rego.Module(name, source),
		).PrepareForEval(context.Background())
		if err != nil {
			trustPolicyErr = fmt.Errorf("prepare trust policy %s: %w", name, err)
			return
		}
		preparedTrustPolicy = query
	})
	return preparedTrustPolicy, trustPolicyErr
}

// policyDocument renders the trust document as the plain JSON the policy reads.
func (c *TrustConfig) policyDocument() map[string]any {
	issuers := make(map[string]any, len(c.Issuers))
	for iss, entry := range c.Issuers {
		purposes := make([]any, 0, len(entry.Purposes))
		for _, p := range entry.Purposes {
			purposes = append(purposes, string(p))
		}
		organizations := make([]any, 0, len(entry.Organizations))
		for _, org := range entry.Organizations {
			organizations = append(organizations, strings.TrimSpace(org))
		}
		issuers[iss] = map[string]any{
			"purposes":      purposes,
			"organizations": organizations,
			"mechanism":     string(entry.Mechanism),
		}
	}
	return map[string]any{
		"issuers":      issuers,
		"peer_dynamic": c.PeerDynamic,
	}
}

// evaluate asks the policy about one (purpose, issuer, organization).
//
// A policy that cannot be evaluated denies. The alternative — treating an
// unloadable or broken policy as permissive — turns a configuration mistake into
// silent trust, which is the failure mode this whole document exists to prevent.
func (c *TrustConfig) evaluate(purpose Purpose, iss, org string) trustDecision {
	if c == nil {
		return trustDecision{Reasons: []string{"no trust configuration is loaded"}}
	}

	query, err := prepareTrustPolicy()
	if err != nil {
		return trustDecision{Reasons: []string{err.Error()}}
	}

	results, err := query.Eval(context.Background(), rego.EvalInput(map[string]any{
		"purpose":      string(purpose),
		"issuer":       strings.TrimSpace(iss),
		"organization": strings.TrimSpace(org),
		"trust":        c.policyDocument(),
	}))
	if err != nil || len(results) == 0 {
		return trustDecision{Reasons: []string{fmt.Sprintf("trust policy did not produce a decision for issuer %q: %v", iss, err)}}
	}

	raw, err := json.Marshal(results[0].Expressions[0].Value)
	if err != nil {
		return trustDecision{Reasons: []string{fmt.Sprintf("trust policy decision is not readable: %v", err)}}
	}
	var decision trustDecision
	if err := json.Unmarshal(raw, &decision); err != nil {
		return trustDecision{Reasons: []string{fmt.Sprintf("trust policy decision is not readable: %v", err)}}
	}
	return decision
}

// DenialReasons explains why an issuer was refused for this purpose, for the
// error a caller reports. A policy that only answers false is a policy nobody
// can operate.
func (v *PurposeView) DenialReasons(iss, org string) []string {
	if v == nil || v.cfg == nil {
		return []string{"no trust configuration is loaded"}
	}
	return v.cfg.evaluate(v.purpose, iss, org).Reasons
}
