// Package client is the HTTP client every hush-hush consumer speaks
// through - the CLI's own transport, and the consumer side of the
// CLI-server Pact contract (openspec/changes/secrets-object-store/design.md).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Sentinel errors mapped from the server's documented status codes -
// callers match on these rather than inspecting a status code themselves.
var (
	ErrUnauthorized  = errors.New("missing or invalid bearer token")
	ErrNotFound      = errors.New("object not found")
	ErrAlreadyExists = errors.New("object already exists")
)

// ObjectMetadata is what a successful create or update returns. Matches
// components.schemas.ObjectMetadata in api/openapi.yaml.
type ObjectMetadata struct {
	ID     string   `json:"id"`
	UsedBy []string `json:"used_by,omitempty"`
}

// Client is a hush-hush API client. Token is the write-path bearer token;
// it's sent on every request, and reads simply ignore it server-side.
type Client struct {
	BaseURL string
	Token   string
	Caller  string
	HTTP    *http.Client
}

// New returns a Client using http.DefaultClient.
func New(baseURL, token string) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTP: http.DefaultClient}
}

type createRequest struct {
	ID     string   `json:"id"`
	Value  []byte   `json:"value"`
	UsedBy []string `json:"used_by,omitempty"`
}

type updateRequest struct {
	Value []byte `json:"value"`
}

type errorBody struct {
	Error string `json:"error"`
}

// ErrUnexpectedStatus anchors statusError's dynamic message to something
// errors.Is can match, since the status code and server message vary per
// call and can't be a fixed sentinel on their own.
var ErrUnexpectedStatus = errors.New("unexpected status")

// Create stores value under id, sealed to usedBy's recipients before this
// is ever called - the client itself does no sealing.
func (c *Client) Create(ctx context.Context, id string, value []byte, usedBy []string) (ObjectMetadata, error) {
	body, err := json.Marshal(createRequest{ID: id, Value: value, UsedBy: usedBy})
	if err != nil {
		return ObjectMetadata{}, fmt.Errorf("marshal create request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/objects", bytes.NewReader(body))
	if err != nil {
		return ObjectMetadata{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	var meta ObjectMetadata
	if err := c.doJSON(req, http.StatusCreated, &meta, map[int]error{
		http.StatusUnauthorized: ErrUnauthorized,
		http.StatusConflict:     ErrAlreadyExists,
	}); err != nil {
		return ObjectMetadata{}, err
	}

	return meta, nil
}

// Get fetches an object's stored ciphertext exactly as sealed.
func (c *Client) Get(ctx context.Context, id string) ([]byte, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/objects/"+id, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(resp)
	}

	value, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read object body: %w", err)
	}

	return value, nil
}

// Update replaces id's stored value, leaving its used_by metadata
// unchanged.
func (c *Client) Update(ctx context.Context, id string, value []byte) (ObjectMetadata, error) {
	body, err := json.Marshal(updateRequest{Value: value})
	if err != nil {
		return ObjectMetadata{}, fmt.Errorf("marshal update request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPut, "/objects/"+id, bytes.NewReader(body))
	if err != nil {
		return ObjectMetadata{}, err
	}

	req.Header.Set("Content-Type", "application/json")

	var meta ObjectMetadata
	if err := c.doJSON(req, http.StatusOK, &meta, map[int]error{
		http.StatusUnauthorized: ErrUnauthorized,
		http.StatusNotFound:     ErrNotFound,
	}); err != nil {
		return ObjectMetadata{}, err
	}

	return meta, nil
}

// Delete permanently removes id.
func (c *Client) Delete(ctx context.Context, id string) error {
	req, err := c.newRequest(ctx, http.MethodDelete, "/objects/"+id, nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return statusError(resp)
	}
}

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	if c.Caller != "" {
		req.Header.Set("X-Caller", c.Caller)
	}

	return req, nil
}

// doJSON sends req, decodes a status-matching JSON response into out, and
// maps any status in knownErrors to its sentinel.
func (c *Client) doJSON(req *http.Request, wantStatus int, out any, knownErrors map[int]error) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if sentinel, ok := knownErrors[resp.StatusCode]; ok {
		return sentinel
	}

	if resp.StatusCode != wantStatus {
		return statusError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

// statusError builds an error from an unexpected response, including the
// server's own message when it sent one.
func statusError(resp *http.Response) error {
	var body errorBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && body.Error != "" {
		return fmt.Errorf("%w %d: %s", ErrUnexpectedStatus, resp.StatusCode, body.Error)
	}

	return fmt.Errorf("%w %d", ErrUnexpectedStatus, resp.StatusCode)
}
