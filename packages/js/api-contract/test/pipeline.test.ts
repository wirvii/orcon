// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// jsdom gives the handlers a page origin to resolve their relative paths against, as in the app's tests.
// @vitest-environment jsdom

import { execFileSync } from 'node:child_process';
import { mkdirSync, readdirSync, readFileSync, rmSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { http, type HttpHandler } from 'msw';
import { setupServer } from 'msw/node';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import type { ZodType } from 'zod';

const packageDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const fixture = join(packageDir, 'test', 'fixtures', 'openapi.yaml');
// Inside the package, so the generated code resolves zod, msw and faker from here.
const scratch = join(packageDir, 'test', '.out', `pipeline-${String(process.pid)}`);

function generate(output: string): void {
  execFileSync(process.execPath, [join(packageDir, 'scripts', 'generate.mjs'), '--input', fixture, '--output', output], {
    cwd: packageDir,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

function filesUnder(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) =>
    entry.isDirectory() ? filesUnder(join(directory, entry.name)) : [join(directory, entry.name)],
  );
}

interface ZodModule {
  flowSchema: ZodType;
  flowPageSchema: ZodType;
  problemSchema: ZodType;
  createFlowRequestSchema: ZodType;
  listFlowsQueryParamsSchema: ZodType;
}

interface FakerModule {
  createFlow: () => unknown;
  createFlowPage: () => unknown;
  createGetFlow404: () => unknown;
  createCreateFlow422: () => unknown;
}

interface MswModule {
  handlers: readonly unknown[];
  listFlowsHandler: (data?: unknown) => HttpHandler;
  getFlowHandler: (data?: unknown) => HttpHandler;
  getFlowHandlerResponse404: (data: unknown) => Response;
  createFlowHandlerResponse422: (data: unknown) => Response;
}

let zod: ZodModule;
let faker: FakerModule;
let msw: MswModule;

beforeAll(async () => {
  mkdirSync(scratch, { recursive: true });
  generate(join(scratch, 'gen'));
  const load = async <T>(path: string) => (await import(/* @vite-ignore */ pathToFileURL(join(scratch, 'gen', path)).href)) as T;
  zod = await load<ZodModule>('zod/index.ts');
  faker = await load<FakerModule>('faker/index.ts');
  msw = await load<MswModule>('msw/index.ts');
}, 120_000);

afterAll(() => {
  rmSync(join(packageDir, 'test', '.out'), { recursive: true, force: true });
});

const flow = {
  id: '0b9f6b1e-8f0e-4a8e-9a39-6f0a3c1d2e4f',
  name: 'Nightly sync',
  status: 'ACTIVE',
  description: null,
  trigger: { kind: 'schedule', cron: '0 2 * * *' },
  draftRevision: 3,
};

describe('generated files', () => {
  it('start with the licence header and the generated notice, barrels included', () => {
    const files = filesUnder(join(scratch, 'gen')).filter((file) => file.endsWith('.ts'));
    expect(files.length).toBeGreaterThan(20);
    for (const file of files) {
      const text = readFileSync(file, 'utf8');
      expect(text, relative(scratch, file)).toMatch(/^\/\/ SPDX-FileCopyrightText: The Orcon Authors\n\/\/ SPDX-License-Identifier: Apache-2.0\n/);
    }
  });

  it('are identical on every run, so the committed output can be checked for drift', () => {
    const again = join(scratch, 'again');
    generate(again);
    const names = filesUnder(join(scratch, 'gen')).map((file) => relative(join(scratch, 'gen'), file)).sort();
    expect(filesUnder(again).map((file) => relative(again, file)).sort()).toEqual(names);
    for (const name of names) {
      expect(readFileSync(join(again, name), 'utf8'), name).toBe(readFileSync(join(scratch, 'gen', name), 'utf8'));
    }
  });

  it('use no extension on relative imports', () => {
    const text = readFileSync(join(scratch, 'gen', 'zod', 'flowSchema.ts'), 'utf8');
    expect(text).toContain('from "./scheduleTriggerSchema"');
    expect(text).not.toMatch(/from "\.\.?\/[^"]*\.ts"/);
  });
});

describe('Zod schemas (Zod 4)', () => {
  it('accept the example the contract describes, including a null where the type allows null', () => {
    expect(zod.flowSchema.safeParse(flow).success).toBe(true);
    expect(zod.flowSchema.safeParse({ ...flow, description: 'Syncs the CRM.' }).success).toBe(true);
  });

  it('accept both members of a oneOf and reject anything that matches neither', () => {
    expect(zod.flowSchema.safeParse({ ...flow, trigger: { kind: 'webhook', path: '/hooks/crm' } }).success).toBe(true);
    expect(zod.flowSchema.safeParse({ ...flow, trigger: { kind: 'manual' } }).success).toBe(false);
    expect(zod.flowSchema.safeParse({ ...flow, trigger: { kind: 'schedule', path: '/x' } }).success).toBe(false);
  });

  it('reject an example that breaks a rule of the contract', () => {
    expect(zod.flowSchema.safeParse({ ...flow, id: 'not-a-uuid' }).success).toBe(false);
    expect(zod.flowSchema.safeParse({ ...flow, name: '' }).success).toBe(false);
    expect(zod.flowSchema.safeParse({ ...flow, name: 'x'.repeat(121) }).success).toBe(false);
    expect(zod.flowSchema.safeParse({ ...flow, status: 'PAUSED' }).success).toBe(false);
    expect(zod.flowSchema.safeParse({ ...flow, draftRevision: -1 }).success).toBe(false);
    expect(zod.flowSchema.safeParse({ ...flow, description: undefined }).success).toBe(false);
  });

  it('describe a request body and the query of a cursor-paginated list', () => {
    expect(zod.createFlowRequestSchema.safeParse({ name: 'New flow' }).success).toBe(true);
    expect(zod.createFlowRequestSchema.safeParse({}).success).toBe(false);
    expect(zod.listFlowsQueryParamsSchema.safeParse({ pageToken: 'abc', limit: 200 }).success).toBe(true);
    expect(zod.listFlowsQueryParamsSchema.safeParse({ limit: 201 }).success).toBe(false);
    expect(zod.listFlowsQueryParamsSchema.safeParse({ limit: 0 }).success).toBe(false);
  });

  it('accept a page with and without a next cursor', () => {
    expect(zod.flowPageSchema.safeParse({ items: [flow], nextPageToken: 'next' }).success).toBe(true);
    expect(zod.flowPageSchema.safeParse({ items: [], nextPageToken: null }).success).toBe(true);
    expect(zod.flowPageSchema.safeParse({ items: [] }).success).toBe(false);
  });

  it('accept a problem+json body with field errors', () => {
    const problem = {
      type: 'https://orcon.dev/problems/validation',
      title: 'Validation failed',
      status: 422,
      errors: [{ pointer: '/name', code: 'too-short', message: 'Use at least 1 character.' }],
    };
    expect(zod.problemSchema.safeParse(problem).success).toBe(true);
    expect(zod.problemSchema.safeParse({ ...problem, errors: [{ pointer: '/name' }] }).success).toBe(false);
  });
});

describe('example data', () => {
  it('is valid against the schemas it was generated with', () => {
    expect(zod.flowSchema.safeParse(faker.createFlow()).success).toBe(true);
    expect(zod.flowPageSchema.safeParse(faker.createFlowPage()).success).toBe(true);
    expect(zod.problemSchema.safeParse(faker.createGetFlow404()).success).toBe(true);
    expect(zod.problemSchema.safeParse(faker.createCreateFlow422()).success).toBe(true);
  });

  it('is the same on every call (the faker seed is fixed)', () => {
    expect(faker.createFlow()).toEqual(faker.createFlow());
  });
});

describe('Mock Service Worker handlers', () => {
  const server = setupServer();
  beforeAll(() => {
    server.listen({ onUnhandledRequest: 'error' });
  });
  afterAll(() => {
    server.close();
  });

  const origin = window.location.origin;

  it('serves the list with example data that matches the schema', async () => {
    server.use(msw.listFlowsHandler());
    const response = await fetch(`${origin}/api/v1/flows?limit=2`);
    expect(response.status).toBe(200);
    expect(response.headers.get('content-type')).toBe('application/json');
    expect(zod.flowPageSchema.safeParse(await response.json()).success).toBe(true);
  });

  it('serves one flow, or the data a test passes in', async () => {
    server.use(msw.getFlowHandler(flow));
    const response = await fetch(`${origin}/api/v1/flows/${flow.id}`);
    expect(await response.json()).toEqual(flow);
  });

  it('serves a declared error as problem+json that matches the schema', async () => {
    server.use(msw.getFlowHandler(() => msw.getFlowHandlerResponse404(faker.createGetFlow404())));
    const response = await fetch(`${origin}/api/v1/flows/${flow.id}`);
    expect(response.status).toBe(404);
    expect(response.headers.get('content-type')).toBe('application/problem+json');
    expect(zod.problemSchema.safeParse(await response.json()).success).toBe(true);
  });

  it('serves a validation problem with field errors', async () => {
    server.use(http.post(`${origin}/api/v1/flows`, () => msw.createFlowHandlerResponse422(faker.createCreateFlow422())));
    const response = await fetch(`${origin}/api/v1/flows`, { method: 'POST', body: '{}' });
    expect(response.status).toBe(422);
    const body = (await response.json()) as { errors?: { pointer: string }[] };
    expect(zod.problemSchema.safeParse(body).success).toBe(true);
    expect(body.errors?.every((error) => error.pointer.length > 0) ?? true).toBe(true);
  });

  it('lists a default handler per operation', () => {
    expect(msw.handlers).toHaveLength(3);
  });
});
