// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package examplestest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// maxResponseBytes bounds how much of a response the runner reads.
const maxResponseBytes = 8 << 20

// Client talks to the API under test with the credential of the run. Scenarios
// receive one: it is their only way to seed state.
type Client struct {
	base   string
	prefix string
	token  string
	orgID  string
	http   *http.Client
}

// NewClient returns a client for the server at baseURL. Paths given to Do are
// relative to apiPrefix (for example "/api/v1"); token is sent as a bearer
// credential when not empty.
func NewClient(baseURL, apiPrefix, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{base: strings.TrimRight(baseURL, "/"), prefix: apiPrefix, token: token, http: httpClient}
}

// WithOrg returns a client that reports orgID as the organization of its
// credential. Scenarios read it with [Client.OrgID] to seed state in the right
// organization.
func (c *Client) WithOrg(orgID string) *Client {
	clone := *c
	clone.orgID = orgID
	return &clone
}

// OrgID returns the id of the organization the credential belongs to.
func (c *Client) OrgID() string { return c.orgID }

// Request is one call to the API.
type Request struct {
	// Method is the HTTP method.
	Method string
	// Path is relative to the API prefix, with path parameters already filled in.
	Path string
	// Query holds query parameters.
	Query url.Values
	// Header holds extra request headers.
	Header http.Header
	// Body is encoded as JSON when not nil.
	Body any
}

// Response is the answer to a Request with its body read in full.
type Response struct {
	// Status is the HTTP status code.
	Status int
	// Header holds the response headers.
	Header http.Header
	// Body is the raw response body.
	Body []byte
}

// Decode unmarshals the JSON body into v.
func (r *Response) Decode(v any) error {
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("decode response body (status %d): %w", r.Status, err)
	}
	return nil
}

// Do sends req to the API prefix of the server and reads the whole answer.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	return c.send(ctx, req.Method, c.prefix+req.Path, req.Query, req.Header, req.Body)
}

// send performs a call; its fullPath already includes the API prefix.
func (c *Client) send(ctx context.Context, method, fullPath string, query url.Values, header http.Header, body any) (*Response, error) {
	hr, err := c.newRequest(ctx, method, fullPath, query, header, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(hr)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, fullPath, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("%s %s: read body: %w", method, fullPath, err)
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: data}, nil
}

func (c *Client) newRequest(ctx context.Context, method, fullPath string, query url.Values, header http.Header, body any) (*http.Request, error) {
	target := c.base + fullPath
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	hr, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, fmt.Errorf("build request %s %s: %w", method, fullPath, err)
	}
	if body != nil {
		hr.Header.Set("Content-Type", "application/json")
	}
	hr.Header.Set("Accept", "application/json, application/problem+json")
	if c.token != "" {
		hr.Header.Set("Authorization", "Bearer "+c.token)
	}
	for name, values := range header {
		hr.Header.Del(name)
		for _, v := range values {
			hr.Header.Add(name, v)
		}
	}
	return hr, nil
}
