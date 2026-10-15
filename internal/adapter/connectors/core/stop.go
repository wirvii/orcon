// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Parameter names of orcon.stopAndError@1.
const (
	paramErrorCode = "errorCode"
	paramMessage   = "message"
	maxMessageLen  = 4096
)

// errorCodePattern is the shape of an error code: UPPER_SNAKE, as the codes of
// the engine are written (ADR-0080 section 3).
var errorCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// StopParams is the parsed parameters of orcon.stopAndError@1.
type StopParams struct {
	// Code identifies the failure in the execution record.
	Code string
	// Message is optional text for people reading the failure.
	Message string
}

// ParseStopAndError reads the parameters of orcon.stopAndError@1. The errorCode
// parameter is required; message is optional. Any other key is an error.
func ParseStopAndError(params jsonvalue.Value) (StopParams, error) {
	if params.Kind() != jsonvalue.KindObject {
		return StopParams{}, errors.New("parameters must be an object")
	}
	var out StopParams
	var hasCode bool
	for _, m := range params.Members() {
		switch m.Key {
		case paramErrorCode:
			code, ok := m.Value.Str()
			if !ok || !errorCodePattern.MatchString(code) {
				return StopParams{}, fmt.Errorf("%s must match %s", paramErrorCode, errorCodePattern.String())
			}
			out.Code, hasCode = code, true
		case paramMessage:
			msg, ok := m.Value.Str()
			if !ok {
				return StopParams{}, fmt.Errorf("%s must be a string", paramMessage)
			}
			if len(msg) > maxMessageLen {
				return StopParams{}, fmt.Errorf("%s is longer than %d bytes", paramMessage, maxMessageLen)
			}
			out.Message = msg
		default:
			return StopParams{}, fmt.Errorf("unknown parameter %q", m.Key)
		}
	}
	if !hasCode {
		return StopParams{}, fmt.Errorf("%s is required", paramErrorCode)
	}
	return out, nil
}

// StopError is the failure the node raises. It is permanent: retrying the same
// flow with the same parameters fails the same way.
func (p StopParams) StopError() *execution.NodeError {
	msg := p.Message
	if msg == "" {
		msg = "the flow stopped by orcon.stopAndError"
	}
	return &execution.NodeError{Code: p.Code, Class: execution.ClassPermanent, Err: errors.New(msg)}
}
