package ipfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type APIClient struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
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
	Data any `json:"data"`
}

type MFSConfig struct {
	Name       string
	MFSBaseURL string
}

func (ipfs *APIClient) CreateFile(ctx context.Context, data any, mfsConfig *MFSConfig) (*IPFSResult, error) {

	url := ipfs.baseURL + "/api/ipfs/create"

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal data: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := ipfs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, body)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var result IPFSResult
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if mfsConfig != nil {
		name := mfsConfig.Name
		if name == "" {
			name = result.Identifier.Value
		}
		err := ipfs.copyToMFS(ctx, mfsConfig.MFSBaseURL, result.Identifier.Value, name)
		if err != nil {
			return &result, err
		}
	}

	return &result, nil
}

func (ipfs *APIClient) FetchFile(cid string) ([]byte, error) {

	url := fmt.Sprintf("%s/api/ipfs/%s", ipfs.baseURL, cid)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (ipfs *APIClient) DeleteFile(cid string) error {

	url := fmt.Sprintf("%s/api/ipfs/%s", ipfs.baseURL, cid)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, url, nil)
	resp, err := ipfs.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil

}

func (ipfs *APIClient) copyToMFS(ctx context.Context, baseURL string, cid string, filename string) error {

	url := fmt.Sprintf("%s/api/v0/files/cp?arg=/ipfs/%s&arg=/%s", baseURL, cid, filename)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	resp, err := ipfs.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
