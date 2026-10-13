// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { describe, expect, it } from 'vitest';

interface Exception {
  license: string;
  reason: string;
  date: string;
  approvedBy: string;
}

interface Pkg {
  name: string;
  version: string;
  license: string;
  source: string;
}

interface Checker {
  allowedLicenses: Set<string>;
  isAllowedTerm: (term: string) => boolean;
  isNeverAllowed: (license: string) => boolean;
  expressionAllowed: (expression: string, ok: (term: string) => boolean) => boolean;
  parseExceptions: (text: string) => { entries: Map<string, Exception>; problems: string[] };
  evaluate: (packages: Pkg[], exceptions: Map<string, Exception>) => { problems: string[]; excepted: string[] };
  shippedWorkspacePackages: (root: string, start?: string) => string[];
}

const script = join(dirname(fileURLToPath(import.meta.url)), '..', 'scripts', 'check-licenses.mjs');
const checker = (await import(pathToFileURL(script).href)) as Checker;

const allowed = (term: string) => checker.isAllowedTerm(term);
const line = (key: string, license: string, date = '2026-10-12', approvedBy = 'approved-by:owner') => `${key}  ${license}  because it is needed  ${date}  ${approvedBy}`;
const pkg = (name: string, version: string, license: string, source = 'production closure'): Pkg => ({ name, version, license, source });

describe('the allowed list', () => {
  it('is Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause and ISC', () => {
    expect([...checker.allowedLicenses].sort()).toEqual(['Apache-2.0', 'BSD-2-Clause', 'BSD-3-Clause', 'ISC', 'MIT']);
  });

  it.each(['GPL-3.0-only', 'AGPL-3.0', 'LGPL-2.1', 'SSPL-1.0', 'BUSL-1.1', 'Elastic-2.0', 'Commons Clause', 'PolyForm-Noncommercial-1.0.0', 'SEE LICENSE IN LICENSE.md', 'UNLICENSED', 'Proprietary'])(
    'never lets an exception approve %s',
    (license) => {
      expect(checker.isNeverAllowed(license)).toBe(true);
    },
  );

  it.each(['MIT', 'Apache-2.0', 'OFL-1.1', '0BSD', 'Unlicense', 'CC0-1.0', 'Python-2.0'])('leaves %s to the allowed list and the exceptions', (license) => {
    expect(checker.isNeverAllowed(license)).toBe(false);
  });
});

describe('SPDX expressions', () => {
  it.each([
    ['MIT', true],
    ['Apache-2.0', true],
    ['(MIT OR Apache-2.0)', true],
    ['(MIT OR GPL-3.0-only)', true],
    ['GPL-3.0-only OR MIT', true],
    ['(BSD-2-Clause OR MIT OR Apache-2.0)', true],
    ['MIT AND ISC', true],
    ['MIT AND GPL-3.0-only', false],
    ['(MIT OR ISC) AND Apache-2.0', true],
    ['(GPL-3.0-only OR AGPL-3.0) AND MIT', false],
    ['mit or isc', true],
    ['OFL-1.1', false],
    ['Unknown', false],
    ['', false],
    ['MIT OR', false],
    ['(MIT', false],
    ['MIT ISC', false],
  ])('%s is allowed: %s', (expression, expected) => {
    expect(checker.expressionAllowed(expression, allowed)).toBe(expected);
  });
});

describe('the exceptions file', () => {
  it('reads a line with its package, version, license, reason, date and approver', () => {
    const { entries, problems } = checker.parseExceptions(`# SPDX header\n\n${line('tslib@2.8.1', '0BSD')}\n`);
    expect(problems).toEqual([]);
    expect(entries.get('tslib@2.8.1')).toEqual({ license: '0BSD', reason: 'because it is needed', date: '2026-10-12', approvedBy: 'owner' });
  });

  it('keeps scoped package names whole', () => {
    expect(checker.parseExceptions(line('@fontsource-variable/inter@5.3.0', 'OFL-1.1')).entries.has('@fontsource-variable/inter@5.3.0')).toBe(true);
  });

  it('reports a malformed line', () => {
    const { problems } = checker.parseExceptions('tslib 0BSD runtime helper');
    expect(problems).toHaveLength(1);
    expect(problems[0]).toContain('line 1 is malformed');
  });

  it('rejects a line without approved-by', () => {
    const { entries, problems } = checker.parseExceptions('tslib@2.8.1  0BSD  runtime helper  2026-10-12');
    expect(entries.size).toBe(0);
    expect(problems).toHaveLength(1);
  });

  it('rejects an invalid date and a repeated package', () => {
    expect(checker.parseExceptions(line('tslib@2.8.1', '0BSD', '2026-13-45')).problems[0]).toContain('invalid date');
    expect(checker.parseExceptions(`${line('tslib@2.8.1', '0BSD')}\n${line('tslib@2.8.1', '0BSD')}`).problems[0]).toContain('repeats tslib@2.8.1');
  });
});

