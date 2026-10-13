// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Lint rules for the web application code (orcon-web): feature folders and
// import boundaries, network access only through the SDK, external input at
// one boundary, strict TypeScript, and the TanStack Query/Router and hooks
// rules. Compose it after `createConfig` (the orcon-ui layer):
//
//   export default [...createConfig({ tailwindEntryPoint }), ...createWebAppConfig({ appRoot })];
//
// Packages under packages/js call it without `appRoot` and get the TypeScript
// and network parts only (no folder rules).

import pluginQuery from '@tanstack/eslint-plugin-query';
import pluginRouter from '@tanstack/eslint-plugin-router';
import boundaries from 'eslint-plugin-boundaries';
import reactHooks from 'eslint-plugin-react-hooks';
import { defineConfig } from 'eslint/config';
import tseslint from 'typescript-eslint';

import { inlineSvg, mascotFreeViews, mascotImport, mascotName, svgImports } from './eslint.config.mjs';

const code = ['**/*.{ts,tsx}'];

// Files that exercise or configure the app instead of being part of it.
const toolingFiles = [
  '**/*.{test,spec}.{ts,tsx}',
  '**/*.stories.tsx',
  'test/**',
  'e2e/**',
  'perf/**',
  'visual/**',
  'scripts/**',
  '.storybook/**',
  '**/*.config.{ts,mjs,js}',
];

const configFiles = ['**/*.config.{ts,mjs,js}', '.storybook/**', '**/*.stories.tsx'];

// ---------------------------------------------------------------------------
// Restricted constructs

const networkGlobals = ['fetch', 'XMLHttpRequest', 'EventSource', 'WebSocket'].map((name) => ({
  name,
  message: 'Only @orcon/sdk performs HTTP or opens event streams (orcon-web DT-1). Call the SDK from the feature api/ layer.',
}));

const networkProperties = [
  { object: 'window', property: 'fetch' },
  { object: 'globalThis', property: 'fetch' },
  { object: 'self', property: 'fetch' },
  { object: 'navigator', property: 'sendBeacon' },
].map((entry) => ({ ...entry, message: 'Only @orcon/sdk performs HTTP (orcon-web DT-1). Call the SDK from the feature api/ layer.' }));

const inputGlobals = ['localStorage', 'sessionStorage'].map((name) => ({
  name,
  message: 'External input is read only in app/boundary and parsed with Zod (orcon-web TS-3). Use the boundary storage helper.',
}));

const inputProperties = [
  { object: 'window', property: 'localStorage' },
  { object: 'window', property: 'sessionStorage' },
  { object: 'globalThis', property: 'localStorage' },
  { object: 'globalThis', property: 'sessionStorage' },
  { object: 'navigator', property: 'clipboard' },
].map((entry) => ({ ...entry, message: 'External input is read only in app/boundary and parsed with Zod (orcon-web TS-3). Use the boundary helper.' }));

const jsonParse = {
  selector: "CallExpression[callee.object.name='JSON'][callee.property.name='parse']",
  message: 'JSON.parse results are external input: parse them with Zod in app/boundary (orcon-web TS-3).',
};

const messageListener = {
  selector: "CallExpression[callee.property.name='addEventListener'][arguments.0.value='message']",
  message: 'postMessage payloads are external input: listen and parse them in app/boundary (orcon-web TS-3).',
};

const doubleAssertion = {
  selector: "TSAsExpression[expression.type='TSAsExpression'][expression.typeAnnotation.type='TSUnknownKeyword']",
  message: '`as unknown as` hides a type error. Parse the value with Zod or fix the type (orcon-web TS-2).',
};

const barrelFile = {
  selector: 'Program',
  message: 'Barrel index files are not allowed in features: they defeat code splitting. Import the concrete file (orcon-web AR-7).',
};

const deepImport = {
  group: ['@orcon/*/src', '@orcon/*/src/**'],
  message: 'Import a package through its exports map, not its source files (orcon-web AR-10).',
};

// ---------------------------------------------------------------------------
// Folder structure (eslint-plugin-boundaries)

