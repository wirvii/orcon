// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// FinalAnswerTool is the name of the tool a forced structured answer uses
// when the model has no native JSON schema mode (ADR-0061 section 4).
const FinalAnswerTool = "final_answer"

// DefaultRepairAttempts is the number of repair calls allowed after the first
// answer when [StructuredOptions.RepairAttempts] is zero.
const DefaultRepairAttempts = 2

// ErrOutputInvalid reports that the model did not produce an answer matching
// the schema within the repair budget. It is a definitive failure of the step.
var ErrOutputInvalid = errors.New("structured output does not match the schema")

// StructuredMode is the way a structured answer is requested from a model.
type StructuredMode string

// The structured modes, in order of preference (ADR-0061 section 4).
const (
	// StructuredNative uses the provider's JSON schema response mode.
	StructuredNative StructuredMode = "NATIVE"
	// StructuredForcedTool forces a call to the final_answer tool whose
	// arguments are the answer.
	StructuredForcedTool StructuredMode = "FORCED_TOOL"
	// StructuredRepaired asks for JSON in plain text, validates it and repairs
	// it with bounded follow-up calls.
	StructuredRepaired StructuredMode = "PROMPT_REPAIR"
)

// StructuredOptions bounds one structured call.
type StructuredOptions struct {
	// RepairAttempts is how many follow-up calls may fix an invalid answer.
	// Zero means [DefaultRepairAttempts].
	RepairAttempts int
	// OnCall is told about every provider call, first answer and repairs
	// alike, so each one can be recorded as a step. It may be nil.
	OnCall func(CallRecord)
}

// CallRecord describes one provider call made by [Structured].
type CallRecord struct {
	Mode StructuredMode
	// Repair is zero for the first call and counts the repairs after it.
	Repair   int
	Response Response
}

// StructuredResult is the validated answer and how it was obtained.
type StructuredResult struct {
	Value jsonvalue.Value
	Mode  StructuredMode
	// Calls is the number of provider calls, including repairs.
	Calls int
	// Repairs is the number of follow-up calls made after an invalid answer.
	Repairs int
}

// Structured asks p for an answer that matches req.ResponseFormat.Schema. The
// mode follows the model's capabilities: native when it supports JSON schema
// output, a forced final_answer tool when it supports tools, and plain JSON
// with repair otherwise. The answer is returned only after it validates; an
// answer that stays invalid after the repair budget fails with
// [ErrOutputInvalid]. Provider errors are returned unchanged so the caller can
// apply its fallback rules.
func Structured(ctx context.Context, p ModelProvider, req Request, opt StructuredOptions) (StructuredResult, error) {
	if req.ResponseFormat.Kind != FormatJSONSchema || req.ResponseFormat.Schema.IsZero() {
		return StructuredResult{}, errors.New("structured: the request needs a JSON schema response format")
	}
	caps := p.Capabilities(req.Model)
	mode := StructuredRepaired
	switch {
	case caps.JSONSchema:
		mode = StructuredNative
	case caps.Tools:
		mode = StructuredForcedTool
	}
	attempts := opt.RepairAttempts
	if attempts == 0 {
		attempts = DefaultRepairAttempts
	}

	work := prepareStructured(req, mode)
	res := StructuredResult{Mode: mode}
	for repair := 0; ; repair++ {
		resp, err := p.Generate(ctx, work)
		if err != nil {
			return res, err
		}
		res.Calls++
		if opt.OnCall != nil {
			opt.OnCall(CallRecord{Mode: mode, Repair: repair, Response: resp})
		}
		value, reason := readStructured(mode, resp, req.ResponseFormat.Schema)
		if reason == "" {
			res.Value = value
			return res, nil
		}
		if repair >= attempts {
			return res, fmt.Errorf("%w after %d repair(s): %s", ErrOutputInvalid, repair, reason)
		}
		res.Repairs++
		work.Messages = appendRepair(work.Messages, mode, resp, reason)
	}
}

// prepareStructured adapts the request to the mode: the schema goes to the
// native response format, to a forced tool, or into an instruction.
func prepareStructured(req Request, mode StructuredMode) Request {
	work := req
	work.Messages = append([]Message(nil), req.Messages...)
	schema := req.ResponseFormat.Schema
	switch mode {
	case StructuredNative:
		// The request already carries the JSON schema response format.
	case StructuredForcedTool:
		work.Tools = append(append([]Tool(nil), req.Tools...), Tool{
			Name:        FinalAnswerTool,
			Description: "Give the final answer. Its arguments are the answer.",
			Schema:      schema,
		})
		work.ToolChoice = ToolChoice{Mode: ToolChoiceNamed, Name: FinalAnswerTool}
		work.ResponseFormat = ResponseFormat{Kind: FormatText}
	case StructuredRepaired:
		text, _ := jsonvalue.Marshal(schema)
		instruction := "Answer only with one JSON value that matches this JSON Schema, with no text around it:\n" + string(text)
		work.Messages = append([]Message{{Role: RoleSystem, Parts: []Part{{Kind: PartText, Text: instruction}}}}, work.Messages...)
		work.ResponseFormat = ResponseFormat{Kind: FormatJSON}
	}
	return work
}

// readStructured extracts the answer of one response and validates it. A
// non-empty reason means the answer is not usable.
func readStructured(mode StructuredMode, resp Response, schema jsonvalue.Value) (jsonvalue.Value, string) {
	var candidate jsonvalue.Value
	if mode == StructuredForcedTool {
		call, ok := finalAnswerCall(resp)
		if !ok {
			return jsonvalue.Value{}, "the model did not call " + FinalAnswerTool
		}
		if call.ParseError != "" {
			return jsonvalue.Value{}, "the arguments of " + FinalAnswerTool + " are not valid JSON: " + call.ParseError
		}
		candidate = call.Arguments
	} else {
		v, err := jsonvalue.Parse([]byte(strings.TrimSpace(resp.Text)))
		if err != nil {
			return jsonvalue.Value{}, "the answer is not valid JSON"
		}
		candidate = v
	}
	if err := ValidateArguments(schema, candidate); err != nil {
		return jsonvalue.Value{}, err.Error()
	}
	return candidate, ""
}

// finalAnswerCall returns the first call to the final_answer tool.
func finalAnswerCall(resp Response) (ToolCall, bool) {
	for i := range resp.ToolCalls {
		if resp.ToolCalls[i].Name == FinalAnswerTool {
			return resp.ToolCalls[i], true
		}
	}
	return ToolCall{}, false
}

// appendRepair records the invalid answer and the reason, then asks the model
// to correct it. In forced-tool mode the reason answers the tool call, so the
// transcript stays well formed for the provider.
func appendRepair(msgs []Message, mode StructuredMode, resp Response, reason string) []Message {
	if mode == StructuredForcedTool {
		if call, ok := finalAnswerCall(resp); ok {
			msgs = append(msgs, Message{Role: RoleAssistant, Parts: []Part{{Kind: PartToolCall, ToolCall: call}}})
			return append(msgs, Message{Role: RoleTool, Parts: []Part{{Kind: PartToolResult, ToolResult: ToolResult{
				CallID: call.ID, Name: FinalAnswerTool, Output: reason, IsError: true,
			}}}})
		}
	}
	if resp.Text != "" {
		msgs = append(msgs, Message{Role: RoleAssistant, Parts: []Part{{Kind: PartText, Text: resp.Text}}})
	}
	return append(msgs, Message{Role: RoleUser, Parts: []Part{{Kind: PartText, Text: "The answer is not valid: " + reason + ". Answer again with only the corrected JSON."}}})
}
