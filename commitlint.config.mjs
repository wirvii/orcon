// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
// Rules: .agents/skills/orcon-contributing/references/commits.md (CM-1..CM-12). ADR 0081.
// The scope list must equal the CM-3 table; `make commitlint-selftest` checks it.

const types = ['feat', 'fix', 'perf', 'refactor', 'docs', 'test', 'build', 'ci', 'chore', 'revert'];

const scopes = [
  // product
  'engine', 'trigger', 'connector', 'api', 'cli', 'mcp', 'web', 'ui', 'db', 'ai', 'security', 'notify',
  // internal
  'docs', 'ci', 'deps', 'tooling',
];

export default {
  extends: ['@commitlint/config-conventional'],
  parserPreset: {
    parserOpts: {
      // Exactly one lowercase scope; a comma or slash makes the header unparsable.
      headerPattern: /^(\w+)(?:\(([a-z]+)\))?(!)?: (.*)$/,
      headerCorrespondence: ['type', 'scope', 'breaking', 'subject'],
      // Same four groups as headerPattern: the parser applies headerCorrespondence to both.
      breakingHeaderPattern: /^(\w+)(?:\(([a-z]+)\))?(!): (.*)$/,
    },
  },
  rules: {
    'type-enum': [2, 'always', types],
    'scope-empty': [2, 'never'],
    'scope-enum': [2, 'always', scopes],
    'scope-case': [2, 'always', 'lower-case'],
    'subject-empty': [2, 'never'],
    'subject-case': [2, 'never', ['sentence-case', 'start-case', 'pascal-case', 'upper-case']],
    'subject-full-stop': [2, 'never', '.'],
    'header-max-length': [2, 'always', 72],
    'body-max-line-length': [1, 'always', 100],
    'footer-max-line-length': [1, 'always', 100],
  },
  helpUrl: 'https://github.com/wirvii/orcon/blob/main/CONTRIBUTING.md#commit-messages',
};
