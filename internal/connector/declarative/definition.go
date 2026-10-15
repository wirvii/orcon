// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package declarative

import "github.com/wirvii/orcon/internal/platform/jsonvalue"

// Format is the value of the "format" member of a declarative connector.
const Format = "orcon/declarative@1"

// Definition is one connector body (ADR-0030 §1). The json tags are read by the
// codec adapter; the package itself never touches an encoding.
type Definition struct {
	Format     string               `json:"format"`
	HTTP       HTTP                 `json:"http"`
	Operations map[string]Operation `json:"operations"`
}

// HTTP holds the defaults of every request (ADR-0030 §2) and the
// authentication scheme the host applies (ADR-0030 §4).
type HTTP struct {
	// BaseURL is an absolute https URL (or http when granted). Its host must be
	// granted by the network policy of the connector.
	BaseURL string            `json:"baseUrl"`
	Headers map[string]string `json:"headers,omitempty"`
	// Timeout is a Go duration string such as "30s". Empty means the default.
	Timeout string `json:"timeout,omitempty"`
	Auth    Auth   `json:"auth"`
}

// Authentication scheme types (ADR-0030 §4). OAUTH2 and SESSION_TOKEN are part
// of the format but the interpreter does not run them yet.
const (
	AuthNone          = "NONE"
	AuthAPIKey        = "API_KEY"
	AuthBasic         = "BASIC"
	AuthBearer        = "BEARER"
	AuthOAuth2        = "OAUTH2"
	AuthCustomHeaders = "CUSTOM_HEADERS"
	AuthAWSSigV4      = "AWS_SIG_V4"
	AuthSessionToken  = "SESSION_TOKEN"
)

// Auth describes how the host authenticates a request. Credential values are
// read from the credential fields named here and never from the connector.
type Auth struct {
	Type string `json:"type"`
	// In places an API key: HEADER, QUERY or COOKIE (API_KEY only).
	In string `json:"in,omitempty"`
	// Name is the header, query parameter or cookie name (API_KEY).
	Name string `json:"name,omitempty"`
	// Prefix is written before the key in a header, such as "Token " (API_KEY).
	Prefix string `json:"prefix,omitempty"`
	// Field is the credential field holding the key (API_KEY). Default "apiKey".
	Field string `json:"field,omitempty"`
	// Headers are header templates over $credentials (CUSTOM_HEADERS).
	Headers map[string]string `json:"headers,omitempty"`
	// Service and Region scope the signature (AWS_SIG_V4).
	Service string `json:"service,omitempty"`
	Region  string `json:"region,omitempty"`
}

// Operation is one "<resource>.<verb>" operation (ADR-0030 §5). Purity and
// Safety together are the ADR-0029 §5 contract that the validator checks.
type Operation struct {
	// Purity is READ or EFFECT.
	Purity string `json:"purity"`
	// Safety is READ_ONLY, IDEMPOTENCY_KEY, RECONCILE or UNSAFE (ADR-0079 §4).
	Safety string `json:"safety"`
	// IdempotencyHeader names the header that carries the key when Safety is
	// IDEMPOTENCY_KEY. IdempotencyKey is the template that yields the key.
	IdempotencyHeader string `json:"idempotencyHeader,omitempty"`
	IdempotencyKey    string `json:"idempotencyKey,omitempty"`
	// MaxAttempts bounds the attempts of one call; zero means the default of
	// [DefaultMaxAttempts]. Only READ_ONLY and IDEMPOTENCY_KEY ever retry.
	MaxAttempts int          `json:"maxAttempts,omitempty"`
	Request     Request      `json:"request"`
	Response    ResponseSpec `json:"response"`
	Pagination  *Pagination  `json:"pagination,omitempty"`
}

// Request is the HTTP request of an operation. Every string is a template (see
// [Render]).
type Request struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    *Body             `json:"body,omitempty"`
}

