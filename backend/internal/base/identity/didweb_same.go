package identity

import "strings"

// SameDIDWeb reports whether two did:web identifiers name the same peer.
//
// The authority is case-insensitive because DNS is, and because DIDWebPath
// normalises it — so a peer DID stored before that normalisation, or spelled
// differently by a counterparty, denotes the same host. Path segments are
// compared exactly: two instances can share a host and be told apart only by
// their path, so folding those would merge distinct peers.
//
// Comparing the raw strings instead leaves each callsite deciding, and the
// callsites disagreed: the trust gate resolved a case-varied self-DID to this
// instance while the same-peer guard in front of it did not.
func SameDIDWeb(a, b string) bool {
	if a == b {
		return true
	}
	aHost, aSegments, err := DIDWebPath(a)
	if err != nil {
		return false
	}
	bHost, bSegments, err := DIDWebPath(b)
	if err != nil {
		return false
	}
	if aHost != bHost || len(aSegments) != len(bSegments) {
		return false
	}
	for i := range aSegments {
		if aSegments[i] != bSegments[i] {
			return false
		}
	}
	return true
}

// NormalizeDIDWeb returns the canonical spelling of a did:web identifier, or the
// input unchanged when it is not one this resolver can parse.
func NormalizeDIDWeb(did string) string {
	host, segments, err := DIDWebPath(did)
	if err != nil {
		return did
	}
	// The port is re-encoded: a bare colon in the authority is a path segment
	// separator in did:web, so emitting it raw turns one identifier into another.
	authority := strings.Replace(host, ":", "%3A", 1)
	if len(segments) == 0 {
		return "did:web:" + authority
	}
	return "did:web:" + authority + ":" + strings.Join(segments, ":")
}
