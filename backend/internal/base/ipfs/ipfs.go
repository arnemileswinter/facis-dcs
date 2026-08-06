// Package ipfs is the client for the IPFS anchor store used by the
// tamper-evident audit trail (base/event.OutboxProcessor writes each signed,
// hash-chained audit entry here) and by C2PA/provenance artifacts
// (pdfgeneration, signingmanagement).
package ipfs

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"
)

type APIClient struct {
	// mfsBaseURL is the Kubo RPC API. Artifacts are stored and read through it
	// directly: the XFSC ipfs-document-manager that used to sit in front of it
	// answered every read by listing the whole pinset under Kubo's pinner lock,
	// and gave an add and its pin one shared 5s deadline (ADR-36).
	mfsBaseURL string
	client     *http.Client
}

func NewClient(mfsBaseURL string) *APIClient {
	return &APIClient{
		mfsBaseURL: mfsBaseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type IPFSResult struct {
	Identifier struct {
		Format string `json:"Format"`
		Value  string `json:"Value"`
	} `json:"identifier"`
	Data json.RawMessage `json:"data"`
}

func (c *APIClient) CreateFile(ctx context.Context, data any) (*IPFSResult, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}
	return c.createKuboFile(ctx, jsonData)
}

func (c *APIClient) FetchFile(cid string) (*IPFSResult, error) {
	return c.fetchKuboFile(cid)
}

func (c *APIClient) DeleteFile(cid string) error {
	return c.deleteKuboFile(cid)
}

func (c *APIClient) createKuboFile(ctx context.Context, data []byte) (*IPFSResult, error) {
	if c.mfsBaseURL == "" {
		return nil, fmt.Errorf("IPFS_MFS_BASE_URL is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="audit-log.json"`)
	header.Set("Content-Type", "application/json")

	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, fmt.Errorf("create multipart part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write multipart data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	url := c.mfsBaseURL + "/api/v0/add?pin=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("create Kubo add request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do Kubo add request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("could not close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected Kubo add status %d: %s", resp.StatusCode, body)
	}

	var addResult struct {
		Hash string `json:"Hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&addResult); err != nil {
		return nil, fmt.Errorf("decode Kubo add response: %w", err)
	}
	if addResult.Hash == "" {
		return nil, fmt.Errorf("the Kubo add response does not include a CID")
	}

	result := &IPFSResult{
		Data: data,
	}
	result.Identifier.Format = "CID"
	result.Identifier.Value = addResult.Hash

	if err := c.copyToMFS(ctx, c.mfsBaseURL, addResult.Hash, addResult.Hash); err != nil {
		return result, err
	}

	return result, nil
}

func (c *APIClient) fetchKuboFile(cid string) (*IPFSResult, error) {
	if c.mfsBaseURL == "" {
		return nil, fmt.Errorf("IPFS_MFS_BASE_URL is required")
	}

	url := fmt.Sprintf("%s/api/v0/cat?arg=%s", c.mfsBaseURL, cid)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create Kubo cat request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do Kubo cat request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("could not close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected Kubo cat status %d: %s", resp.StatusCode, body)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read Kubo cat response: %w", err)
	}

	var dataStr string
	var resultData []byte
	if json.Unmarshal(body, &dataStr) == nil {
		decoded, err := base64.StdEncoding.DecodeString(dataStr)
		if err != nil {
			return nil, fmt.Errorf("base64 decode Kubo file data: %w", err)
		}
		resultData = decoded
	} else {
		resultData = body
	}

	result := &IPFSResult{
		Data: resultData,
	}
	result.Identifier.Format = "CID"
	result.Identifier.Value = cid

	return result, nil
}

func (c *APIClient) deleteKuboFile(cid string) error {
	if c.mfsBaseURL == "" {
		return fmt.Errorf("IPFS_MFS_BASE_URL is required")
	}

	url := fmt.Sprintf("%s/api/v0/pin/rm?arg=%s", c.mfsBaseURL, cid)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("create Kubo unpin request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("do Kubo unpin request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("could not close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected Kubo unpin status %d: %s", resp.StatusCode, body)
	}

	return nil
}

func (c *APIClient) copyToMFS(ctx context.Context, baseURL string, cid string, filename string) error {

	url := fmt.Sprintf("%s/api/v0/files/cp?arg=/ipfs/%s&arg=/%s", baseURL, cid, filename)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Println("could not close response body")
		}
	}(resp.Body)

	if resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	// files/cp fails when /<filename> already exists in MFS. In the shared-IPFS
	// federation a peer instance may have already copied this exact CID: the
	// store is content-addressed, so an existing entry at the same path holds
	// identical bytes and the desired postcondition already holds. Confirm the
	// entry resolves to the same CID and treat that as success rather than
	// rolling back the caller's work over a benign collision.
	if c.mfsEntryHasCID(ctx, baseURL, filename, cid) {
		return nil
	}
	return fmt.Errorf("unexpected Kubo files/cp status %d: %s", resp.StatusCode, body)
}

// mfsEntryHasCID reports whether the MFS path /<filename> already resolves to
// the given CID (via files/stat). Used to make copyToMFS idempotent: a
// content-addressed entry that is already present holds the same bytes.
func (c *APIClient) mfsEntryHasCID(ctx context.Context, baseURL string, filename string, cid string) bool {
	url := fmt.Sprintf("%s/api/v0/files/stat?arg=/%s", baseURL, filename)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			log.Println("could not close response body")
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return false
	}
	var stat struct {
		Hash string `json:"Hash"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stat); err != nil {
		return false
	}
	return stat.Hash == cid
}
