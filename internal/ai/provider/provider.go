// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Role is the speaker of a canonical message. Stored and exposed values are
// UPPER_SNAKE (ADR-0015 §2).
type Role string

// The roles of a canonical message.
const (
	RoleSystem    Role = "SYSTEM"
	RoleUser      Role = "USER"
	RoleAssistant Role = "ASSISTANT"
	RoleTool      Role = "TOOL"
)

// PartKind names the content of one [Part].
type PartKind string

// The part kinds. Image, file and reasoning parts arrive with the cuts that
// use them; this cut carries text and tool traffic only.
const (
	PartText       PartKind = "TEXT"
	PartToolCall   PartKind = "TOOL_CALL"
	PartToolResult PartKind = "TOOL_RESULT"
)

// Part is one piece of a [Message]. Exactly one of Text, ToolCall and
// ToolResult is meaningful, selected by Kind.
type Part struct {
	Kind       PartKind
	Text       string
	ToolCall   ToolCall
	ToolResult ToolResult
}

// Message is one turn of the canonical transcript.
type Message struct {
	Role  Role
	Parts []Part
}

// ToolCall is a model's request to run a tool. ID is the provider's call
// identifier and must be echoed in the matching [ToolResult].
type ToolCall struct {
	ID   string
	Name string
	// Arguments holds the parsed arguments when the provider sent valid JSON.
	Arguments jsonvalue.Value
	// RawArguments is the argument text exactly as the provider sent it. The
	// engine keeps it for the step record when parsing failed.
	RawArguments string
	// ParseError describes why RawArguments is not valid JSON. Empty when
	// Arguments is set.
	ParseError string
}

// ToolResult answers a [ToolCall]. IsError tells the model the tool failed,
// for example when the arguments did not validate (ADR-0061 §3).
type ToolResult struct {
	CallID  string
	Name    string
	Output  string
	IsError bool
}

// Tool declares a tool the model may call. Schema is a JSON Schema object
// limited to the subset in ValidateArguments.
type Tool struct {
	Name        string
	Description string
	Schema      jsonvalue.Value
}

// ToolChoiceMode says how the model may use the declared tools.
type ToolChoiceMode string

// The tool-choice modes (ADR-0061 §2).
const (
	ToolChoiceAuto     ToolChoiceMode = "AUTO"
	ToolChoiceNone     ToolChoiceMode = "NONE"
	ToolChoiceRequired ToolChoiceMode = "REQUIRED"
	ToolChoiceNamed    ToolChoiceMode = "NAMED"
)

// ToolChoice selects the tool-use mode. Name is set only for
// [ToolChoiceNamed].
type ToolChoice struct {
	Mode ToolChoiceMode
	Name string
}

// ResponseFormatKind says what shape the final answer must take.
type ResponseFormatKind string

// The response formats (ADR-0061 §2).
const (
	FormatText       ResponseFormatKind = "TEXT"
	FormatJSON       ResponseFormatKind = "JSON"
	FormatJSONSchema ResponseFormatKind = "JSON_SCHEMA"
)

// ResponseFormat requests a shape for the answer. Schema is set only for
// [FormatJSONSchema].
type ResponseFormat struct {
	Kind   ResponseFormatKind
	Schema jsonvalue.Value
	Strict bool
}

// Request is one call to a model. Zero numeric fields mean "provider default".
type Request struct {
	Model           string
	Messages        []Message
	Tools           []Tool
	ToolChoice      ToolChoice
	ResponseFormat  ResponseFormat
	MaxOutputTokens int
	Temperature     *float64
	TopP            *float64
	Stop            []string
}

// FinishReason is the normalized reason a model stopped. Every adapter maps
// its provider's values onto these (ADR-0061 §3).
type FinishReason string

// The finish reasons.
const (
	FinishStop          FinishReason = "STOP"
	FinishLength        FinishReason = "LENGTH"
	FinishToolCalls     FinishReason = "TOOL_CALLS"
	FinishContentFilter FinishReason = "CONTENT_FILTER"
	FinishRefusal       FinishReason = "REFUSAL"
	FinishError         FinishReason = "ERROR"
)

// Usage counts the tokens of one call. Input excludes tokens read from or
// written to the prompt cache, which have their own fields.
type Usage struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
	Reasoning  int64
}

// Response is the normalized answer of one call.
type Response struct {
	// Text is the concatenated text of the answer.
	Text         string
	ToolCalls    []ToolCall
	FinishReason FinishReason
	Usage        Usage
	// ProviderRequestID is the identifier the provider assigned, for support
	// requests and the step record. Empty when the provider sent none.
	ProviderRequestID string
	// ModelVersion is the concrete model the provider served, which can differ
	// from the requested alias.
	ModelVersion string
}

// Capabilities declares what one model supports. Publication checks a node's
// needs against them (R-PRV-05, E-PRV-3).
type Capabilities struct {
	Tools           bool
	ParallelTools   bool
	Vision          bool
	Files           bool
	JSONSchema      bool
	Reasoning       bool
	PromptCache     bool
	Streaming       bool
	ContextTokens   int
	MaxOutputTokens int
}

// ModelProvider is the port every model adapter implements. Stream and Embed
// join the port with the cuts that use them (AI6 and AI10).
type ModelProvider interface {
	// Capabilities reports what model supports. It does not perform I/O.
	Capabilities(model string) Capabilities
	// Generate performs one call and returns its normalized answer. Errors
	// are [*Error] values when the provider answered with a failure.
	Generate(ctx context.Context, req Request) (Response, error)
}
