// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Git helpers of `make api-compat` and `make api-changelog`: find the last released tag and
 * bundle the contract as it was at a git reference.
 */

import { spawnSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, relative, resolve } from 'node:path';

import { bundle } from './bundle.mjs';
import { readDocument } from './doc-io.mjs';
import { defaultContract, repoRoot } from './lib.mjs';

function git(args, options = {}) {
  const result = spawnSync('git', args, { encoding: 'utf8', cwd: repoRoot(), maxBuffer: 256 * 1024 * 1024, ...options });
  if (result.error) throw result.error;
  return result;
}

/** The latest `v*` tag reachable from `ref` (excluding `except` when `ref` is that very tag), or undefined. */
export function latestReleaseTag(ref = 'HEAD', except) {
  let result = git(['describe', '--tags', '--abbrev=0', '--match', 'v*', ref]);
  if (result.status === 0 && result.stdout.trim() === except) result = git(['describe', '--tags', '--abbrev=0', '--match', 'v*', `${ref}^`]);
  return result.status === 0 ? result.stdout.trim() : undefined;
}

/** The contract directory relative to the repository root ("packages/openapi/v1"). */
export const contractDirRelative = () => relative(repoRoot(), resolve(defaultContract(), '..'));

/** True when the contract exists in the tree of `ref`. */
export function contractExistsAt(ref) {
  return git(['cat-file', '-e', `${ref}:${contractDirRelative()}/openapi.yaml`]).status === 0;
}

/**
 * Bundles the contract of `ref` and returns the parsed document. The files are extracted
 * with `git archive` into a temporary directory that is removed afterwards.
 */
export function bundledDocumentAt(ref) {
  const dir = mkdtempSync(join(tmpdir(), 'orcon-api-base-'));
  try {
    const archive = git(['archive', '--format=tar', ref, contractDirRelative()], { encoding: 'buffer' });
    if (archive.status !== 0) throw new Error(`git archive ${ref} failed: ${archive.stderr}`);
    const untar = spawnSync('tar', ['-x', '-C', dir], { input: archive.stdout });
    if (untar.status !== 0) throw new Error(`tar failed: ${untar.stderr}`);
    const out = join(dir, 'bundle.yaml');
    mkdirSync(dir, { recursive: true });
    bundle(join(dir, contractDirRelative(), 'openapi.yaml'), out);
    return readDocument(out);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

/** The bundled document of the contract in the working tree, or undefined when there is none. */
export function bundledDocumentOfWorkingTree() {
  if (!existsSync(defaultContract())) return undefined;
  const dir = mkdtempSync(join(tmpdir(), 'orcon-api-head-'));
  try {
    const out = join(dir, 'bundle.yaml');
    bundle(defaultContract(), out);
    return readDocument(out);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}