const layers = ['routes', 'screens', 'api', 'forms', 'components', 'model'];

/** Element descriptors: one per feature layer plus the app composition modules. */
function elements() {
  return [
    ...layers.map((layer) => ({
      type: `feature-${layer}`,
      pattern: `src/features/*/${layer}`,
      partialMatch: false,
      capture: ['area'],
    })),
    { type: 'feature-store', pattern: 'src/features/editor/store', partialMatch: false },
    { type: 'app-module', pattern: 'src/app/*', partialMatch: false, capture: ['module'] },
    { type: 'app-root', pattern: 'src/app', partialMatch: false },
  ];
}

const sameArea = { area: '${from.area}' };

/** What each layer may import (orcon-web AR-5). Imports inside one element are always fine. */
function elementTypeRules() {
  const app = ['app-module', 'app-root'];
  return [
    { from: 'feature-routes', allow: [['feature-routes', sameArea], ['feature-screens', sameArea], ['feature-api', sameArea], ['feature-model', sameArea], ...app] },
    { from: 'feature-screens', allow: [['feature-screens', sameArea], ['feature-forms', sameArea], ['feature-components', sameArea], ['feature-api', sameArea], ['feature-model', sameArea], ...app] },
    { from: 'feature-forms', allow: [['feature-forms', sameArea], ['feature-components', sameArea], ['feature-api', sameArea], ['feature-model', sameArea], ...app] },
    { from: 'feature-components', allow: [['feature-components', sameArea], ['feature-model', sameArea], ...app] },
    { from: 'feature-api', allow: [['feature-api', sameArea], ['feature-model', sameArea], ...app] },
    { from: 'feature-model', allow: [['feature-model', sameArea]] },
    { from: 'feature-store', allow: ['feature-store', ['feature-model', { area: 'editor' }]] },
    // The app composes features only through their route files (the generated route tree).
    { from: 'app-module', allow: ['app-module', 'app-root'] },
    { from: 'app-root', allow: ['app-module', 'app-root', 'feature-routes'] },
  ];
}

const allFeatureTypes = [...layers.map((layer) => `feature-${layer}`), 'feature-store'];

/** Packages only some layers may import (orcon-web DT-2, ST-2, AR-5). */
function externalRules() {
  const sdkAllowed = new Set(['feature-api']);
  return [
    {
      from: [...allFeatureTypes, 'app-root'].filter((type) => !sdkAllowed.has(type)),
      disallow: ['@orcon/sdk', '@orcon/sdk/*'],
      message: '@orcon/sdk is called only from features/<area>/api, app/live-events and app/errors (orcon-web DT-2).',
    },
    {
      from: [['app-module', { module: '!(live-events|errors)' }]],
      disallow: ['@orcon/sdk', '@orcon/sdk/*'],
      message: '@orcon/sdk is called only from features/<area>/api, app/live-events and app/errors (orcon-web DT-2).',
    },
    {
      from: [...allFeatureTypes.filter((type) => type !== 'feature-store'), 'app-root', 'app-module'],
      disallow: ['zustand', 'zustand/*', 'immer'],
      message: 'zustand and immer live only in features/editor/store (orcon-web ST-2).',
    },
    {
      from: ['feature-model'],
      disallow: ['react', 'react/*', 'react-dom', 'react-dom/*', '@orcon/ui', '@orcon/ui/*'],
      message: 'model/ is plain TypeScript: no React and no UI package (orcon-web AR-3).',
    },
  ];
}

// ---------------------------------------------------------------------------
// TypeScript rules

const typescriptRules = {
  '@typescript-eslint/no-explicit-any': 'error',
  '@typescript-eslint/no-non-null-assertion': 'error',
  '@typescript-eslint/ban-ts-comment': [
    'error',
    {
      'ts-ignore': true,
      'ts-nocheck': true,
      'ts-check': false,
      'ts-expect-error': 'allow-with-description',
      minimumDescriptionLength: 10,
    },
  ],
  '@typescript-eslint/consistent-type-imports': ['error', { prefer: 'type-imports', fixStyle: 'inline-type-imports' }],
  '@typescript-eslint/consistent-type-assertions': ['error', { assertionStyle: 'as', objectLiteralTypeAssertions: 'never' }],
  '@typescript-eslint/no-namespace': 'error',
  // Its advice (use `!`) contradicts no-non-null-assertion.
  '@typescript-eslint/non-nullable-type-assertion-style': 'off',
};

