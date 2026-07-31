package compiler

import (
	"context"
	"fmt"
	"sync"
)

// signerCtxKey carries the request-scoped Signer the compiler uses to obtain the
// 64-byte ES256 signature for each COSE_Sign1 it emits. pdf-core holds no key
// material: during the stateless "prepare" step the caller injects a
// CapturingSigner, which records each Sig_structure and emits a zeroed
// placeholder; the DCS backend then signs those Sig_structures with its own key
// and posts them back for the "embed" step (InjectCOSESignatures).
type signerCtxKey struct{}

// WithSigner returns a context carrying the Signer the compiler must use for this
// render. A compile that reaches a COSE signature without a Signer in context is
// a programming error and fails loudly.
func WithSigner(ctx context.Context, signer Signer) context.Context {
	return context.WithValue(ctx, signerCtxKey{}, signer)
}

func signerFromContext(ctx context.Context) (Signer, bool) {
	signer, ok := ctx.Value(signerCtxKey{}).(Signer)
	return signer, ok && signer != nil
}

// authorityCtxKey carries the DID of the instance asserting the lifecycle events
// this render writes, which every dcs.lifecycle assertion records as its
// authority. It is request-scoped rather than process-scoped because pdf-core is
// a stateless renderer that several DCS instances may share.
type authorityCtxKey struct{}

// WithLifecycleAuthority returns a context naming the DID that asserts the
// lifecycle events of this render.
func WithLifecycleAuthority(ctx context.Context, did string) context.Context {
	return context.WithValue(ctx, authorityCtxKey{}, did)
}

func lifecycleAuthorityFromContext(ctx context.Context) string {
	did, _ := ctx.Value(authorityCtxKey{}).(string)
	return did
}

// signingChainCtxKey carries the RFC 9360 x5chain (DER, leaf first) a render
// embeds in its COSE_Sign1 protected headers, in place of the process-wide chain
// mustSigningMaterial reads from the environment. It is request-scoped for the
// same reason the lifecycle authority is: each DCS instance signs its manifests
// under its own dcs-c2pa leaf, so the chain belongs to the document being
// rendered, not to whichever pdf-core process renders it.
type signingChainCtxKey struct{}

// WithSigningChain returns a context whose renders embed chain as the COSE
// x5chain. Verification uses it to re-render a manifest under the leaf that
// actually signed it — for a PDF received from a federation peer that is the
// peer's leaf, which this instance cannot otherwise reproduce. An empty chain
// leaves the context untouched, so the configured chain still applies.
func WithSigningChain(ctx context.Context, chain [][]byte) context.Context {
	if len(chain) == 0 {
		return ctx
	}
	return context.WithValue(ctx, signingChainCtxKey{}, chain)
}

func signingChainFromContext(ctx context.Context) ([][]byte, bool) {
	chain, ok := ctx.Value(signingChainCtxKey{}).([][]byte)
	return chain, ok && len(chain) > 0
}

// zeroedCOSESignature is the placeholder a CapturingSigner emits: a 64-byte run
// the "embed" step later overwrites with the real ES256 r||s.
var zeroedCOSESignature = make([]byte, 64)

// CapturingSigner records every COSE Sig_structure the compiler asks it to sign
// and returns a zeroed 64-byte placeholder in its place. After a compile, Captured
// returns those Sig_structures in emission order — the exact bytes the DCS backend
// signs with the dcs-c2pa key, and whose signatures InjectCOSESignatures then
// splices back into the zeroed slots in the same order.
type CapturingSigner struct {
	mu       sync.Mutex
	captured [][]byte
}

// NewCapturingSigner returns a CapturingSigner ready to inject via WithSigner.
func NewCapturingSigner() *CapturingSigner {
	return &CapturingSigner{}
}

func (c *CapturingSigner) Sign(_ context.Context, data []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.captured = append(c.captured, append([]byte(nil), data...))
	return append([]byte(nil), zeroedCOSESignature...), nil
}

// Captured returns the recorded Sig_structures in emission order.
func (c *CapturingSigner) Captured() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.captured))
	copy(out, c.captured)
	return out
}

// signClaimSigStructure builds the COSE Sig_structure over the protected headers
// and detached claim payload and delegates it to the context's Signer.
func signClaimSigStructure(ctx context.Context, protected []byte, claimPayload []byte) ([]byte, error) {
	signer, ok := signerFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no C2PA signer in context: a compile that emits a COSE signature must run under WithSigner")
	}
	sigStructure := cborArray(
		cborText("Signature1"),
		cborBytes(protected),
		cborBytes([]byte{}),
		cborBytes(claimPayload),
	)
	return signer.Sign(ctx, sigStructure)
}
