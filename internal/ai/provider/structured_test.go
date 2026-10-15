// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// scripted is a ModelProvider that answers from a fixed list and records the
// requests it received.
type scripted struct {
	caps      Capabilities
	responses []Response
	requests  []Request
}

func (s *scripted) Capabilities(string) Capabilities { return s.caps }

func (s *scripted) Generate(_ context.Context, req Request) (Response, error) {
	s.requests = append(s.requests, req)
	if len(s.responses) == 0 {
		return Response{}, errors.New("scripted: no more responses")
	}
	r := s.responses[0]
	s.responses = s.responses[1:]
	return r, nil
}

// answerSchema requires an object with a string "city" field.
func answerSchema(t *testing.T) jsonvalue.Value {
	t.Helper()
	s, err := jsonvalue.Parse([]byte(`{"type":"object","required":["city"],"properties":{"city":{"type":"string"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func schemaRequest(t *testing.T) Request {
	t.Helper()
	return Request{
		Model:          "m",
		Messages:       []Message{{Role: RoleUser, Parts: []Part{{Kind: PartText, Text: "where is it?"}}}},
		ResponseFormat: ResponseFormat{Kind: FormatJSONSchema, Schema: answerSchema(t), Strict: true},
	}
}

// TestStructuredOutputNative: a model with a JSON schema mode gets the schema
// as its response format and one call is enough.
func TestStructuredOutputNative(t *testing.T) {
	t.Parallel()
	p := &scripted{
		caps:      Capabilities{JSONSchema: true, Tools: true},
		responses: []Response{{Text: `{"city":"Bogota"}`, FinishReason: FinishStop}},
	}
	var calls int
	res, err := Structured(t.Context(), p, schemaRequest(t), StructuredOptions{OnCall: func(CallRecord) { calls++ }})
	if err != nil {
		t.Fatalf("Structured() error = %v", err)
	}
	if res.Mode != StructuredNative || res.Calls != 1 || res.Repairs != 0 || calls != 1 {
		t.Errorf("result = %+v, calls recorded = %d; want native, one call", res, calls)
	}
	if got := p.requests[0].ResponseFormat.Kind; got != FormatJSONSchema {
		t.Errorf("response format = %s, want JSON_SCHEMA", got)
	}
	if city, _ := res.Value.Get("city"); city.IsZero() {
		t.Errorf("value = %v, want the city field", res.Value)
	}
}

// TestStructuredOutputForcedTool: without a schema mode, the answer arrives as
// the arguments of a forced final_answer call.
func TestStructuredOutputForcedTool(t *testing.T) {
	t.Parallel()
	p := &scripted{
		caps: Capabilities{Tools: true},
		responses: []Response{{
			FinishReason: FinishToolCalls,
			ToolCalls:    []ToolCall{{ID: "c1", Name: FinalAnswerTool, Arguments: mustObject(t, `{"city":"Cali"}`)}},
		}},
	}
	res, err := Structured(t.Context(), p, schemaRequest(t), StructuredOptions{})
	if err != nil {
		t.Fatalf("Structured() error = %v", err)
	}
	if res.Mode != StructuredForcedTool || res.Calls != 1 {
		t.Errorf("result = %+v, want forced-tool mode with one call", res)
	}
	sent := p.requests[0]
	if len(sent.Tools) != 1 || sent.Tools[0].Name != FinalAnswerTool {
		t.Errorf("tools sent = %+v, want the final_answer tool", sent.Tools)
	}
	if sent.ToolChoice.Mode != ToolChoiceNamed || sent.ToolChoice.Name != FinalAnswerTool {
		t.Errorf("tool choice = %+v, want the named final_answer tool", sent.ToolChoice)
	}
}

// TestStructuredOutputRepairBounded: an invalid answer is repaired at most
// RepairAttempts times, each repair is reported as a call, and an answer that
// stays invalid fails with ErrOutputInvalid.
func TestStructuredOutputRepairBounded(t *testing.T) {
	t.Parallel()

	t.Run("a repaired answer succeeds and is counted", func(t *testing.T) {
		t.Parallel()
		p := &scripted{
			caps: Capabilities{},
			responses: []Response{
				{Text: "Sure! Here it is: {}"},
				{Text: `{"city":"Medellin"}`},
			},
		}
		var records []CallRecord
		res, err := Structured(t.Context(), p, schemaRequest(t), StructuredOptions{OnCall: func(r CallRecord) { records = append(records, r) }})
		if err != nil {
			t.Fatalf("Structured() error = %v", err)
		}
		if res.Mode != StructuredRepaired || res.Calls != 2 || res.Repairs != 1 {
			t.Errorf("result = %+v, want two calls and one repair", res)
		}
		if len(records) != 2 || records[1].Repair != 1 {
			t.Errorf("recorded calls = %+v, want the repair recorded as call 1", records)
		}
	})

	t.Run("the budget stops the loop", func(t *testing.T) {
		t.Parallel()
		p := &scripted{
			caps: Capabilities{},
			responses: []Response{
				{Text: "not json"},
				{Text: `{"city":5}`},
				{Text: `{"other":1}`},
				{Text: `{"city":"never reached"}`},
			},
		}
		var recorded int
		_, err := Structured(t.Context(), p, schemaRequest(t), StructuredOptions{
			RepairAttempts: 2,
			OnCall:         func(CallRecord) { recorded++ },
		})
		if !errors.Is(err, ErrOutputInvalid) {
			t.Fatalf("Structured() error = %v, want ErrOutputInvalid", err)
		}
		if len(p.requests) != 3 || recorded != 3 {
			t.Errorf("calls = %d, recorded = %d; want exactly 1 answer + 2 repairs", len(p.requests), recorded)
		}
	})
}

// TestStructuredRejectsRequestWithoutSchema: the entry point refuses a request
// that does not ask for a schema.
func TestStructuredRejectsRequestWithoutSchema(t *testing.T) {
	t.Parallel()
	p := &scripted{}
	if _, err := Structured(t.Context(), p, Request{Model: "m"}, StructuredOptions{}); err == nil {
		t.Error("Structured() without a schema succeeded")
	}
}

func mustObject(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
