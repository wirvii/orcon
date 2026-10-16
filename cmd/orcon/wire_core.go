// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/wirvii/orcon/internal/adapter/connectors/core"
	"github.com/wirvii/orcon/internal/adapter/exprflow"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// coreEvaluator gives the core nodes the expression language of the engine. It
// is bound to the recorded environment of each activation before it evaluates,
// so a recomputed activation reads the same $now and the same random values.
type coreEvaluator struct {
	routing *exprflow.Routing
	env     execution.NodeEnv
}

var _ core.ActivationEvaluator = coreEvaluator{}

// Evaluate implements [core.Evaluator].
func (e coreEvaluator) Evaluate(ctx context.Context, template string, item jsonvalue.Value) (jsonvalue.Value, error) {
	return e.routing.EvalValue(ctx, template, item, e.env)
}

// Bind implements [core.ActivationEvaluator].
func (e coreEvaluator) Bind(env execution.NodeEnv) core.Evaluator { //nolint:ireturn // the port returns the evaluator of one activation
	e.env = env
	return e
}