/** Rules that need type information (the type-aware mode). */
const typedRules = {
  '@typescript-eslint/no-floating-promises': 'error',
  // A `default` branch is a deliberate catch-all for the remaining members of a union.
  '@typescript-eslint/switch-exhaustiveness-check': ['error', { considerDefaultExhaustiveForUnions: true }],
  // React handlers are written as `onClick={() => close()}`; the arrow shorthand is clear there.
  '@typescript-eslint/no-confusing-void-expression': ['error', { ignoreArrowShorthand: true }],
  // The SDK reports a failed request as a problem object, not an Error: it is thrown as `unknown` and read by toProblemDetails.
  '@typescript-eslint/only-throw-error': ['error', { allowThrowingUnknown: true }],
  // Numbers in template strings are safe and common in class names and labels.
  '@typescript-eslint/restrict-template-expressions': ['error', { allowNumber: true }],
};

/**
 * Builds the web application ESLint configuration.
 *
 * @param {object} [options]
 * @param {string} [options.appRoot] Absolute path of the app (the folder that holds `src/`). When set, the folder
 *   structure, SDK, store and external input rules apply to it. Without it only the TypeScript, network and
 *   hooks parts apply (packages under packages/js).
 * @param {string} [options.tsconfigRootDir] Folder the type-aware parser resolves tsconfig.json from. Defaults to `appRoot`.
 * @param {string[]} [options.untypedFiles] Files outside every tsconfig (tool configuration) that are linted without type information.
 * @param {boolean} [options.network] Apply the "only the SDK talks to the server" rules. Off for the SDK package itself.
 * @param {boolean} [options.typed] Use type-aware rules (needs a tsconfig). Defaults to true.
 */
