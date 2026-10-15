// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// maxMessageChars bounds the provider's error text kept in an error value.
const maxMessageChars = 300

type chatResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   *string        `json:"content"`
			Refusal   *string        `json:"refusal"`
			ToolCalls []chatToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *wireUsage `json:"usage"`
}

type wireUsage struct {
	PromptTokens        int64 `json:"prompt_tokens"`
	CompletionTokens    int64 `json:"completion_tokens"`
	PromptTokensDetails *struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionTokensDetails *struct {
		ReasoningTokens int64 `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

// decodeResponse normalizes a successful Chat Completions body. The names
// argument maps the provider's tool names back to the declared ones; a name
// the request never declared is kept as the model sent it, so the engine can
// reject it.
func decodeResponse(raw []byte, names provider.ToolNames) (provider.Response, error) {
	var wire chatResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return provider.Response{}, &provider.Error{Class: provider.ClassPermanent, Code: "decode", Message: "response is not a chat completion"}
	}
	if len(wire.Choices) == 0 {
		return provider.Response{}, &provider.Error{Class: provider.ClassPermanent, Code: "no_choices", Message: "response has no choices"}
	}
	choice := wire.Choices[0]
	out := provider.Response{
		FinishReason:      finishReason(choice.FinishReason, choice.Message.Refusal),
		Usage:             usage(wire.Usage),
		ProviderRequestID: wire.ID,
		ModelVersion:      wire.Model,
	}
	if choice.Message.Content != nil {
		out.Text = *choice.Message.Content
	}
	for _, tc := range choice.Message.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, toolCall(tc, names))
	}
	return out, nil
}

// finishReason maps the provider's finish value. A refusal field wins over
// "stop", because the model stopped by declining rather than by answering.
func finishReason(value string, refusal *string) provider.FinishReason {
	if refusal != nil && *refusal != "" {
		return provider.FinishRefusal
	}
	switch value {
	case "stop":
		return provider.FinishStop
	case "length":
		return provider.FinishLength
	case "tool_calls", "function_call":
		return provider.FinishToolCalls
	case "content_filter":
		return provider.FinishContentFilter
	default:
		return provider.FinishError
	}
}

// usage maps the token counts. The provider counts cached prompt tokens inside
// prompt_tokens and reasoning tokens inside completion_tokens; the canonical
// fields are disjoint, so cached and reasoning tokens are subtracted out.
func usage(u *wireUsage) provider.Usage {
	if u == nil {
		return provider.Usage{}
	}
	var cached, reasoning int64
	if u.PromptTokensDetails != nil {
		cached = u.PromptTokensDetails.CachedTokens
	}
	if u.CompletionTokensDetails != nil {
		reasoning = u.CompletionTokensDetails.ReasoningTokens
	}
	return provider.Usage{
		Input:     u.PromptTokens - cached,
		Output:    u.CompletionTokens - reasoning,
		CacheRead: cached,
		Reasoning: reasoning,
	}
}

// toolCall normalizes one call. Arguments that are not valid JSON do not fail
// the response: the engine returns the parse error to the model (ADR-0061 §3).
func toolCall(tc chatToolCall, names provider.ToolNames) provider.ToolCall {
	name := tc.Function.Name
	if canon, ok := names.Canonical(name); ok {
		name = canon
	}
	out := provider.ToolCall{ID: tc.ID, Name: name, RawArguments: tc.Function.Arguments}
	if strings.TrimSpace(tc.Function.Arguments) == "" {
		empty, _ := jsonvalue.Object()
		out.Arguments = empty
		return out
	}
	args, err := jsonvalue.Parse([]byte(tc.Function.Arguments))
	if err != nil {
		out.ParseError = err.Error()
		return out
	}
	out.Arguments = args
	return out
}

type errorBody struct {
	Error struct {
		Message string          `json:"message"`
		Type    string          `json:"type"`
		Code    json.RawMessage `json:"code"`
	} `json:"error"`
}

// decodeError classifies a non-2xx answer. The message is the provider's own
// text, truncated; the request content and the credential are never included.
func decodeError(status int, header http.Header, raw []byte) error {
	pe := &provider.Error{
		Class:      provider.Classify(status),
		Status:     status,
		RetryAfter: retryAfter(header.Get("Retry-After"), time.Now()),
	}
	var body errorBody
	if err := json.Unmarshal(raw, &body); err == nil {
		pe.Code = errorCode(body.Error.Code, body.Error.Type)
		pe.Message = truncate(body.Error.Message)
	}
	if pe.Message == "" {
		pe.Message = http.StatusText(status)
	}
	return pe
}

func errorCode(code json.RawMessage, typ string) string {
	var s string
	if len(code) > 0 && json.Unmarshal(code, &s) == nil && s != "" {
		return s
	}
	return typ
}

// retryAfter reads a Retry-After value given as seconds or as an HTTP date.
func retryAfter(value string, now time.Time) time.Duration {
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

// truncate shortens s to at most maxMessageChars bytes without splitting a
// UTF-8 sequence.
func truncate(s string) string {
	if len(s) <= maxMessageChars {
		return s
	}
	cut := maxMessageChars
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
