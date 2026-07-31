package provenance

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeXFSCStatusListResponse builds a response matching the ACTUAL shape
// returned by the deployed XFSC statuslist-service
// (deployment/helm/charts/statuslist-service): a plain {"list", "listId",
// "tenantId"} object, gzip-compressed, standard base64, LSB bit packing.
func makeXFSCStatusListResponse(bitstringLen int, setIndex uint32, revoked bool) []byte {
	bitstring := make([]byte, bitstringLen)
	if revoked {
		byteIdx := setIndex / 8
		bitIdx := uint(setIndex % 8)
		bitstring[byteIdx] |= 1 << bitIdx
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, _ = w.Write(bitstring)
	_ = w.Close()
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	resp := map[string]interface{}{
		"list":     encoded,
		"listId":   1,
		"tenantId": "default",
	}
	b, _ := json.Marshal(resp)
	return b
}

// TestOCMWStatusListPublisher_PublishStatus_TerminalStatesCallRevoke verifies
// that all terminal states — including the uppercase forms emitted by the CWE
// (DCS-OR-C2PA-005 Gap 1) — trigger a revocation POST to the status list service.
func TestOCMWStatusListPublisher_PublishStatus_TerminalStatesCallRevoke(t *testing.T) {
	for _, state := range []string{
		"terminated", "TERMINATED",
		"expired", "EXPIRED",
		"replaced", "REPLACED",
		"suspended", "SUSPENDED",
	} {
		t.Run(state, func(t *testing.T) {
			revokeCalled := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/revoke/") {
					revokeCalled = true
					w.WriteHeader(http.StatusOK)
					_, err := fmt.Fprintf(w, `{"tenantId":"default","listId":1,"index":0,"status":"revoked"}`)
					if err != nil {
						log.Println("could not write response:", err)
					}
					return
				}
				http.NotFound(w, r)
			}))
			defer srv.Close()

			p := newTestPublisher(srv.URL, "default")
			ref, err := p.PublishStatus(context.Background(), "did:example:contract1", state, "test reason", time.Now())
			require.NoError(t, err, "state %q should not error", state)
			assert.True(t, revokeCalled, "state %q must POST to /revoke/ endpoint", state)
			assert.Contains(t, ref.StatusListCredential, "/v1/tenants/")
		})
	}
}

// TestOCMWStatusListPublisher_PublishStatus_NonTerminalStatesDoNotRevoke verifies
// that active, draft, and amended states do NOT call the revoke endpoint.
func TestOCMWStatusListPublisher_PublishStatus_NonTerminalStatesDoNotRevoke(t *testing.T) {
	for _, state := range []string{"active", "draft", "amended", "ACTIVE", "DRAFT", "AMENDED"} {
		t.Run(state, func(t *testing.T) {
			revokeCalled := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/revoke/") {
					revokeCalled = true
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			p := newTestPublisher(srv.URL, "default")
			_, err := p.PublishStatus(context.Background(), "did:example:contract1", state, "", time.Now())
			require.NoError(t, err)
			assert.False(t, revokeCalled, "state %q must NOT call /revoke/ endpoint", state)
		})
	}
}

// TestOCMWStatusListPublisher_RevokeStatus_CallsCorrectPath verifies the
// revoke endpoint path and response parsing.
func TestOCMWStatusListPublisher_RevokeStatus_CallsCorrectPath(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w, `{"tenantId":"default","listId":1,"index":42,"status":"revoked"}`)
		if err != nil {
			log.Println("could not write response:", err)
		}
	}))
	defer srv.Close()

	p := newTestPublisher(srv.URL, "default")
	ref, err := p.RevokeStatus(context.Background(), "did:example:contractX")
	require.NoError(t, err)
	assert.Contains(t, capturedPath, "/v1/tenants/default/status/1/revoke/", "revoke path must contain tenant and list ID")
	assert.Contains(t, ref.StatusListCredential, "/v1/tenants/default/status/1", "returned URI must point to status list endpoint")
}

// TestOCMWStatusListPublisher_RevokeStatus_PropagatesHTTPError verifies that
// a non-2xx response from the status list service propagates as an error
// (consistent with hard-fail policy for required external deps).
func TestOCMWStatusListPublisher_RevokeStatus_PropagatesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newTestPublisher(srv.URL, "default")
	_, err := p.RevokeStatus(context.Background(), "did:example:contract1")
	require.Error(t, err, "HTTP 500 from status list must propagate as error")
	assert.Contains(t, err.Error(), "statuslist-service revoke returned 500")
}

// TestOCMWStatusListPublisher_EmptyServiceURL_HardFails verifies that a
// publisher with no URL configured returns an error for terminal states.
// Empty ServiceURL is a hard failure per project policy (DCS hard-fail policy).
func TestOCMWStatusListPublisher_EmptyServiceURL_HardFails(t *testing.T) {
	p := newTestPublisher("", "")
	_, err := p.PublishStatus(context.Background(), "did:example:c1", "terminated", "", time.Now())
	require.Error(t, err, "empty ServiceURL must hard-fail for terminal states (revocation is mandatory)")
	assert.Contains(t, err.Error(), "ServiceURL must not be empty")
}