export function createWebAppConfig({ appRoot, tsconfigRootDir = appRoot, network = true, typed = true, untypedFiles = [] } = {}) {
  const app = appRoot !== undefined;
  if (typed && tsconfigRootDir === undefined) {
    throw new Error('createWebAppConfig: pass appRoot or tsconfigRootDir so the type-aware parser knows where tsconfig.json is.');
  }
  const typeChecked = typed
    ? defineConfig({
        files: code,
        extends: [tseslint.configs.strictTypeChecked, tseslint.configs.stylisticTypeChecked],
        languageOptions: {
          parserOptions: { projectService: true, ...(tsconfigRootDir === undefined ? {} : { tsconfigRootDir }) },
        },
      })
    : defineConfig({ files: code, extends: [tseslint.configs.strict, tseslint.configs.stylistic] });

  // no-restricted-syntax replaces the orcon-ui list, so each variant repeats inlineSvg.
  const syntaxFor = (...selectors) => ['error', ...selectors];
  const importPatterns = (...extra) => ['error', { patterns: [...extra, deepImport] }];

  const config = [
    // Generated files (the route tree, the contract code) are type-checked but not linted (orcon-web TS-8).
    { ignores: ['**/*.gen.ts', '**/gen/**'] },
    // Every file the TypeScript parser reads (including .mjs configs) needs the root, or it guesses.
    ...(tsconfigRootDir === undefined ? [] : [{ languageOptions: { parserOptions: { tsconfigRootDir } } }]),
    ...typeChecked,
    {
      files: code,
      plugins: { '@tanstack/query': pluginQuery, '@tanstack/router': pluginRouter, 'react-hooks': reactHooks },
      rules: {
        ...typescriptRules,
        ...(typed ? typedRules : {}),
        '@tanstack/query/exhaustive-deps': 'error',
        '@tanstack/query/stable-query-client': 'error',
        '@tanstack/router/create-route-property-order': 'error',
        'react-hooks/rules-of-hooks': 'error',
        'react-hooks/exhaustive-deps': 'error',
        'no-restricted-syntax': syntaxFor(inlineSvg, doubleAssertion, ...(app ? [jsonParse, messageListener] : [])),
        'no-restricted-imports': importPatterns(svgImports),
        ...(network
          ? {
              'no-restricted-globals': ['error', ...networkGlobals, ...(app ? inputGlobals : [])],
              'no-restricted-properties': ['error', ...networkProperties, ...(app ? inputProperties : [])],
            }
          : {}),
        ...(app ? { 'no-console': 'error' } : {}),
      },
    },
    {
      // The mascot is banned from some views (orcon-ui); keep that list next to the deep import rule.
      files: mascotFreeViews,
      rules: { 'no-restricted-imports': importPatterns(mascotImport, mascotName, svgImports) },
    },
    {
      files: ['**/icons/**/*.{ts,tsx}'],
      rules: {
        'no-restricted-syntax': syntaxFor(doubleAssertion, ...(app ? [jsonParse, messageListener] : [])),
        'no-restricted-imports': importPatterns(),
      },
    },
  ];

  if (app) {
    config.push(
      {
        files: ['src/**/*.{ts,tsx}'],
        plugins: { boundaries },
        settings: {
          'import/resolver': { node: { extensions: ['.ts', '.tsx', '.js', '.jsx', '.mjs', '.json'] } },
          'boundaries/root-path': appRoot,
          'boundaries/include': ['src/**/*.{ts,tsx}'],
          // Entry, generated, ambient and catalog files are not part of the folder structure.
          'boundaries/ignore': ['src/main.tsx', 'src/**/*.d.ts', 'src/**/*.gen.ts', 'src/i18n/**'],
          'boundaries/elements': elements(),
          'boundaries/dependency-nodes': ['import', 'dynamic-import', 'export'],
        },
        rules: {
          'boundaries/no-unknown-files': 'error',
          'boundaries/element-types': ['error', { default: 'disallow', rules: elementTypeRules() }],
          'boundaries/external': ['error', { default: 'allow', rules: externalRules() }],
        },
      },
      {
        // Named exports only (route files export `Route`); configuration files are exempt.
        files: ['src/**/*.{ts,tsx}'],
        ignores: configFiles,
        rules: {
          'no-restricted-exports': [
            'error',
            { restrictDefaultExports: { direct: true, named: true, defaultFrom: true, namedFrom: true, namespaceFrom: false } },
          ],
        },
      },
      {
        files: ['src/features/**/index.{ts,tsx}'],
        rules: {
          'no-restricted-syntax': syntaxFor(inlineSvg, doubleAssertion, jsonParse, messageListener, barrelFile),
        },
      },
      {
        // External input is read and parsed in one place.
        files: ['src/app/boundary/**/*.{ts,tsx}'],
        rules: {
          'no-restricted-syntax': syntaxFor(inlineSvg, doubleAssertion),
          'no-restricted-globals': ['error', ...(network ? networkGlobals : [])],
          'no-restricted-properties': ['error', ...(network ? networkProperties : [])],
        },
      },
      {
        files: ['src/app/errors/**/*.{ts,tsx}'],
        rules: { 'no-console': ['error', { allow: ['error'] }] },
      },
    );
  }

  config.push({
    // Tests, stories and tooling read storage, call fetch for harnesses and log freely.
    files: toolingFiles,
    rules: {
      'no-restricted-globals': 'off',
      'no-restricted-properties': 'off',
      'no-restricted-syntax': syntaxFor(inlineSvg, doubleAssertion),
      'no-console': 'off',
      'no-restricted-exports': 'off',
    },
  });

  // Last, so nothing above turns a type-aware rule back on for these files.
  if (untypedFiles.length > 0) {
    config.push(
      ...defineConfig({ files: untypedFiles, extends: [tseslint.configs.disableTypeChecked], languageOptions: { parserOptions: { projectService: false } } }),
    );
  }

  return config;
}
