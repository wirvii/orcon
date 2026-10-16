// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"fmt"

	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// turn is one model answer as it is stored in a MODEL step: its text and the
// tool calls it asked for, with the arguments kept as the model sent them.
type turn struct {
	Text  string
	Calls []provider.ToolCall
}

// encodeTurn writes the answer of a model call as the JSON document of its step.
func encodeTurn(resp provider.Response) (string, error) {
	calls := make([]jsonvalue.Value, 0, len(resp.ToolCalls))
	for i := range resp.ToolCalls {
		c := resp.ToolCalls[i]
		raw := c.RawArguments
		if c.ParseError == "" {
			b, err := jsonvalue.Marshal(c.Arguments)
			if err != nil {
				return "", fmt.Errorf("agent: encode tool arguments: %w", err)
			}
			raw = string(b)
		}
		item, err := jsonvalue.Object(
			jsonvalue.Member{Key: "id", Value: jsonvalue.String(c.ID)},
			jsonvalue.Member{Key: "name", Value: jsonvalue.String(c.Name)},
			jsonvalue.Member{Key: "arguments", Value: jsonvalue.String(raw)},
			jsonvalue.Member{Key: "parseError", Value: jsonvalue.String(c.ParseError)},
		)
		if err != nil {
			return "", fmt.Errorf("agent: encode tool call: %w", err)
		}
		calls = append(calls, item)
	}
	doc, err := jsonvalue.Object(
		jsonvalue.Member{Key: "text", Value: jsonvalue.String(resp.Text)},
		jsonvalue.Member{Key: "toolCalls", Value: jsonvalue.Array(calls...)},
	)
	if err != nil {
		return "", fmt.Errorf("agent: encode answer: %w", err)
	}
	b, err := jsonvalue.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("agent: encode answer: %w", err)
	}
	return string(b), nil
}

// decodeTurn reads back the document written by encodeTurn.
func decodeTurn(doc string) (turn, error) {
	v, err := jsonvalue.Parse([]byte(doc))
	if err != nil {
		return turn{}, fmt.Errorf("agent: recorded answer is not JSON: %w", err)
	}
	var t turn
	if txt, ok := v.Get("text"); ok {
		t.Text, _ = txt.Str()
	}
	list, _ := v.Get("toolCalls")
	for _, item := range list.Items() {
		var c provider.ToolCall
		c.ID = strField(item, "id")
		c.Name = strField(item, "name")
		raw := strField(item, "arguments")
		c.RawArguments = raw
		c.ParseError = strField(item, "parseError")
		if c.ParseError == "" {
			args, perr := jsonvalue.Parse([]byte(raw))
			if perr != nil {
				c.ParseError = perr.Error()
			} else {
				c.Arguments = args
			}
		}
		t.Calls = append(t.Calls, c)
	}
	return t, nil
}

func strField(v jsonvalue.Value, key string) string {
	f, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, _ := f.Str()
	return s
}

// rowsOfTurn returns the transcript rows a model answer adds: its text, then
// one row per tool call, all as ASSISTANT messages.
func rowsOfTurn(resp provider.Response, sequence int) []TranscriptRow {
	var rows []TranscriptRow
	if resp.Text != "" {
		rows = append(rows, TranscriptRow{Sequence: sequence, Role: provider.RoleAssistant, Part: PartText, Text: resp.Text})
		sequence++
	}
	for i := range resp.ToolCalls {
		c := resp.ToolCalls[i]
		text := c.RawArguments
		if c.ParseError == "" {
			if b, err := jsonvalue.Marshal(c.Arguments); err == nil {
				text = string(b)
			}
		}
		rows = append(rows, TranscriptRow{Sequence: sequence, Role: provider.RoleAssistant, Part: PartToolCall, CallID: c.ID, ToolName: c.Name, Text: text})
		sequence++
	}
	return rows
}

// messagesOf rebuilds the provider messages of a transcript. Consecutive
// assistant rows form one message and consecutive tool results form one, so
// the provider sees each turn the way the model produced it.
func messagesOf(rows []TranscriptRow) []provider.Message {
	var msgs []provider.Message
	for i := range rows {
		r := rows[i]
		part := partOf(r)
		n := len(msgs)
		if n > 0 && msgs[n-1].Role == r.Role && r.Part != PartText && msgs[n-1].Parts[0].Kind != provider.PartText {
			msgs[n-1].Parts = append(msgs[n-1].Parts, part)
			continue
		}
		if n > 0 && r.Part == PartToolCall && msgs[n-1].Role == provider.RoleAssistant {
			msgs[n-1].Parts = append(msgs[n-1].Parts, part)
			continue
		}
		msgs = append(msgs, provider.Message{Role: r.Role, Parts: []provider.Part{part}})
	}
	return msgs
}

func partOf(r TranscriptRow) provider.Part {
	switch r.Part {
	case PartToolCall:
		c := provider.ToolCall{ID: r.CallID, Name: r.ToolName, RawArguments: r.Text}
		args, err := jsonvalue.Parse([]byte(r.Text))
		if err != nil {
			c.ParseError = err.Error()
		} else {
			c.Arguments = args
		}
		return provider.Part{Kind: provider.PartToolCall, ToolCall: c}
	case PartToolResult:
		return provider.Part{Kind: provider.PartToolResult, ToolResult: provider.ToolResult{
			CallID: r.CallID, Name: r.ToolName, Output: r.Text, IsError: r.IsError,
		}}
	default:
		return provider.Part{Kind: provider.PartText, Text: r.Text}
	}
}
