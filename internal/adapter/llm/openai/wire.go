// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Wire types of the Chat Completions request. They exist only in this
// package (ADR-0061 §2: provider wire values stay inside the adapter).

type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []chatMessage `json:"messages"`
	Tools          []chatTool    `json:"tools,omitempty"`
	ToolChoice     any           `json:"tool_choice,omitempty"`
	ResponseFormat any           `json:"response_format,omitempty"`
	MaxTokens      int           `json:"max_tokens,omitempty"`
	Temperature    *float64      `json:"temperature,omitempty"`
	TopP           *float64      `json:"top_p,omitempty"`
	Stop           []string      `json:"stop,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatFunctionCall `json:"function"`
}

type chatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatTool struct {
	Type     string          `json:"type"`
	Function chatFunctionDef `json:"function"`
}

type chatFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// encodeRequest translates a canonical request. It returns the tool-name
// mapping it used, so the response can be read back with the same names.
func encodeRequest(req provider.Request) ([]byte, provider.ToolNames, error) {
	names, err := provider.NewToolNames(req.Tools)
	if err != nil {
		return nil, provider.ToolNames{}, err
	}
	msgs, err := encodeMessages(req.Messages, names)
	if err != nil {
		return nil, provider.ToolNames{}, err
	}
	out := chatRequest{
		Model:       req.Model,
		Messages:    msgs,
		MaxTokens:   req.MaxOutputTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}
	if len(req.Tools) > 0 {
		tools, err := encodeTools(req.Tools, names)
		if err != nil {
			return nil, provider.ToolNames{}, err
		}
		out.Tools = tools
		out.ToolChoice, err = encodeToolChoice(req.ToolChoice, names)
		if err != nil {
			return nil, provider.ToolNames{}, err
		}
	}
	rf, err := encodeResponseFormat(req.ResponseFormat)
	if err != nil {
		return nil, provider.ToolNames{}, err
	}
	out.ResponseFormat = rf

	body, err := json.Marshal(out)
	if err != nil {
		return nil, provider.ToolNames{}, fmt.Errorf("encode chat request: %w", err)
	}
	return body, names, nil
}

// encodeMessages maps the canonical transcript to Chat Completions messages.
// Each tool result becomes its own "tool" message, because the wire format
// has no grouping; an assistant message carries its text and its tool calls
// together.
func encodeMessages(msgs []provider.Message, names provider.ToolNames) ([]chatMessage, error) {
	out := make([]chatMessage, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case provider.RoleSystem, provider.RoleUser:
			text := joinText(m.Parts)
			out = append(out, chatMessage{Role: roleName(m.Role), Content: &text})
		case provider.RoleAssistant:
			msg := chatMessage{Role: "assistant"}
			if text := joinText(m.Parts); text != "" {
				msg.Content = &text
			}
			for i := range m.Parts {
				if m.Parts[i].Kind != provider.PartToolCall {
					continue
				}
				call, err := encodeToolCall(m.Parts[i].ToolCall, names)
				if err != nil {
					return nil, err
				}
				msg.ToolCalls = append(msg.ToolCalls, call)
			}
			out = append(out, msg)
		case provider.RoleTool:
			for i := range m.Parts {
				if m.Parts[i].Kind != provider.PartToolResult {
					continue
				}
				res := &m.Parts[i].ToolResult
				content := res.Output
				if res.IsError {
					content = "ERROR: " + content
				}
				out = append(out, chatMessage{Role: "tool", ToolCallID: res.CallID, Content: &content})
			}
		default:
			return nil, fmt.Errorf("unknown message role %q", m.Role)
		}
	}
	return out, nil
}

func roleName(r provider.Role) string {
	switch r {
	case provider.RoleSystem:
		return "system"
	case provider.RoleUser:
		return "user"
	case provider.RoleAssistant:
		return "assistant"
	case provider.RoleTool:
		return "tool"
	default:
		return string(r)
	}
}

func joinText(parts []provider.Part) string {
	var b strings.Builder
	for i := range parts {
		if parts[i].Kind == provider.PartText {
			b.WriteString(parts[i].Text)
		}
	}
	return b.String()
}

func encodeToolCall(c provider.ToolCall, names provider.ToolNames) (chatToolCall, error) {
	wire, ok := names.Wire(c.Name)
	if !ok {
		return chatToolCall{}, fmt.Errorf("transcript calls tool %q that was not declared", c.Name)
	}
	args := c.RawArguments
	if c.Arguments.Kind() != jsonvalue.KindInvalid {
		b, err := jsonvalue.Marshal(c.Arguments)
		if err != nil {
			return chatToolCall{}, fmt.Errorf("encode arguments of tool %q: %w", c.Name, err)
		}
		args = string(b)
	}
	if args == "" {
		args = "{}"
	}
	return chatToolCall{ID: c.ID, Type: "function", Function: chatFunctionCall{Name: wire, Arguments: args}}, nil
}

func encodeTools(tools []provider.Tool, names provider.ToolNames) ([]chatTool, error) {
	out := make([]chatTool, 0, len(tools))
	for _, t := range tools {
		wire, _ := names.Wire(t.Name)
		def := chatFunctionDef{Name: wire, Description: t.Description}
		if !t.Schema.IsZero() {
			raw, err := jsonvalue.Marshal(t.Schema)
			if err != nil {
				return nil, fmt.Errorf("encode schema of tool %q: %w", t.Name, err)
			}
			def.Parameters = raw
		}
		out = append(out, chatTool{Type: "function", Function: def})
	}
	return out, nil
}

func encodeToolChoice(c provider.ToolChoice, names provider.ToolNames) (any, error) {
	switch c.Mode {
	case "", provider.ToolChoiceAuto:
		return "auto", nil
	case provider.ToolChoiceNone:
		return "none", nil
	case provider.ToolChoiceRequired:
		return "required", nil
	case provider.ToolChoiceNamed:
		wire, ok := names.Wire(c.Name)
		if !ok {
			return nil, fmt.Errorf("tool choice names tool %q that was not declared", c.Name)
		}
		return map[string]any{"type": "function", "function": map[string]string{"name": wire}}, nil
	default:
		return nil, fmt.Errorf("unknown tool choice mode %q", c.Mode)
	}
}

func encodeResponseFormat(f provider.ResponseFormat) (any, error) {
	switch f.Kind {
	case "", provider.FormatText:
		return nil, nil
	case provider.FormatJSON:
		return map[string]string{"type": "json_object"}, nil
	case provider.FormatJSONSchema:
		if f.Schema.IsZero() {
			return nil, errors.New("JSON_SCHEMA response format without a schema")
		}
		raw, err := jsonvalue.Marshal(f.Schema)
		if err != nil {
			return nil, fmt.Errorf("encode response schema: %w", err)
		}
		return map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "final_answer",
				"schema": json.RawMessage(raw),
				"strict": f.Strict,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unknown response format %q", f.Kind)
	}
}
