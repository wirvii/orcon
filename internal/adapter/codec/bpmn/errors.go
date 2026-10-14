// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package bpmn

import "fmt"

// UnsupportedError reports a part of a flow that this version of the codec
// has no BPMN form for yet.
type UnsupportedError struct {
	// Pointer is the JSON Pointer of the member in the flow document.
	Pointer string
	// What names the unsupported element class or block.
	What string
}

// Error names the member and what is not supported.
func (e *UnsupportedError) Error() string {
	return fmt.Sprintf("bpmn: %s at %s is not supported yet", e.What, e.Pointer)
}

// UnrepresentableError reports a string that XML 1.0 cannot carry, such as one
// with a control character. The codec refuses it instead of changing it.
type UnrepresentableError struct {
	// Pointer is the JSON Pointer of the member in the flow document.
	Pointer string
}

// Error names the member holding the string.
func (e *UnrepresentableError) Error() string {
	return fmt.Sprintf("bpmn: the string at %s has a character XML 1.0 cannot carry", e.Pointer)
}

// FormatError reports a document that is not BPMN produced by Orcon.
type FormatError struct {
	// Detail says what is wrong, without echoing document content.
	Detail string
}

// Error returns the detail.
func (e *FormatError) Error() string { return "bpmn: " + e.Detail }

func formatErrorf(format string, args ...any) error {
	return &FormatError{Detail: fmt.Sprintf(format, args...)}
}
