// Package client is the CLI's transport to the hush-hush server - a thin
// adapter over the generated hush-hush-go SDK, kept so internal/cli depends
// on this package's own sentinel errors and ObjectMetadata shape rather than
// the SDK's directly (design.md).
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	hushhush "github.com/alrayyes/hush-hush-go"
)

// Sentinel errors mapped from the server's documented status codes -
// callers match on these rather than inspecting a status code themselves.
var (
	ErrUnauthorized  = errors.New("missing or invalid bearer token")
	ErrNotFound      = errors.New("object not found")
	ErrAlreadyExists = errors.New("object already exists")
)

// ErrUnexpectedStatus anchors an unmapped status's error to something
// errors.Is can match, since the status code and server message vary per
// call and can't be a fixed sentinel on their own.
var ErrUnexpectedStatus = errors.New("unexpected status")

// ObjectMetadata is what a successful create or update returns. Matches
// components.schemas.ObjectMetadata in api/openapi.yaml.
type ObjectMetadata struct {
	ID     string
	UsedBy []string
}

// Client is a hush-hush API client. Token is the write-path bearer token;
// it's sent on every request, and reads simply ignore it server-side.
type Client struct {
	Caller string

	sdk *hushhush.Client
}

// New returns a Client backed by the hush-hush-go SDK.
func New(baseURL, token string) (*Client, error) {
	sdk, err := hushhush.NewClient(baseURL, hushhush.WithAPIKey(token))
	if err != nil {
		return nil, fmt.Errorf("build hush-hush-go client: %w", err)
	}

	return &Client{sdk: sdk}, nil
}

// Create stores value under id, sealed to usedBy's recipients before this
// is ever called - the client itself does no sealing.
func (c *Client) Create(ctx context.Context, id string, value []byte, usedBy []string) (ObjectMetadata, error) {
	req := hushhush.CreateObjectRequest{Id: id, Value: value}
	if len(usedBy) > 0 {
		req.UsedBy = &usedBy
	}

	meta, err := c.sdk.CreateObject(ctx, req, c.Caller)
	if err != nil {
		return ObjectMetadata{}, mapError(err)
	}

	return toObjectMetadata(meta), nil
}

// Get fetches an object's stored ciphertext exactly as sealed.
func (c *Client) Get(ctx context.Context, id string) ([]byte, error) {
	value, err := c.sdk.GetObject(ctx, id, c.Caller)
	if err != nil {
		return nil, mapError(err)
	}

	return value, nil
}

// Update replaces id's stored value, leaving its used_by metadata
// unchanged.
func (c *Client) Update(ctx context.Context, id string, value []byte) (ObjectMetadata, error) {
	meta, err := c.sdk.UpdateObject(ctx, id, hushhush.UpdateObjectRequest{Value: value}, c.Caller)
	if err != nil {
		return ObjectMetadata{}, mapError(err)
	}

	return toObjectMetadata(meta), nil
}

// Delete permanently removes id.
func (c *Client) Delete(ctx context.Context, id string) error {
	if err := c.sdk.DeleteObject(ctx, id, c.Caller); err != nil {
		return mapError(err)
	}

	return nil
}

func toObjectMetadata(m *hushhush.ObjectMetadata) ObjectMetadata {
	meta := ObjectMetadata{ID: m.Id}
	if m.UsedBy != nil {
		meta.UsedBy = *m.UsedBy
	}

	return meta
}

// mapError translates the SDK's *hushhush.APIError into this package's
// sentinels, so callers keep matching on client.Err* regardless of which
// transport sits underneath.
func mapError(err error) error {
	var apiErr *hushhush.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("call hush-hush server: %w", err)
	}

	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrAlreadyExists
	default:
		if apiErr.Message != "" {
			return fmt.Errorf("%w %d: %s", ErrUnexpectedStatus, apiErr.StatusCode, apiErr.Message)
		}

		return fmt.Errorf("%w %d", ErrUnexpectedStatus, apiErr.StatusCode)
	}
}
