// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

import comments from '@eslint-community/eslint-plugin-eslint-comments';
import js from '@eslint/js';
import { defineConfig } from 'eslint/config';
import betterTailwind from 'eslint-plugin-better-tailwindcss';
import formatjs from 'eslint-plugin-formatjs';
import i18next from 'eslint-plugin-i18next';
import globals from 'globals';
import tseslint from 'typescript-eslint';

import { restrictedClasses } from './rules/class-patterns.mjs';
import orcon from './rules/orcon-plugin.mjs';

// The Tailwind plugin resolves `tailwindcss` from this directory, where it is a
// dependency, so consumers do not each need their own copy just to lint.
const here = dirname(fileURLToPath(import.meta.url));

const code = ['**/*.{ts,tsx}'];
const tsx = ['**/*.tsx'];
const testsAndStories = ['**/*.test.{ts,tsx}', '**/test/**', '**/*.stories.tsx'];

// Views where the mascot never appears: canvas, execution detail, errors,
// security warnings and dense views (inbox, tables).
export const mascotFreeViews = [
  '**/canvas/**',
  '**/{execution,executions,errors,error,security,inbox,data-table}/**',
  '**/screens/**/*{Execution,Inbox,ErrorPage,Security,Approval}*.{ts,tsx}',
];

export const mascotImport = {
  group: ['**/Mascot', '**/mascot', '**/mascot/**', '@orcon/ui/mascot'],
  message:
    'The mascot is not allowed on canvas, execution, error, security or dense views. Use EmptyState in an allowed context.',
};
export const mascotName = {
  group: ['**'],
  importNamePattern: '^Mascot$',
  message: mascotImport.message,
};
export const svgImports = {
  group: ['**/*.svg?react', '**/*.svg?raw', '**/*.svg?inline'],
  message:
    'Connector logos render only through ConnectorIcon (an <img>). Own icons live under icons/.',
};

export const inlineSvg = {
  selector: 'JSXOpeningElement[name.name="svg"]',
  message:
    'Inline <svg> is not allowed. Connector logos render only through ConnectorIcon (an <img>); own icons live under icons/.',
};

/**
 * Builds the shared Orcon ESLint configuration.
 *
 * @param {object} [options]
 * @param {string} [options.tailwindEntryPoint] Absolute path of the CSS entry that imports Tailwind and
 *   the token theme. When set, classes that Tailwind does not know (for example
 *   a removed default palette class) are reported as well.
 * @param {string} [options.cwd] Directory used to resolve Tailwind. Defaults to this package.
 */
export function createConfig({ tailwindEntryPoint, cwd } = {}) {
  return defineConfig([
    {
      ignores: [
        '**/node_modules/**',
        '**/dist/**',
        '**/dist-perf/**',
        '**/storybook-static/**',
        '**/coverage/**',
        '**/test/fixtures/**',
        '**/.turbo/**',
      ],
    },
    js.configs.recommended,
    tseslint.configs.recommended,
    {
      languageOptions: { globals: { ...globals.browser, ...globals.node } },
      linterOptions: { reportUnusedDisableDirectives: 'error' },
      plugins: { '@eslint-community/eslint-comments': comments },
      rules: {
        '@eslint-community/eslint-comments/require-description': 'error',
        '@eslint-community/eslint-comments/no-unlimited-disable': 'error',
        '@eslint-community/eslint-comments/disable-enable-pair': 'error',
      },
    },
    {
      files: code,
      plugins: { orcon, 'better-tailwindcss': betterTailwind },
      settings: {
        'better-tailwindcss': {
          ...(tailwindEntryPoint ? { entryPoint: tailwindEntryPoint } : {}),
          cwd: cwd ?? here,
        },
      },
      rules: {
        'orcon/no-color-literal': 'error',
        'orcon/no-inline-style': 'error',
        'better-tailwindcss/no-restricted-classes': ['error', { restrict: restrictedClasses }],
        ...(tailwindEntryPoint ? { 'better-tailwindcss/no-unknown-classes': 'error' } : {}),
        'no-restricted-syntax': ['error', inlineSvg],
        'no-restricted-imports': ['error', { patterns: [svgImports] }],
      },
    },
    {
      files: ['**/icons/**/*.{ts,tsx}'],
      rules: { 'no-restricted-syntax': 'off', 'no-restricted-imports': 'off' },
    },
    {
      files: mascotFreeViews,
      rules: {
        'no-restricted-imports': ['error', { patterns: [mascotImport, mascotName, svgImports] }],
      },
    },
    {
      files: tsx,
      plugins: { i18next, formatjs },
      rules: {
        'i18next/no-literal-string': [
          'error',
          {
            mode: 'jsx-only',
            'jsx-attributes': {
              include: ['aria-label', 'aria-description', 'title', 'placeholder', 'alt', 'label'],
              exclude: [],
            },
            // Replaces the plugin default list, so the defaults are repeated.
            callees: {
              exclude: [
                'i18n(ext)?',
                't',
                'require',
                'addEventListener',
                'removeEventListener',
                'postMessage',
                'getElementById',
                'dispatch',
                'commit',
                'includes',
                'indexOf',
                'endsWith',
                'startsWith',
                '(?:.*\\.)?formatMessage',
                'defineMessages?',
              ],
            },
          },
        ],
        'formatjs/no-invalid-icu': 'error',
        'formatjs/enforce-placeholders': 'error',
        'formatjs/enforce-description': ['error', 'literal'],
      },
    },
    {
      files: code,
      plugins: { formatjs },
      rules: {
        'formatjs/no-invalid-icu': 'error',
        'formatjs/enforce-placeholders': 'error',
      },
    },
    {
      // Tests and stories assert on literals and build fixtures; they still
      // follow the color, class and comment rules above.
      files: testsAndStories,
      rules: { 'i18next/no-literal-string': 'off' },
    },
  ]);
}

export default createConfig();
