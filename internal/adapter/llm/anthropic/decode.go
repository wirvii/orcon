// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package anthropic

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

type msgResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	StopReason string     `json:"stop_reason"`
	Usage      *wireUsage `json:"usage"`
}

type wireUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// decodeResponse normalizes a successful Messages body. Blocks of kinds this
// cut does not carry (for example thinking blocks) are skipped, not failed.
func decodeResponse(raw []byte, names provider.ToolNames) (provider.Response, error) {
	var wire msgResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return provider.Response{}, &provider.Error{Class: provider.ClassPermanent, Code: "decode", Message: "response is not a message"}
	}
	out := provider.Response{
		FinishReason:      finishReason(wire.StopReason),
		Usage:             usage(wire.Usage),
		ProviderRequestID: wire.ID,
		ModelVersion:      wire.Model,
	}
	var text strings.Builder
	for _, block := range wire.Content {
		switch block.Type {
		case "text":
			text.WriteString(block.Text)
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, toolCall(block.ID, block.Name, block.Input, names))
		}
	}
	out.Text = text.String()
	return out, nil
}

// finishReason maps stop_reason. "pause_turn" asks the caller to continue a
// long server-side turn; this cut does not continue turns, so it is an error
// rather than a silent stop.
func finishReason(value string) provider.FinishReason {
	switch value {
	case "end_turn", "stop_sequence":
		return provider.FinishStop
	case "max_tokens", "model_context_window_exceeded":
		return provider.FinishLength
	case "tool_use":
		return provider.FinishToolCalls
	case "refusal":
		return provider.FinishRefusal
	default:
		return provider.FinishError
	}
}

// usage maps the token counts. Anthropic reports input_tokens without the
// cached tokens, so the canonical fields map one to one.
func usage(u *wireUsage) provider.Usage {
	if u == nil {
		return provider.Usage{}
	}
	return provider.Usage{
		Input:      u.InputTokens,
		Output:     u.OutputTokens,
		CacheRead:  u.CacheReadInputTokens,
		CacheWrite: u.CacheCreationInputTokens,
	}
}

func toolCall(id, name string, input json.RawMessage, names provider.ToolNames) provider.ToolCall {
	if canon, ok := names.Canonical(name); ok {
		name = canon
	}
	raw := string(input)
	out := provider.ToolCall{ID: id, Name: name, RawArguments: raw}
	if len(input) == 0 {
		empty, _ := jsonvalue.Object()
		out.Arguments = empty
		return out
	}
	args, err := jsonvalue.Parse(input)
	if err != nil {
		out.ParseError = err.Error()
		return out
	}
	out.Arguments = args
	return out
}

type errorBody struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// decodeError classifies a non-2xx answer. Only the provider's own text is
// kept, truncated; the request content and the credential are never included.
func decodeError(status int, header http.Header, raw []byte) error {
	pe := &provider.Error{
		Class:      provider.Classify(status),
		Status:     status,
		RetryAfter: retryAfter(header.Get("Retry-After"), time.Now()),
	}
	var body errorBody
	if err := json.Unmarshal(raw, &body); err == nil {
		pe.Code = body.Error.Type
		pe.Message = truncate(body.Error.Message)
	}
	if pe.Message == "" {
		pe.Message = http.StatusText(status)
	}
	return pe
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
