// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/trigger"
)

// WebhookPrefix is the path prefix of the public addresses of triggers (SPEC-002
// R-ADDR-02: /hooks/<project>/<path>). The router serves it beside /api/.
const WebhookPrefix = "/hooks/"

// WebhookBodyLimit bounds the body of one public request. The per-flow limits
// of SPEC-002 R-ING-03 arrive with the ingress cut that adds them.
const WebhookBodyLimit = 1 << 20

// WebhookIngress accepts the public requests addressed to triggers. The trigger
// service implements it.
type WebhookIngress interface {
	Ingest(ctx context.Context, req trigger.IngressRequest) (trigger.Accepted, error)
}

// webhookHandler serves the public addresses: it reads the body, hands the
// request to the ingress and answers 202 Accepted with the execution started.
// Problems are answered as the rest of the API does; nothing of another
// organization reaches the response.
func webhookHandler(in WebhookIngress, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, WebhookBodyLimit))
		if err != nil {
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				problem.Write(w, r, problem.PayloadTooLarge, problem.Detail("the body is larger than the limit"))
				return
			}
			problem.Write(w, r, problem.ValidationFailed, problem.Detail("the body could not be read"))
			return
		}
		acc, err := in.Ingest(r.Context(), trigger.IngressRequest{Method: r.Method, Path: r.URL.Path, Body: body})
		if err != nil {
			writeIngressProblem(w, r, logger, err)
			return
		}
		w.Header().Set("Orcon-Execution-Id", acc.ExecutionID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{"executionId": acc.ExecutionID})
	})
}

// writeIngressProblem maps the errors of the ingress to the problem catalog.
func writeIngressProblem(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	switch {
	case errors.Is(err, trigger.ErrNoRoute):
		problem.Write(w, r, problem.NotFound, problem.Detail("no active address answers this request"))
	case errors.Is(err, trigger.ErrInactive):
		problem.Write(w, r, problem.Conflict, problem.Detail("the flow does not accept events right now"))
	case errors.Is(err, trigger.ErrBadBody):
		problem.Write(w, r, problem.ValidationFailed, problem.Detail("the body must be JSON"))
	case errors.Is(err, trigger.ErrNoStarter):
		problem.Write(w, r, problem.ServiceUnavailable, problem.Detail("the engine is not running in this process"))
	default:
		logger.ErrorContext(r.Context(), "webhook ingress failed", "error", err)
		problem.Write(w, r, problem.InternalError)
	}
}
