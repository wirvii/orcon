// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, readdirSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { ESLint, type Linter } from 'eslint';
import { describe, expect, it } from 'vitest';

const packageDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const webFixtures = join(packageDir, 'test', 'fixtures', 'web');

type ConfigFactory = (options: { appRoot?: string; tsconfigRootDir?: string; typed?: boolean; network?: boolean }) => Linter.Config[];

async function loadConfigs() {
  const base = (await import(pathToFileURL(join(packageDir, 'eslint.config.mjs')).href)) as { createConfig: () => Linter.Config[] };
  const web = (await import(pathToFileURL(join(packageDir, 'web-app.config.mjs')).href)) as { createWebAppConfig: ConfigFactory };
  return { createConfig: base.createConfig, createWebAppConfig: web.createWebAppConfig };
}

/** Lints the `src` tree of one fixture app with the same layers `apps/web` composes. */
async function lintApp(appRoot: string) {
  const { createConfig, createWebAppConfig } = await loadConfigs();
  const eslint = new ESLint({
    cwd: appRoot,
    overrideConfigFile: true,
    overrideConfig: [...createConfig(), ...createWebAppConfig({ appRoot, tsconfigRootDir: webFixtures })],
  });
  const results = await eslint.lintFiles(['src/**/*.{ts,tsx}']);
  return results.flatMap((result) => result.messages.map((message) => ({ ...message, file: result.filePath.replace(appRoot, '') })));
}

/** Lints a snippet as if it lived at `path` inside the fixture app, without type information. */
async function lintSnippet(code: string, path: string) {
  const { createConfig, createWebAppConfig } = await loadConfigs();
  const appRoot = join(webFixtures, 'good');
  const eslint = new ESLint({
    cwd: appRoot,
    overrideConfigFile: true,
    overrideConfig: [...createConfig(), ...createWebAppConfig({ appRoot, typed: false })],
  });
  const [result] = await eslint.lintText(code, { filePath: join(appRoot, path) });
  return (result?.messages ?? []).map((message) => message.ruleId);
}

interface PlantedCase {
  /** Folder under test/fixtures/web/bad; the source of truth for the case. */
  folder: string;
  rule: string;
  forbids: string;
}

const planted: PlantedCase[] = [
  { folder: 'cross-feature-import', rule: 'boundaries/element-types', forbids: 'a feature importing another feature' },
  { folder: 'components-import-forms', rule: 'boundaries/element-types', forbids: 'the components of a feature other than the editor importing its forms' },
  { folder: 'model-imports-react-component', rule: 'boundaries/element-types', forbids: 'model/ importing a React component' },
  { folder: 'utils-folder', rule: 'boundaries/no-unknown-files', forbids: 'a utils/ folder inside a feature' },
  { folder: 'barrel-index', rule: 'boundaries/no-unknown-files', forbids: 'a barrel index.ts inside a feature' },
  { folder: 'deep-import', rule: 'no-restricted-imports', forbids: 'a deep import of @orcon/ui/src' },
  { folder: 'fetch-in-screen', rule: 'no-restricted-globals', forbids: 'fetch in a screen' },
  { folder: 'window-fetch', rule: 'no-restricted-properties', forbids: 'window.fetch in a component' },
  { folder: 'sdk-in-screen', rule: 'boundaries/external', forbids: '@orcon/sdk imported by a screen' },
  { folder: 'zustand-outside-editor', rule: 'boundaries/external', forbids: 'zustand outside the editor store' },
  { folder: 'json-parse', rule: 'no-restricted-syntax', forbids: 'JSON.parse outside app/boundary' },
  { folder: 'local-storage', rule: 'no-restricted-properties', forbids: 'localStorage outside app/boundary' },
  { folder: 'explicit-any', rule: '@typescript-eslint/no-explicit-any', forbids: 'an explicit any' },
  { folder: 'non-null-assertion', rule: '@typescript-eslint/no-non-null-assertion', forbids: 'a non-null assertion' },
  { folder: 'ts-ignore', rule: '@typescript-eslint/ban-ts-comment', forbids: '@ts-ignore' },
  { folder: 'as-unknown-as', rule: 'no-restricted-syntax', forbids: 'a double assertion through unknown' },
  { folder: 'floating-promise', rule: '@typescript-eslint/no-floating-promises', forbids: 'a floating promise' },
  { folder: 'non-exhaustive-switch', rule: '@typescript-eslint/switch-exhaustiveness-check', forbids: 'a switch that misses a union member' },
  { folder: 'default-export', rule: 'no-restricted-exports', forbids: 'a default export' },
  { folder: 'query-key-missing-dependency', rule: '@tanstack/query/exhaustive-deps', forbids: 'a query key without a value the query function uses' },
];

