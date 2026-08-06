package ipfs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateFileAddsToKuboAndCopiesToMFS(t *testing.T) {
	var addCalled bool
	var mfsCopyCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v0/add":
			addCalled = true
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for Kubo add, got %s", r.Method)
			}
			if err := r.ParseMultipartForm(1024); err != nil {
				t.Fatalf("parse multipart form: %v", err)
			}
			_, fileHeader, err := r.FormFile("file")
			if err != nil {
				t.Fatalf("read multipart file: %v", err)
			}
			if fileHeader.Filename != "audit-log.json" {
				t.Fatalf("unexpected file name %q", fileHeader.Filename)
			}
			_, _ = w.Write([]byte(`{"Hash":"bafy-test-cid"}`))
		case "/api/v0/files/cp":
			mfsCopyCalled = true
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST for Kubo files/cp, got %s", r.Method)
			}
			if got := r.URL.Query()["arg"]; len(got) != 2 || got[0] != "/ipfs/bafy-test-cid" || got[1] != "/bafy-test-cid" {
				t.Fatalf("unexpected files/cp args: %v", got)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.CreateFile(context.Background(), map[string]string{"event": "created"})
	if err != nil {
		t.Fatalf("CreateFile returned error: %v", err)
	}

	if !addCalled {
		t.Fatal("expected Kubo add to be called")
	}
	if !mfsCopyCalled {
		t.Fatal("expected Kubo files/cp to be called")
	}
	if result.Identifier.Format != "CID" {
		t.Fatalf("unexpected identifier format %q", result.Identifier.Format)
	}
	if result.Identifier.Value != "bafy-test-cid" {
		t.Fatalf("unexpected CID %q", result.Identifier.Value)
	}

	var payload map[string]string
	if err := json.Unmarshal(result.Data, &payload); err != nil {
		t.Fatalf("unmarshal result data: %v", err)
	}
	if payload["event"] != "created" {
		t.Fatalf("unexpected result payload: %v", payload)
	}
}

func TestFetchFileReadsFromKuboByCID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/cat" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST for Kubo cat, got %s", r.Method)
		}
		if got := r.URL.Query().Get("arg"); got != "bafy-test-cid" {
			t.Fatalf("unexpected cat arg %q", got)
		}
		_, _ = w.Write([]byte(`{"event":"created"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.FetchFile("bafy-test-cid")
	if err != nil {
		t.Fatalf("FetchFile returned error: %v", err)
	}

	if result.Identifier.Format != "CID" {
		t.Fatalf("unexpected identifier format %q", result.Identifier.Format)
	}
	if result.Identifier.Value != "bafy-test-cid" {
		t.Fatalf("unexpected CID %q", result.Identifier.Value)
	}
	if string(result.Data) != `{"event":"created"}` {
		t.Fatalf("unexpected result data %s", result.Data)
	}
}

func TestFetchKuboFile_DecodesBase64WrapPayload(t *testing.T) {
	payload := []byte("%PDF-1.3\nhello pdf content")
	encoded := base64.StdEncoding.EncodeToString(payload)
	stored := fmt.Sprintf("%q", encoded) // produces "JVBERi0x..."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v0/cat" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(stored))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.FetchFile("bafy-binary-cid")
	if err != nil {
		t.Fatalf("FetchFile returned error: %v", err)
	}
	if string(result.Data) != string(payload) {
		t.Fatalf("expected decoded binary payload, got %q", result.Data[:min(20, len(result.Data))])
	}
}

// TestCreateFile_CopyToMFSIsIdempotentWhenEntryExists reproduces the shared-IPFS
// federation collision: instance A already copied a PDF's CID into MFS, then
// instance B ships the identical bytes and stores them. The store is
// content-addressed, so B computes the same CID and its files/cp onto the
// existing /<cid> path fails ("already has entry by that name"). CreateFile must
// treat that as success — the byte-identical entry already satisfies the
// postcondition — rather than surfacing an error that would roll back B's
// contract receive.
func TestCreateFile_CopyToMFSIsIdempotentWhenEntryExists(t *testing.T) {
	const cid = "shared-cid"
	var statCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v0/add":
			_, _ = fmt.Fprintf(w, `{"Hash":%q}`, cid)
		case "/api/v0/files/cp":
			// The peer already copied this CID: Kubo rejects the duplicate path.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"Message":"cp: cannot put node in path /`+cid+`: directory already has entry by that name","Code":0,"Type":"error"}`)
		case "/api/v0/files/stat":
			statCalled = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"Hash":%q,"Type":"file"}`, cid)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.CreateFile(context.Background(), []byte("%PDF-1.7\npeer shipped bytes"))
	if err != nil {
		t.Fatalf("CreateFile must tolerate an already-present shared-MFS CID, got error: %v", err)
	}
	if result.Identifier.Value != cid {
		t.Fatalf("expected CID %q, got %q", cid, result.Identifier.Value)
	}
	if !statCalled {
		t.Fatal("expected files/stat to confirm the existing entry resolves to the same CID")
	}
}

// TestCreateFile_CopyToMFSFailsWhenEntryDiffers ensures the idempotency shortcut
// does not mask a genuine files/cp failure: if the MFS path does not resolve to
// the expected CID, CreateFile still returns an error.
func TestCreateFile_CopyToMFSFailsWhenEntryDiffers(t *testing.T) {
	const cid = "wanted-cid"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v0/add":
			_, _ = fmt.Fprintf(w, `{"Hash":%q}`, cid)
		case "/api/v0/files/cp":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprint(w, `{"Message":"cp: some other failure"}`)
		case "/api/v0/files/stat":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"Hash":"a-different-cid","Type":"file"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if _, err := client.CreateFile(context.Background(), []byte("payload")); err == nil {
		t.Fatal("expected CreateFile to surface a files/cp failure when MFS does not hold the expected CID")
	}
}