describe('evaluating packages', () => {
  const exceptions = (text: string) => checker.parseExceptions(text).entries;

  it('passes allowed licenses and compound expressions', () => {
    const { problems } = checker.evaluate([pkg('react', '19.0.0', 'MIT'), pkg('dual', '1.0.0', '(MIT OR GPL-3.0-only)'), pkg('both', '1.0.0', 'ISC')], new Map());
    expect(problems).toEqual([]);
  });

  it('fails a license outside the list that has no exception, naming the package and where it was found', () => {
    const { problems } = checker.evaluate([pkg('font', '1.0.0', 'OFL-1.1', 'bundled')], new Map());
    expect(problems).toHaveLength(1);
    expect(problems[0]).toContain('font@1.0.0 (OFL-1.1) [bundled]');
    expect(problems[0]).toContain('no line in tools/licenses/npm-exceptions.txt');
  });

  it('passes it when its exact name and version have a line', () => {
    const result = checker.evaluate([pkg('tslib', '2.8.1', '0BSD')], exceptions(line('tslib@2.8.1', '0BSD')));
    expect(result.problems).toEqual([]);
    expect(result.excepted).toEqual(['tslib@2.8.1']);
  });

  it('fails again when the package is upgraded, until the line is renewed', () => {
    const { problems } = checker.evaluate([pkg('tslib', '2.9.0', '0BSD')], exceptions(line('tslib@2.8.1', '0BSD')));
    expect(problems).toHaveLength(1);
    expect(problems[0]).toContain('tslib@2.9.0');
  });

  it('fails when the license of the package is not the one the line approved', () => {
    const { problems } = checker.evaluate([pkg('tslib', '2.8.1', 'Unlicense')], exceptions(line('tslib@2.8.1', '0BSD')));
    expect(problems[0]).toContain('the exception line says 0BSD, the package says Unlicense');
  });

  it('fails a license no exception can approve, even with a line', () => {
    const { problems } = checker.evaluate([pkg('copyleft', '1.0.0', 'GPL-3.0-only')], exceptions(line('copyleft@1.0.0', 'GPL-3.0-only')));
    expect(problems).toHaveLength(1);
    expect(problems[0]).toContain('no exception can approve');
  });

  it('fails a package that declares no license', () => {
    const { problems } = checker.evaluate([pkg('anon', '1.0.0', '')], new Map());
    expect(problems[0]).toContain('declares no license');
  });

  it('reports a package once per source, however many times it is listed', () => {
    const list = [pkg('font', '1.0.0', 'OFL-1.1'), pkg('font', '1.0.0', 'OFL-1.1'), pkg('font', '1.0.0', 'OFL-1.1', 'bundled')];
    expect(checker.evaluate(list, new Map()).problems).toHaveLength(2);
  });
});

describe('which workspace packages ship', () => {
  function workspace(manifests: Record<string, object>): string {
    const root = mkdtempSync(join(tmpdir(), 'orcon-licenses-'));
    for (const [dir, manifest] of Object.entries(manifests)) {
      mkdirSync(join(root, dir), { recursive: true });
      writeFileSync(join(root, dir, 'package.json'), JSON.stringify(manifest));
    }
    return root;
  }

  it('follows workspace dependencies, not development dependencies', () => {
    const root = workspace({
      'apps/web': { name: '@orcon/web', dependencies: { '@orcon/ui': 'workspace:*', react: '^19' }, devDependencies: { '@orcon/lint-config': 'workspace:*' } },
      'packages/js/ui': { name: '@orcon/ui', dependencies: { '@orcon/tokens': 'workspace:*' } },
      'packages/js/tokens': { name: '@orcon/tokens' },
      'packages/js/lint-config': { name: '@orcon/lint-config', dependencies: { eslint: '^9' } },
    });
    expect(checker.shippedWorkspacePackages(root).sort()).toEqual(['@orcon/tokens', '@orcon/ui', '@orcon/web']);
  });

  it('stops at a workspace cycle and at a package that does not exist', () => {
    const root = workspace({
      'apps/web': { name: '@orcon/web', dependencies: { '@orcon/a': 'workspace:*', '@orcon/missing': 'workspace:*' } },
      'packages/js/a': { name: '@orcon/a', dependencies: { '@orcon/web': 'workspace:*' } },
    });
    expect(checker.shippedWorkspacePackages(root).sort()).toEqual(['@orcon/a', '@orcon/web']);
  });
});
