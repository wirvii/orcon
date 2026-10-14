// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { z } from 'zod';

import { ContractError } from '../../../app/errors/contract-error';

/**
 * The transport of the session operations. It is the one place of the session area that calls
 * `fetch`: it sends the cookies of the same origin, copies the CSRF cookie into the header of
 * every unsafe request and turns an error answer into an {@link ApiProblem}. When the generated
 * SDK (`@orcon/sdk`, SPEC-005 API2-contract) exists, the operations in `operations.ts` call it
 * instead and this file goes away.
 */

const API_PREFIX = '/api/v1';
/** The CSRF cookie is readable by the page on purpose: the page echoes it in a header (SPEC-016 R-SH-SES-03). */
const CSRF_COOKIE = '__Host-orcon_csrf';
const CSRF_HEADER = 'Orcon-Csrf-Token';
const UNSAFE_METHODS = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

/** The problem+json body of an error answer, read leniently (every field may be missing). */
export interface ProblemBody {
  type?: string | undefined;
  title?: string | undefined;
  status?: number | undefined;
  code?: string | undefined;
  detail?: string | undefined;
  requestId?: string | undefined;
  errors?: { pointer: string; code?: string | undefined; message?: string | undefined }[] | undefined;
}

/**
 * A failed answer from the server. It is an `Error` for the logs, and it carries the fields of the
 * problem so `toProblemDetails` can read it like any other problem.
 */
export class ApiProblem extends Error {
  override readonly name = 'ApiProblem';
  readonly status: number;
  readonly code: string | undefined;
  readonly title: string | undefined;
  readonly detail: string | undefined;
  readonly requestId: string | undefined;
  readonly errors: ProblemBody['errors'];
  /** Seconds the server asked to wait (`Retry-After`), when it sent one. */
  readonly retryAfterSeconds: number | undefined;

  constructor(status: number, body: ProblemBody, retryAfter: string | null) {
    super(body.detail ?? body.title ?? `The request failed with status ${status}.`);
    this.status = status;
    this.code = body.code;
    this.title = body.title;
    this.detail = body.detail;
    this.requestId = body.requestId;
    this.errors = body.errors;
    const seconds = retryAfter === null ? Number.NaN : Number(retryAfter);
    this.retryAfterSeconds = Number.isFinite(seconds) ? seconds : undefined;
  }
}

export interface ApiRequest<T> {
  method: 'GET' | 'POST';
  /** The path below `/api/v1`, starting with `/`. */
  path: string;
  query?: Record<string, string | number | undefined> | undefined;
  body?: unknown;
  /** Parses the success body. A body that does not match is a contract error (orcon-web ER-5). */
  schema?: z.ZodType<T> | undefined;
  /** Names the answer in a contract error, for example "response of GET /users/me". */
  source: string;
}

/** The CSRF token the page echoes back, when the server has set its cookie. */
export function csrfToken(): string | undefined {
  const prefix = `${CSRF_COOKIE}=`;
  const entry = document.cookie.split('; ').find((cookie) => cookie.startsWith(prefix));
  return entry === undefined ? undefined : decodeURIComponent(entry.slice(prefix.length));
}

export async function apiRequest<T>({ method, path, query, body, schema, source }: ApiRequest<T>): Promise<T> {
  const url = new URL(`${API_PREFIX}${path}`, window.location.origin);
  for (const [key, value] of Object.entries(query ?? {})) {
    if (value !== undefined) url.searchParams.set(key, String(value));
  }

  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (UNSAFE_METHODS.has(method)) {
    const token = csrfToken();
    if (token !== undefined) headers[CSRF_HEADER] = token;
  }

  // The one exception to DT-1 until `@orcon/sdk` exists: this transport is replaced by the SDK when
  // SPEC-016 SH1 lands, and the rule applies again from then on.
  // eslint-disable-next-line no-restricted-globals -- temporary transport, see the comment above the function
  const response = await fetch(url, {
    method,
    headers,
    credentials: 'same-origin',
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });

  if (!response.ok) {
    const problem = (await response.json().catch(() => ({}))) as ProblemBody;
    throw new ApiProblem(response.status, problem, response.headers.get('Retry-After'));
  }
  if (response.status === 204 || schema === undefined) return undefined as T;

  const json: unknown = await response.json();
  const parsed = schema.safeParse(json);
  if (!parsed.success) throw new ContractError(source, parsed.error.issues);
  return parsed.data;
}