// Body is a JSON object or a form whose field values are templates. A JSON
// field may be a nested object or array of templates; a value that is exactly
// one placeholder keeps the parameter's JSON type. A FORM field must be a
// string template.
type Body struct {
	Type   string          `json:"type"`
	Fields map[string]Node `json:"fields"`
}

// Node is one value of a body template: any JSON value whose strings may hold
// placeholders. The value is read through jsonvalue, so this package never
// imports encoding/json.
type Node struct {
	Value jsonvalue.Value
}

// UnmarshalJSON keeps the JSON value exactly as the document writes it.
func (n *Node) UnmarshalJSON(data []byte) error {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return err
	}
	n.Value = v
	return nil
}

// ResponseSpec maps a response to items (ADR-0030 §5).
type ResponseSpec struct {
	// Select is a path such as "$.data.items" to the array or object to emit.
	// Empty means the whole body.
	Select string `json:"select,omitempty"`
	// Map adds members to every emitted item: each key names a JSON path read
	// from the item (such as "$.messages[0].id"), and the provider's record is
	// kept as it is. A key the record already has is replaced. A path that finds
	// nothing adds nothing.
	Map map[string]string `json:"map,omitempty"`
	// Empty is NO_ITEMS (default) or ONE_EMPTY_ITEM, for an empty result.
	Empty  string      `json:"empty,omitempty"`
	Errors []ErrorRule `json:"errors,omitempty"`
}

// ErrorRule maps a failed response to a class and a code (ADR-0030 §5). A rule
// matches when every condition it names holds: the HTTP status is one of Status
// (when Status is given) and the body member at Body.Path has one of Body.In
// (when Body is given). Rules are tried in order and the first match wins.
type ErrorRule struct {
	Status []int `json:"status,omitempty"`
	// Body matches a provider error code carried in the response body, such as
	// the "error.code" member of the Graph API error envelope.
	Body  *BodyMatch `json:"body,omitempty"`
	Class string     `json:"class"`
	Code  string     `json:"code,omitempty"`
}

// BodyMatch compares one member of a JSON response body with a list of
// scalars. Numbers and strings match by their text, so 4 and "4" are the same.
type BodyMatch struct {
	Path string `json:"path"`
	In   []Node `json:"in"`
}

// Pagination describes how read operations walk pages (ADR-0030 §6).
type Pagination struct {
	// Strategy is OFFSET, PAGE, CURSOR, LINK_HEADER or NEXT_URL.
	Strategy string `json:"strategy"`
	// Param is the query parameter that carries the offset, page or cursor.
	Param string `json:"param,omitempty"`
	// LimitParam and PageSize set the page size on OFFSET.
	LimitParam string `json:"limitParam,omitempty"`
	PageSize   int    `json:"pageSize,omitempty"`
	// Start is the first offset (OFFSET) or page number (PAGE). Default 0 and 1.
	Start int `json:"start,omitempty"`
	// From is PATH or HEADER for CURSOR, and PATH for NEXT_URL. Path is the
	// JSON path in the body; Header is the response header name.
	From   string `json:"from,omitempty"`
	Path   string `json:"path,omitempty"`
	Header string `json:"header,omitempty"`
	// MaxPages bounds the pages of one call. Zero means [DefaultMaxPages].
	MaxPages int `json:"maxPages,omitempty"`
	// OnMaxPages is TRUNCATE to stop quietly with a warning; empty makes
	// reaching the bound a PERMANENT failure.
	OnMaxPages string `json:"onMaxPages,omitempty"`
}

// Purity values (ADR-0029 §5).
const (
	PurityRead   = "READ"
	PurityEffect = "EFFECT"
)

// Safety levels (ADR-0079 §4).
const (
	SafetyReadOnly       = "READ_ONLY"
	SafetyIdempotencyKey = "IDEMPOTENCY_KEY"
	SafetyReconcile      = "RECONCILE"
	SafetyUnsafe         = "UNSAFE"
)

// Defaults of the interpreter.
const (
	DefaultMaxAttempts = 3
	DefaultMaxPages    = 1000
)
