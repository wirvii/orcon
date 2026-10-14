// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"fmt"
	"strings"
)

// Request body limits (orcon-go API-5.2, SPEC-005 Amendment 8 section 8-5).
// There is one table, by operationId, and no extension in the contract: the
// pipeline applies the limit before it authenticates, and each operation with a
// body ends its description with "Request body limit: <n> MiB." so clients can
// read it. TestBodyLimitsDocumented keeps the two in step. Raising a limit
// later is additive; lowering it breaks clients.
const (
	mebibyte = 1 << 20

	// DefaultBodyLimit applies to every operation not listed in bodyLimitMiB.
	DefaultBodyLimit = 1 * mebibyte
	// The limit of operations that carry a whole flow document.
	documentBodyLimit = 8 * mebibyte
	// The limit of the n8n converter and of pins.
	payloadBodyLimit = 2 * mebibyte
)

// BodyLimitFor returns the request body limit of an operation in bytes.
func BodyLimitFor(operationID string) int64 {
	switch operationID {
	case "replaceDraft", "updateDraft", "createTestRun":
		return documentBodyLimit
	case "evaluateN8nConverter", "createPin", "replacePin":
		return payloadBodyLimit
	default:
		return DefaultBodyLimit
	}
}

// bodyLimitSentence is the closing sentence of the description of an operation
// with a request body.
func bodyLimitSentence(operationID string) string {
	return fmt.Sprintf("Request body limit: %d MiB.", BodyLimitFor(operationID)/mebibyte)
}

// bodyLimitProblems lists the operations with a request body whose description
// does not end with the sentence the table dictates.
func bodyLimitProblems(spec *Spec) []string {
	var out []string
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if !op.HasBody {
			continue
		}
		if !strings.HasSuffix(strings.TrimSpace(op.Description), bodyLimitSentence(op.ID)) {
			out = append(out, fmt.Sprintf("%s must end its description with %q", op.ID, bodyLimitSentence(op.ID)))
		}
	}
	return out
}
