// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { ESLint } from 'eslint';
import stylelint from 'stylelint';
import { describe, expect, it } from 'vitest';

const packageDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const fixtures = join(packageDir, 'test', 'fixtures');

function fixture(kind: 'bad' | 'good', name: string): string {
  return readFileSync(join(fixtures, kind, name), 'utf8');
}

const eslint = new ESLint({
  cwd: packageDir,
  overrideConfigFile: join(packageDir, 'eslint.config.mjs'),
});

async function lint(code: string, filePath: string) {
  const [result] = await eslint.lintText(code, { filePath: join(packageDir, filePath) });
  return result?.messages ?? [];
}

async function lintStyles(code: string, codeFilename: string) {
  const result = await stylelint.lint({
    code,
    codeFilename: join(packageDir, codeFilename),
    cwd: packageDir,
    configFile: join(packageDir, 'stylelint.config.mjs'),
  });
  const invalid = result.results.flatMap((r) => r.invalidOptionWarnings);
  if (invalid.length > 0) {
    throw new Error(`Invalid Stylelint configuration: ${invalid.map((w) => w.text).join('; ')}`);
  }
  return result.results.flatMap((r) => r.warnings);
}

interface BadCase {
  /** What the fixture proves is forbidden. */
  forbids: string;
  file: string;
  /** Virtual path the fixture is linted as, since some rules are path scoped. */
  path: string;
  rule: string;
}

const badCases: BadCase[] = [
  { forbids: 'hex literal in TSX', file: 'hex-literal.tsx', path: 'src/a/Hex.tsx', rule: 'orcon/no-color-literal' },
  { forbids: 'rgb literal in TSX', file: 'rgb-literal.tsx', path: 'src/a/Rgb.tsx', rule: 'orcon/no-color-literal' },
  { forbids: 'hsl literal in TSX', file: 'hsl-literal.tsx', path: 'src/a/Hsl.tsx', rule: 'orcon/no-color-literal' },
  { forbids: 'arbitrary Tailwind class', file: 'arbitrary-class.tsx', path: 'src/a/A.tsx', rule: 'better-tailwindcss/no-restricted-classes' },
  { forbids: 'default palette class', file: 'default-palette-class.tsx', path: 'src/a/A.tsx', rule: 'better-tailwindcss/no-restricted-classes' },
  { forbids: 'spacing off the scale', file: 'off-scale-spacing.tsx', path: 'src/a/A.tsx', rule: 'better-tailwindcss/no-restricted-classes' },
  { forbids: 'inline style without a reason', file: 'inline-style.tsx', path: 'src/canvas/A.tsx', rule: 'orcon/no-inline-style' },
  { forbids: 'eslint-disable without a description', file: 'disable-without-description.tsx', path: 'src/a/a.ts', rule: '@eslint-community/eslint-comments/require-description' },
  { forbids: 'literal JSX text', file: 'jsx-text.tsx', path: 'src/a/A.tsx', rule: 'i18next/no-literal-string' },
  { forbids: 'literal aria-label', file: 'aria-label-literal.tsx', path: 'src/a/A.tsx', rule: 'i18next/no-literal-string' },
  { forbids: 'literal title', file: 'title-literal.tsx', path: 'src/a/A.tsx', rule: 'i18next/no-literal-string' },
  { forbids: 'literal placeholder', file: 'placeholder-literal.tsx', path: 'src/a/A.tsx', rule: 'i18next/no-literal-string' },
  { forbids: 'literal alt', file: 'alt-literal.tsx', path: 'src/a/A.tsx', rule: 'i18next/no-literal-string' },
  { forbids: 'literal label prop', file: 'label-literal.tsx', path: 'src/a/A.tsx', rule: 'i18next/no-literal-string' },
  { forbids: 'invalid ICU message', file: 'invalid-icu.tsx', path: 'src/a/A.tsx', rule: 'formatjs/no-invalid-icu' },
  { forbids: 'message without a translator description', file: 'missing-description.tsx', path: 'src/a/A.tsx', rule: 'formatjs/enforce-description' },
  { forbids: 'Mascot import in the canvas', file: 'mascot-import.tsx', path: 'src/canvas/Node.tsx', rule: 'no-restricted-imports' },
  { forbids: 'Mascot import in execution views', file: 'mascot-import.tsx', path: 'src/execution/Detail.tsx', rule: 'no-restricted-imports' },
  { forbids: 'Mascot import in security views', file: 'mascot-import.tsx', path: 'src/security/Warning.tsx', rule: 'no-restricted-imports' },
  { forbids: 'Mascot import in error views', file: 'mascot-import.tsx', path: 'src/errors/ProblemAlert.tsx', rule: 'no-restricted-imports' },
  { forbids: 'inline SVG outside icons', file: 'inline-svg.tsx', path: 'src/canvas/ConnectorLogo.tsx', rule: 'no-restricted-syntax' },
  { forbids: 'SVG imported as a component', file: 'svg-react-import.tsx', path: 'src/canvas/ConnectorLogo.tsx', rule: 'no-restricted-imports' },
];

