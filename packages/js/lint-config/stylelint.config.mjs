// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Token lint for CSS. Only packages/js/tokens may hold raw color and size
// values; everywhere else a declaration uses var(--orcon-*).

// Stylelint options must be JSON-like, so regular expressions are strings.
const tokenValue = '/^var\\(--orcon-[a-z0-9-]+\\)$/';
const tokenCalc = '/^calc\\(.*var\\(--orcon-[a-z0-9-]+\\).*\\)$/';
const keywords = ['inherit', 'initial', 'unset', 'revert', 'none', 'auto', 'currentColor', 'transparent', '0'];

const ignoreValues = [tokenValue, tokenCalc, ...keywords];

/** @type {import('stylelint').Config} */
export default {
  plugins: ['stylelint-declaration-strict-value'],
  ignoreFiles: ['**/node_modules/**', '**/dist/**', '**/dist-perf/**', '**/storybook-static/**', '**/test/fixtures/**', 'packages/js/tokens/**'],
  rules: {
    'color-no-hex': true,
    'function-disallowed-list': ['rgb', 'rgba', 'hsl', 'hsla', 'hwb', 'lab', 'lch', 'oklab', 'oklch', 'color'],
    'color-named': 'never',
    'scale-unlimited/declaration-strict-value': [
      [
        '/color$/',
        'fill',
        'stroke',
        'box-shadow',
        'border-radius',
        '/^border-(top|bottom|start|end|left|right)-.*radius$/',
        '/^(margin|padding)(-.+)?$/',
        '/^(gap|row-gap|column-gap)$/',
        'font-size',
      ],
      {
        ignoreValues,
        ignoreFunctions: false,
        disableFix: true,
        message: 'Use a var(--orcon-*) token for "${property}" (found "${value}").',
      },
    ],
  },
};
