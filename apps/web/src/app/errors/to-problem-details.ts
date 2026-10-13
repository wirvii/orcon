// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ProblemDetails } from '@orcon/ui';
import { z } from 'zod';

import { ContractError } from './contract-error';

/**
 * The problem+json body as the API sends it (SPEC-005 R-CONV-03): field errors carry
 * `message`. Everything is optional: a problem is read leniently, because an error
 * response must still be shown when it is not perfectly formed.
 */
const fieldErrorSchema = z.object({
  pointer: z.string(),
  code: z.string().optional(),
  message: z.string().optional(),
  detail: z.string().optional(),
});

const problemSchema = z.object({
  type: z.string().optional(),
  title: z.string().optional(),
  status: z.number().int().optional(),
  detail: z.string().optional(),
  instance: z.string().optional(),
  code: z.string().optional(),
  requestId: z.string().optional(),
  errors: z.array(fieldErrorSchema).optional(),
});

/** The HTTP status of a failure, when it has one. */
export function problemStatus(error: unknown): number | undefined {
  return toProblemDetails(error).status;
}

/**
 * The only place that reads an error into the `ProblemDetails` the UI components
 * show (orcon-web ER-1). `@orcon/ui` names the text of a field error `detail`, the API
 * names it `message`; this adapter maps one to the other (see SPEC-010 K2).
 * A contract error and any error without a problem body become a code-only problem,
 * which the UI shows as the generic message.
 */
export function toProblemDetails(error: unknown): ProblemDetails {
  if (error instanceof ContractError) return { code: 'contract-error' };
  const parsed = problemSchema.safeParse(error);
  if (!parsed.success) return {};
  const { errors, ...rest } = parsed.data;
  return {
    ...rest,
    ...(errors
      ? {
          errors: errors.map((entry) => ({
            pointer: entry.pointer,
            detail: entry.message ?? entry.detail ?? '',
            ...(entry.code === undefined ? {} : { code: entry.code }),
          })),
        }
      : {}),
  };
}
