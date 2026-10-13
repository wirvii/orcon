// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0
// Validates release-please-config.json and .release-please-manifest.json
// (orcon-contributing RL-2, ADR 0081). No dependencies: Node standard library only.
// Usage: node scripts/release/check-release-please-config.mjs

import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const problems = [];

function readJson(name) {
  try {
    return JSON.parse(readFileSync(join(root, name), 'utf8'));
  } catch (err) {
    problems.push(`${name}: ${err.message}`);
    return undefined;
  }
}

// isAncestorOfMain reports whether sha is reachable from origin/main (or main when
// there is no remote). A checkout without that history cannot prove it, so it fails:
// CI checks out with fetch-depth 0.
function isAncestorOfMain(sha) {
  for (const ref of ['origin/main', 'main']) {
    try {
      execFileSync('git', ['-C', root, 'rev-parse', '--verify', '--quiet', `${ref}^{commit}`], { stdio: 'ignore' });
    } catch {
      continue;
    }
    try {
      execFileSync('git', ['-C', root, 'merge-base', '--is-ancestor', sha, ref], { stdio: 'ignore' });
      return true;
    } catch {
      return false;
    }
  }
  return false;
}

const config = readJson('release-please-config.json');
const manifest = readJson('.release-please-manifest.json');
const commitlint = (await import(pathToFileURL(join(root, 'commitlint.config.mjs')).href)).default;
const commitTypes = commitlint.rules['type-enum'][2];

if (config) {
  const expected = {
    'release-type': 'go',
    'include-component-in-tag': false,
    'include-v-in-tag': true,
    'bump-minor-pre-major': true,
    'bump-patch-for-minor-pre-major': false,
    'pull-request-title-pattern': 'chore(tooling): release ${version}',
  };
  for (const [key, value] of Object.entries(expected)) {
    if (config[key] !== value) {
      problems.push(`release-please-config.json: ${key} must be ${JSON.stringify(value)}, got ${JSON.stringify(config[key])}`);
    }
  }
  const bootstrap = config['bootstrap-sha'] ?? '';
  if (!/^[0-9a-f]{40}$/.test(bootstrap)) {
    problems.push('release-please-config.json: bootstrap-sha must be a 40-character commit hash');
  } else if (!isAncestorOfMain(bootstrap)) {
    problems.push(`release-please-config.json: bootstrap-sha ${bootstrap} is not an ancestor of main (origin/main or main); set it to a commit on main`);
  }
  const sections = config['changelog-sections'];
  if (!Array.isArray(sections)) {
    problems.push('release-please-config.json: changelog-sections must be a list');
  } else {
    const types = sections.map((s) => s.type).sort();
    const want = [...commitTypes].sort();
    if (JSON.stringify(types) !== JSON.stringify(want)) {
      problems.push(`release-please-config.json: changelog-sections types ${types.join(',')} differ from the commit types ${want.join(',')}`);
    }
    for (const s of sections) {
      if (typeof s.section !== 'string' || s.section === '') {
        problems.push(`release-please-config.json: changelog section of type ${s.type} has no name`);
      }
    }
  }
  const pkg = config.packages?.['.'];
  if (!pkg || pkg['package-name'] !== 'orcon' || pkg['changelog-path'] !== 'CHANGELOG.md') {
    problems.push('release-please-config.json: packages["."] must have package-name "orcon" and changelog-path "CHANGELOG.md"');
  }
}

if (manifest) {
  const semver = /^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/;
  if (Object.keys(manifest).join() !== '.' || !semver.test(manifest['.'] ?? '')) {
    problems.push('.release-please-manifest.json: must be {"." : "<semver>"}');
  }
}

if (problems.length > 0) {
  for (const p of problems) {
    console.error(`release-config-check: ${p}`);
  }
  process.exit(1);
}
console.log('release-please configuration: ok');