describe('web rules: the planted cases are exactly the folders on disk', () => {
  it('has one table row per fixture folder and one fixture folder per row', () => {
    const onDisk = readdirSync(join(webFixtures, 'bad'), { withFileTypes: true })
      .filter((entry) => entry.isDirectory())
      .map((entry) => entry.name)
      .sort();
    expect(onDisk).toEqual(planted.map((entry) => entry.folder).sort());
  });

  it('plants the 19 violations of the lint-case table in checks.md and the one that bounds the editor exception', () => {
    expect(planted).toHaveLength(20);
  });
});

describe('web rules: every planted violation is detected', () => {
  it.each(planted)('rejects $forbids ($folder)', async ({ folder, rule }) => {
    const messages = await lintApp(join(webFixtures, 'bad', folder));
    expect(messages.filter((message) => message.ruleId === null)).toEqual([]);
    expect(messages.map((message) => message.ruleId)).toContain(rule);
  });
});

describe('web rules: the good tree', () => {
  it('has a complete feature area and produces no finding', async () => {
    expect(await lintApp(join(webFixtures, 'good'))).toEqual([]);
  });
});

describe('web rules: what each layer may do', () => {
  it('keeps zustand and immer for the editor store', async () => {
    expect(await lintSnippet("import { createStore } from 'zustand/vanilla';\nexport const store = createStore(() => ({ n: 1 }));\n", 'src/features/editor/store/editor-store.ts')).toEqual([]);
    expect(await lintSnippet("import { produce } from 'immer';\nexport const next = produce;\n", 'src/features/flows/model/next.ts')).toContain('boundaries/external');
  });

  it('keeps React and the UI package out of model/', async () => {
    expect(await lintSnippet("import { useState } from 'react';\nexport const hook = useState;\n", 'src/features/flows/model/hook.ts')).toContain('boundaries/external');
    expect(await lintSnippet("import { Button } from '@orcon/ui';\nexport const button = Button;\n", 'src/features/flows/model/button.ts')).toContain('boundaries/external');
  });

  it('lets the SDK be used by api/, live-events and errors only', async () => {
    const sdk = "import { sdk } from '@orcon/sdk';\nexport const client = sdk;\n";
    expect(await lintSnippet(sdk, 'src/features/flows/api/client.ts')).not.toContain('boundaries/external');
    expect(await lintSnippet(sdk, 'src/app/live-events/client.ts')).not.toContain('boundaries/external');
    expect(await lintSnippet(sdk, 'src/app/errors/client.ts')).not.toContain('boundaries/external');
    expect(await lintSnippet(sdk, 'src/app/forms/client.ts')).toContain('boundaries/external');
    expect(await lintSnippet(sdk, 'src/features/flows/forms/client.ts')).toContain('boundaries/external');
  });

  it('reads external input in app/boundary only', async () => {
    const parse = 'export const parsed = JSON.parse("1");\n';
    expect(await lintSnippet(parse, 'src/app/boundary/read.ts')).not.toContain('no-restricted-syntax');
    expect(await lintSnippet(parse, 'src/app/perf/read.ts')).toContain('no-restricted-syntax');
    const storage = 'export const saved = localStorage.getItem("k");\n';
    expect(await lintSnippet(storage, 'src/app/boundary/read.ts')).not.toContain('no-restricted-globals');
    expect(await lintSnippet(storage, 'src/app/perf/read.ts')).toContain('no-restricted-globals');
  });

  it('keeps network access out of app/boundary too', async () => {
    expect(await lintSnippet('export const load = () => fetch("/x");\n', 'src/app/boundary/read.ts')).toContain('no-restricted-globals');
  });

  it('allows console.error only in app/errors', async () => {
    const log = 'console.error("x");\n';
    expect(await lintSnippet(log, 'src/app/errors/report.ts')).not.toContain('no-console');
    expect(await lintSnippet(log, 'src/app/perf/report.ts')).toContain('no-console');
    expect(await lintSnippet('console.log("x");\n', 'src/app/errors/report.ts')).toContain('no-console');
  });

  it('lets tests read storage and call fetch', async () => {
    expect(await lintSnippet('export const saved = localStorage.getItem("k");\n', 'src/app/perf/read.test.ts')).toEqual([]);
  });

  it('lets a configuration file use a default export', async () => {
    expect(await lintSnippet('export default { a: 1 };\n', 'vite.config.ts')).toEqual([]);
  });

  it('applies only the TypeScript and network parts to a package (no app/ or features/ rules)', async () => {
    const { createConfig, createWebAppConfig } = await loadConfigs();
    const eslint = new ESLint({
      cwd: join(webFixtures, 'good'),
      overrideConfigFile: true,
      overrideConfig: [...createConfig(), ...createWebAppConfig({ tsconfigRootDir: webFixtures, typed: false })],
    });
    const [result] = await eslint.lintText('export const load = () => fetch("/x");\nexport const saved = localStorage.getItem("k");\n', {
      filePath: join(webFixtures, 'good', 'src/utils/helper.ts'),
    });
    const rules = (result?.messages ?? []).map((message) => message.ruleId);
    expect(rules).toContain('no-restricted-globals');
    expect(rules).not.toContain('boundaries/no-unknown-files');
  });
});

