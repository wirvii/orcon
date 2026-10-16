// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package anthropic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Wire types of the Messages request. They exist only in this package.

type msgRequest struct {
	Model         string       `json:"model"`
	MaxTokens     int          `json:"max_tokens"`
	System        string       `json:"system,omitempty"`
	Messages      []msgMessage `json:"messages"`
	Tools         []msgTool    `json:"tools,omitempty"`
	ToolChoice    any          `json:"tool_choice,omitempty"`
	Temperature   *float64     `json:"temperature,omitempty"`
	TopP          *float64     `json:"top_p,omitempty"`
	StopSequences []string     `json:"stop_sequences,omitempty"`
}

type msgMessage struct {
	Role    string     `json:"role"`
	Content []msgBlock `json:"content"`
}

type msgBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type msgTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// encodeRequest translates a canonical request into a Messages body. It
// returns the tool-name mapping used, so the response reads back with the
// same names.
//
// Two provider rules are handled here: system text goes in the top-level
// system field, and consecutive turns of the same role are merged, because
// tool results must share one user turn.
func encodeRequest(req provider.Request) ([]byte, provider.ToolNames, error) {
	names, err := provider.NewToolNames(req.Tools)
	if err != nil {
		return nil, provider.ToolNames{}, err
	}
	system, msgs, err := encodeMessages(req.Messages, names)
	if err != nil {
		return nil, provider.ToolNames{}, err
	}
	out := msgRequest{
		Model:         req.Model,
		MaxTokens:     req.MaxOutputTokens,
		System:        system,
		Messages:      msgs,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.Stop,
	}
	if out.MaxTokens == 0 {
		out.MaxTokens = DefaultMaxTokens
	}
	if len(req.Tools) > 0 {
		out.Tools, err = encodeTools(req.Tools, names)
		if err != nil {
			return nil, provider.ToolNames{}, err
		}
		out.ToolChoice, err = encodeToolChoice(req.ToolChoice, names)
		if err != nil {
			return nil, provider.ToolNames{}, err
		}
	}
	if err := checkResponseFormat(req.ResponseFormat); err != nil {
		return nil, provider.ToolNames{}, err
	}
	body, err := json.Marshal(out)
	if err != nil {
		return nil, provider.ToolNames{}, fmt.Errorf("encode messages request: %w", err)
	}
	return body, names, nil
}

func encodeMessages(msgs []provider.Message, names provider.ToolNames) (string, []msgMessage, error) {
	var systemParts []string
	out := make([]msgMessage, 0, len(msgs))
	for _, m := range msgs {
		var role string
		var blocks []msgBlock
		switch m.Role {
		case provider.RoleSystem:
			if text := joinText(m.Parts); text != "" {
				systemParts = append(systemParts, text)
			}
			continue
		case provider.RoleUser:
			role = "user"
			blocks = textBlocks(m.Parts)
		case provider.RoleAssistant:
			role = "assistant"
			blocks = textBlocks(m.Parts)
			for i := range m.Parts {
				if m.Parts[i].Kind != provider.PartToolCall {
					continue
				}
				block, err := encodeToolUse(m.Parts[i].ToolCall, names)
				if err != nil {
					return "", nil, err
				}
				blocks = append(blocks, block)
			}
		case provider.RoleTool:
			role = "user"
			for i := range m.Parts {
				res := &m.Parts[i].ToolResult
				if m.Parts[i].Kind != provider.PartToolResult {
					continue
				}
				blocks = append(blocks, msgBlock{
					Type:      "tool_result",
					ToolUseID: res.CallID,
					Content:   res.Output,
					IsError:   res.IsError,
				})
			}
		default:
			return "", nil, fmt.Errorf("unknown message role %q", m.Role)
		}
		if len(blocks) == 0 {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Role == role {
			out[n-1].Content = append(out[n-1].Content, blocks...)
			continue
		}
		out = append(out, msgMessage{Role: role, Content: blocks})
	}
	return strings.Join(systemParts, "\n\n"), out, nil
}

func textBlocks(parts []provider.Part) []msgBlock {
	var blocks []msgBlock
	for i := range parts {
		// The Messages API rejects empty text blocks.
		if parts[i].Kind == provider.PartText && parts[i].Text != "" {
			blocks = append(blocks, msgBlock{Type: "text", Text: parts[i].Text})
		}
	}
	return blocks
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

func encodeToolUse(c provider.ToolCall, names provider.ToolNames) (msgBlock, error) {
	wire, ok := names.Wire(c.Name)
	if !ok {
		return msgBlock{}, fmt.Errorf("transcript calls tool %q that was not declared", c.Name)
	}
	// tool_use input must be a JSON object. An invalid argument text from the
	// model is replaced by an empty object here; the engine has already
	// returned the parse error to the model as an error tool result.
	input := []byte("{}")
	if c.Arguments.Kind() == jsonvalue.KindObject {
		b, err := jsonvalue.Marshal(c.Arguments)
		if err != nil {
			return msgBlock{}, fmt.Errorf("encode arguments of tool %q: %w", c.Name, err)
		}
		input = b
	}
	return msgBlock{Type: "tool_use", ID: c.ID, Name: wire, Input: input}, nil
}

func encodeTools(tools []provider.Tool, names provider.ToolNames) ([]msgTool, error) {
	out := make([]msgTool, 0, len(tools))
	for _, t := range tools {
		wire, _ := names.Wire(t.Name)
		schema := []byte(`{"type":"object"}`)
		if !t.Schema.IsZero() {
			b, err := jsonvalue.Marshal(t.Schema)
			if err != nil {
				return nil, fmt.Errorf("encode schema of tool %q: %w", t.Name, err)
			}
			schema = b
		}
		out = append(out, msgTool{Name: wire, Description: t.Description, InputSchema: schema})
	}
	return out, nil
}

func encodeToolChoice(c provider.ToolChoice, names provider.ToolNames) (any, error) {
	switch c.Mode {
	case "", provider.ToolChoiceAuto:
		return map[string]string{"type": "auto"}, nil
	case provider.ToolChoiceNone:
		return map[string]string{"type": "none"}, nil
	case provider.ToolChoiceRequired:
		return map[string]string{"type": "any"}, nil
	case provider.ToolChoiceNamed:
		wire, ok := names.Wire(c.Name)
		if !ok {
			return nil, fmt.Errorf("tool choice names tool %q that was not declared", c.Name)
		}
		return map[string]string{"type": "tool", "name": wire}, nil
	default:
		return nil, fmt.Errorf("unknown tool choice mode %q", c.Mode)
	}
}

// checkResponseFormat rejects the JSON response formats until the
// structured-output cut lands. Failing here, instead of silently returning
// free text, keeps a node from believing it received a schema-checked answer.
func checkResponseFormat(f provider.ResponseFormat) error {
	switch f.Kind {
	case "", provider.FormatText:
		return nil
	case provider.FormatJSON, provider.FormatJSONSchema:
		return fmt.Errorf("response format %s: %w", f.Kind, provider.ErrUnsupported)
	default:
		return fmt.Errorf("unknown response format %q", f.Kind)
	}
}
