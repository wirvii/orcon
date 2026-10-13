// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Writes the API changelog of a release (R-CHG-01, spec section 4.8):
 * header + `oasdiff changelog` of the stable and beta API + table of deprecations + the
 * separate section of experimental operations.
 *
 *   node scripts/changelog.mjs --base <bundle> --head <bundle> --version vX.Y.Z --from <label>
 *        --output <file> [--oasdiff "<command>"] [--oasdiff-cwd <dir>]
 *
 * `--from` is the label of the base in the text (the tag it was bundled from).
 * `--check` compares the generated text with --output instead of writing it (the release job
 * regenerates the changelog and fails if the committed one differs).
 */

import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

import { deprecations, experimental, renderDeprecations, renderExperimental } from './changelog-deprecations.mjs';
import { generatedHeader, must, readDocument, writeText, writeYaml } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';
import { splitCommand } from './compat-selftest.mjs';
import { withoutExperimental } from './stability-filter.mjs';

/** The body of `oasdiff changelog --format markdown` without its title and its "Info" section. */
export function oasdiffBody(markdown) {
  const lines = markdown.replace(/\r\n/g, '\n').split('\n');
  const start = lines.findIndex((l) => l.startsWith('## '));
  if (start < 0) return '';
  let text = lines.slice(start).join('\n');
  const info = text.indexOf('\n## Info');
  if (info >= 0) text = text.slice(0, info);
  text = text.replace(/^## API Changes\n/, '').replace(/\n{3,}/g, '\n\n').trim();
  return text;
}

/** Runs oasdiff changelog on two (already filtered) documents; returns its Markdown. */
export function oasdiffChangelog(command, baseFile, headFile, cwd) {
  const [program, ...prefix] = splitCommand(command);
  const result = spawnSync(program, [...prefix, 'changelog', '--format', 'markdown', baseFile, headFile], { encoding: 'utf8', cwd, env: { ...process.env, GOWORK: 'off' } });
  if (result.error) throw new Error(`cannot run ${command}: ${result.error.message}`);
  if (result.status !== 0) throw new Error(`oasdiff changelog failed:\n${result.stdout}${result.stderr}`);
  return result.stdout;
}

/** Composes the changelog text for two bundled documents. */
export function composeChangelog({ base, head, version, from, command, cwd }) {
  const dir = mkdtempSync(join(tmpdir(), 'orcon-changelog-'));
  try {
    const baseFile = join(dir, 'base.yaml');
    const headFile = join(dir, 'head.yaml');
    writeYaml(baseFile, withoutExperimental(base));
    writeYaml(headFile, withoutExperimental(head));
    const changes = oasdiffBody(oasdiffChangelog(command, baseFile, headFile, cwd));
    return [
      generatedHeader('scripts/changelog.mjs of @orcon/openapi-tools (make api-changelog)'),
      `# API changelog ${version}`,
      '',
      `Changes to the stable and beta API since ${from}. Experimental operations are outside the compatibility guarantee and are listed separately below.`,
      '',
      '## Changes',
      '',
      changes === '' ? 'No changes to the stable and beta API.' : changes,
      '',
      '## Deprecations',
      '',
      renderDeprecations(deprecations(head)),
      '## Experimental',
      '',
      renderExperimental(experimental(head, base)),
    ].join('\n');
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2), ['check']);
  must(flags.base && flags.head && flags.version && flags.from && flags.output, 'usage: changelog.mjs --base <bundle> --head <bundle> --version vX.Y.Z --from <label> --output <file> [--oasdiff "<command>"] [--oasdiff-cwd <dir>] [--check]');
  const command = flags.oasdiff ?? process.env.ORCON_OASDIFF ?? 'oasdiff';
  const text = composeChangelog({
    base: readDocument(flags.base),
    head: readDocument(flags.head),
    version: flags.version,
    from: flags.from,
    command,
    cwd: flags['oasdiff-cwd'] ? resolve(flags['oasdiff-cwd']) : undefined,
  });
  if (flags.check) {
    const current = existsSync(resolve(flags.output)) ? readFileSync(resolve(flags.output), 'utf8') : '';
    must(current === text, `${flags.output} differs from the generated changelog; run make api-changelog VERSION=${flags.version} and commit it`);
  } else {
    writeText(flags.output, text);
  }
}