describe('check-tsconfig.mjs', () => {
  const script = join(packageDir, 'scripts', 'check-tsconfig.mjs');
  const base = join(packageDir, 'tsconfig.base.json');

  function workspace(tsconfigs: Record<string, object>): string {
    const root = mkdtempSync(join(tmpdir(), 'orcon-tsconfig-'));
    for (const [path, config] of Object.entries(tsconfigs)) {
      mkdirSync(join(root, path), { recursive: true });
      writeFileSync(join(root, path, 'tsconfig.json'), JSON.stringify(config));
      writeFileSync(join(root, path, 'a.ts'), 'export const a = 1;\n');
    }
    return root;
  }

  function run(root: string): { status: number; output: string } {
    try {
      const output = execFileSync(process.execPath, [script, '--root', root], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
      return { status: 0, output };
    } catch (error) {
      const failure = error as { status: number; stdout: string; stderr: string };
      return { status: failure.status, output: `${failure.stdout}${failure.stderr}` };
    }
  }

  it('accepts packages that extend the shared base', () => {
    const root = workspace({ 'apps/web': { extends: base, include: ['a.ts'] }, 'packages/js/ui': { extends: base, include: ['a.ts'] } });
    expect(run(root).status).toBe(0);
  });

  it('rejects a package that does not extend the base', () => {
    const root = workspace({ 'apps/web': { compilerOptions: { strict: true }, include: ['a.ts'] } });
    const { status, output } = run(root);
    expect(status).toBe(1);
    expect(output).toContain('exactOptionalPropertyTypes must be true');
  });

  it('rejects a package that turns a flag off locally', () => {
    const root = workspace({ 'packages/js/ui': { extends: base, compilerOptions: { noUncheckedIndexedAccess: false }, include: ['a.ts'] } });
    const { status, output } = run(root);
    expect(status).toBe(1);
    expect(output).toContain('noUncheckedIndexedAccess must be true');
  });
});