// TestOCMWStatusListPublisher_DefaultTenant verifies that an empty tenantID
// defaults to "default" in the endpoint path.
func TestOCMWStatusListPublisher_DefaultTenant(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w, `{"status":"revoked"}`)
		if err != nil {
			log.Printf("could not write response: %v", err)
		}
	}))
	defer srv.Close()

	p := newTestPublisher(srv.URL, "") // empty tenant
	_, err := p.RevokeStatus(context.Background(), "contract-abc")
	require.NoError(t, err)
	assert.Contains(t, capturedPath, "/default/", "empty tenantID must default to 'default'")
}

// TestPublishedEntryIsTheEntryTheRevokeCallFlips: the reference a credential
// carries and the bit a revocation sets are read from the same allocation, so a
// verifier that follows the credential lands on the bit that was flipped.
func TestPublishedEntryIsTheEntryTheRevokeCallFlips(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newTestPublisher(srv.URL, "default")
	advertised, err := p.PublishStatus(context.Background(), "did:example:contract-flip", "active", "", time.Now())
	require.NoError(t, err)

	revoked, err := p.RevokeStatus(context.Background(), "did:example:contract-flip")
	require.NoError(t, err)

	assert.Equal(t, advertised, revoked)
	assert.True(t, strings.HasSuffix(capturedPath, fmt.Sprintf("/revoke/%d", advertised.Index)),
		"revoke POST %s must name the advertised entry %d", capturedPath, advertised.Index)
}

// TestStatusListURI_Format verifies the URI returned by statusListURI matches
// the expected XFSC statuslist-service path format.
func TestStatusListURI_Format(t *testing.T) {
	p := newTestPublisher("http://statuslist:8080", "acme")
	uri := p.statusListURI(StatusListEntry{ListID: DefaultListID})
	assert.Equal(t, "http://statuslist:8080/v1/tenants/acme/status/1", uri)
}

// TestReadUnsignedStatusList_ActiveBitNotSet verifies "active" is returned when
// the bitstring bit at the contract's index is 0, against the ACTUAL XFSC
// statuslist-service response shape ({"list", "listId", "tenantId"}, gzip, LSB).
func TestReadUnsignedStatusList_ActiveBitNotSet(t *testing.T) {
	const idx = uint32(4711)

	body := makeXFSCStatusListResponse(int(listSize/8), idx, false /* not revoked */)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(body)
		if err != nil {
			log.Printf("could not write response: %v", err)
		}
	}))
	defer srv.Close()

	status, err := ReadUnsignedStatusList(context.Background(), srv.Client(), srv.URL, idx)
	require.NoError(t, err)
	assert.Equal(t, "active", status)
}

// TestReadUnsignedStatusList_RevokedBitSet verifies "revoked" is returned when
// the bit at the contract's index is 1, against the ACTUAL XFSC
// statuslist-service response shape ({"list", "listId", "tenantId"}, gzip, LSB).
func TestReadUnsignedStatusList_RevokedBitSet(t *testing.T) {
	const idx = uint32(88123)

	body := makeXFSCStatusListResponse(int(listSize/8), idx, true /* revoked */)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(body)
		if err != nil {
			log.Printf("could not write response: %v", err)
		}
	}))
	defer srv.Close()

	status, err := ReadUnsignedStatusList(context.Background(), srv.Client(), srv.URL, idx)
	require.NoError(t, err)
	assert.Equal(t, "revoked", status)
}

// TestReadUnsignedStatusList_HTTPErrorPropagates verifies that a non-200 response
// from the status list service is returned as an error (hard-fail policy).
func TestReadUnsignedStatusList_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := ReadUnsignedStatusList(context.Background(), srv.Client(), srv.URL, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

// TestReadUnsignedStatusList_MissingEncodedList verifies that a response with
// neither "list" nor credentialSubject.encodedList returns an error.
func TestReadUnsignedStatusList_MissingEncodedList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(`{"credentialSubject":{}}`))
		if err != nil {
			log.Printf("could not write response: %v", err)
		}
	}))
	defer srv.Close()

	_, err := ReadUnsignedStatusList(context.Background(), srv.Client(), srv.URL, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no list field")
}

// A reading off an unsigned list is never rendered as the bare word a reader
// takes for an established verdict. The list carries no signature, so
// "revoked"/"active" is what answered the URL and nothing more (ADR-34); both
// report writers render through this one helper so they cannot disagree about
// how much the reading is worth.
func TestUnverifiedStatusReadingDoesNotStateTheReadingAsFact(t *testing.T) {
	for _, state := range []string{"active", "revoked"} {
		rendered := UnverifiedStatusReading(state)
		assert.NotEqual(t, state, rendered,
			"a bare %q reads as an established revocation state", state)
		assert.Contains(t, rendered, state, "the reading itself must still be legible")
		assert.Contains(t, rendered, "UNVERIFIED")
	}
}

// The failure named is the one that occurred. Calling every failure an
// unreachable service asserted a cause that was never established and pointed
// whoever read the report at the network, when the list had been served in full
// and was simply unusable.
func TestUnverifiedStatusUnavailableNamesTheFailureItHad(t *testing.T) {
	rendered := UnverifiedStatusUnavailable(fmt.Errorf("index 9 out of range for bitstring of 1 bytes"))
	assert.Contains(t, rendered, "UNKNOWN")
	assert.Contains(t, rendered, "index 9 out of range")
	assert.NotContains(t, strings.ToLower(rendered), "unreachable")
}
