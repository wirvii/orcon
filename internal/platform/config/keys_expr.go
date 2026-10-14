// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package config

import "strconv"

// keysExpr declares the keys of the expression sandbox. They belong to SPEC-001
// (cut C5, ADR-0018 section 3, amendment 10) and are registered here on its
// behalf; their paths are chosen so the derived flags are the names the server
// has always had: --expr-sandbox-slots and --expr-max-abandoned.
func keysExpr() []Key {
	rolesEvaluating := []Role{RoleStandalone, RoleAPI, RoleWorker}
	return []Key{
		{
			Path: "expr.sandboxSlots", Kind: KindInt, Default: "0", HasRange: true, Min: 0, Max: 1024, Roles: rolesEvaluating,
			DefaultDoc: text("0, which means GOMAXPROCS - 1", "0, que significa GOMAXPROCS - 1"),
			Description: text(
				"Expression sandbox calls that may execute guest code at once, from 1 to GOMAXPROCS - 1. 0 means GOMAXPROCS - 1. The server rejects a value above GOMAXPROCS - 1 at start.",
				"Llamadas del sandbox de expresiones que pueden ejecutar código invitado a la vez, de 1 a GOMAXPROCS - 1. 0 significa GOMAXPROCS - 1. El servidor rechaza al arrancar un valor mayor que GOMAXPROCS - 1."),
		},
		{
			Path: "expr.maxAbandoned", Kind: KindInt, Default: strconv.Itoa(SandboxMaxAbandonedDefault), HasRange: true,
			Min: SandboxMaxAbandonedMin, Max: SandboxMaxAbandonedMax, Roles: rolesEvaluating,
			Description: text(
				"Maximum number of expression evaluations abandoned at their deadline that may still be running. When the limit is reached, new sandbox calls fail at once and a lasting saturation restarts the process.",
				"Número máximo de evaluaciones de expresiones abandonadas al vencer su plazo que pueden seguir ejecutándose. Al llegar al límite, las llamadas nuevas al sandbox fallan de inmediato y una saturación prolongada reinicia el proceso."),
		},
	}
}
