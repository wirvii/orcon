// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useContext, useMemo, type ReactNode } from 'react';

import { errorsByPointer, type ProblemDetails } from '../problem-alert/problem';

const ProblemErrorsContext = createContext<Record<string, string>>({});

/**
 * Hands the per-field errors of a problem+json response to the FormFields
 * below it. A FormField with `name="/title"` shows the error whose JSON
 * pointer is "/title".
 */
export function ProblemErrorsProvider({ problem, children }: { problem: ProblemDetails | undefined; children: ReactNode }) {
  const errors = useMemo(() => (problem ? errorsByPointer(problem) : {}), [problem]);
  return <ProblemErrorsContext.Provider value={errors}>{children}</ProblemErrorsContext.Provider>;
}

export function useFieldProblemError(name: string | undefined): string | undefined {
  const errors = useContext(ProblemErrorsContext);
  return name ? errors[name] : undefined;
}