interface GoodCase {
  accepts: string;
  file: string;
  path: string;
}

const goodCases: GoodCase[] = [
  { accepts: 'token utilities, variants and arbitrary variants', file: 'token-classes.tsx', path: 'src/a/A.tsx' },
  { accepts: 'catalog messages with descriptions and valid ICU', file: 'messages.tsx', path: 'src/a/A.tsx' },
  { accepts: 'inline style with a stated reason', file: 'inline-style-with-reason.tsx', path: 'src/canvas/A.tsx' },
  { accepts: 'eslint-disable with a description', file: 'disable-with-description.tsx', path: 'src/a/a.ts' },
  { accepts: 'Mascot in a welcome screen', file: 'mascot-in-welcome.tsx', path: 'src/screens/Welcome.tsx' },
  { accepts: 'Mascot on the not found page', file: 'mascot-thinking.tsx', path: 'src/features/errors/NotFoundScreen.tsx' },
  { accepts: 'Mascot on the generic error page', file: 'mascot-thinking.tsx', path: 'src/screens/ErrorPageScreen.tsx' },
  { accepts: 'Mascot on the sign-in screen', file: 'mascot-in-welcome.tsx', path: 'src/features/session/screens/SignInScreen.tsx' },
  { accepts: 'connector logo through an img', file: 'connector-icon-img.tsx', path: 'src/canvas/ConnectorIcon.tsx' },
  { accepts: 'own inline SVG under icons', file: 'own-icon-svg.tsx', path: 'src/icons/Own.tsx' },
  { accepts: 'token variables and anchors', file: 'no-color-literals.tsx', path: 'src/a/a.ts' },
];

describe('ESLint: every forbidden pattern has a failing fixture', () => {
  it.each(badCases)('rejects $forbids ($file as $path)', async ({ file, path, rule }) => {
    const messages = await lint(fixture('bad', file), path);
    expect(messages.map((m) => m.ruleId)).toContain(rule);
  });
});

describe('ESLint: every good fixture passes', () => {
  it.each(goodCases)('accepts $accepts ($file as $path)', async ({ file, path }) => {
    const messages = await lint(fixture('good', file), path);
    expect(messages).toEqual([]);
  });
});

describe('ESLint with a Tailwind entry point', () => {
  const withTheme = new ESLint({
    cwd: packageDir,
    overrideConfigFile: join(fixtures, 'tailwind', 'eslint.config.mjs'),
  });

  async function lintWithTheme(code: string) {
    const [result] = await withTheme.lintText(code, { filePath: join(packageDir, 'src/a/A.tsx') });
    return result?.messages ?? [];
  }

  it('rejects a class that the token theme removed', async () => {
    const messages = await lintWithTheme(fixture('bad', 'unknown-class.tsx'));
    expect(messages.map((m) => m.ruleId)).toContain('better-tailwindcss/no-unknown-classes');
  });

  it('accepts classes the token theme registers', async () => {
    expect(await lintWithTheme(fixture('good', 'known-class.tsx'))).toEqual([]);
  });
});

describe('ESLint: rule scoping', () => {
  it('does not apply mascot boundaries outside the listed views', async () => {
    const messages = await lint(fixture('bad', 'mascot-import.tsx'), 'src/screens/Welcome.tsx');
    expect(messages).toEqual([]);
  });

  it('never reports a clean file', async () => {
    expect(await lint('export const answer = 42;\n', 'src/a/a.ts')).toEqual([]);
  });
});

const cssBadCases = [
  { forbids: 'hex color in CSS', file: 'hex.css', rule: 'color-no-hex' },
  { forbids: 'rgb function color in CSS', file: 'rgb.css', rule: 'function-disallowed-list' },
  { forbids: 'named color in CSS', file: 'named-color.css', rule: 'color-named' },
  { forbids: 'raw spacing in CSS', file: 'raw-spacing.css', rule: 'scale-unlimited/declaration-strict-value' },
  { forbids: 'raw radius in CSS', file: 'raw-radius.css', rule: 'scale-unlimited/declaration-strict-value' },
];

describe('Stylelint: every forbidden pattern has a failing fixture', () => {
  it.each(cssBadCases)('rejects $forbids ($file)', async ({ file, rule }) => {
    const warnings = await lintStyles(fixture('bad', file), 'src/a/style.css');
    expect(warnings.map((w) => w.rule)).toContain(rule);
  });

  it('accepts a stylesheet that uses tokens only', async () => {
    expect(await lintStyles(fixture('good', 'tokens.css'), 'src/a/style.css')).toEqual([]);
  });

  it('allows raw values inside packages/js/tokens', async () => {
    const warnings = await lintStyles(fixture('bad', 'hex.css'), 'packages/js/tokens/src/raw.css');
    expect(warnings).toEqual([]);
  });
});
