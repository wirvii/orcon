// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';

import { BUDGET_BYTES, buildReference, externalResourceProblems, hostsIn, renderConfigJs, renderIndexHtml, scalarScript } from '../scripts/build-reference.mjs';
import { bundle } from '../scripts/bundle.mjs';
import { decorate } from '../scripts/decorate.mjs';
import { readDocument } from '../scripts/doc-io.mjs';
import { operationsOf } from '../scripts/doc-io.mjs';
import { packageDir } from '../scripts/lib.mjs';
import { renderOperationIndex } from '../scripts/operation-index.mjs';
import { renderProblemCodes } from '../scripts/problem-codes.mjs';
import { withoutExperimental } from '../scripts/stability-filter.mjs';

const good = join(packageDir, 'testdata', 'good', 'openapi.yaml');
let work;
let doc;
let site;

before(() => {
  work = mkdtempSync(join(tmpdir(), 'orcon-reference-test-'));
  const bundled = join(work, 'openapi.bundle.yaml');
  bundle(good, bundled);
  doc = readDocument(bundled);
  site = buildReference({ input: good, output: join(work, 'site'), basePath: '/api/reference/' });
});

after(() => rmSync(work, { recursive: true, force: true }));

test('P1: the reference site builds with the vendored Scalar script and its license, within 3 MiB', () => {
  assert.deepEqual(site.problems, []);
  assert.deepEqual(site.files.map((f) => f.name), ['LICENSE-scalar.txt', 'index.html', 'openapi.json', 'reference-config.js', 'scalar.standalone.js']);
  assert.ok(site.gzipTotal <= BUDGET_BYTES, `${site.gzipTotal} bytes compressed`);
  const script = readFileSync(join(work, 'site', 'scalar.standalone.js'));
  assert.equal(createHash('sha256').update(script).digest('hex'), createHash('sha256').update(readFileSync(scalarScript())).digest('hex'), 'the script is copied unchanged');
  assert.match(readFileSync(join(work, 'site', 'LICENSE-scalar.txt'), 'utf8'), /MIT License[\s\S]*Scalar/);
});

test('P1: the site loads nothing from another origin and "try it" is same-origin without cookies', () => {
  const html = readFileSync(join(work, 'site', 'index.html'), 'utf8');
  const config = readFileSync(join(work, 'site', 'reference-config.js'), 'utf8');
  assert.doesNotMatch(html, /https?:\/\//);
  assert.deepEqual(hostsIn(config), []);
  assert.match(html, /<script src="scalar\.standalone\.js">/);
  assert.doesNotMatch(html, /<script>[^<]/, 'no inline script (the content security policy forbids it)');
  assert.match(config, /credentials: 'omit'/);
  assert.match(config, /persistAuth: false/);
  assert.match(config, /withDefaultFonts: false/);
  assert.match(config, /telemetry: false/);
  assert.match(config, /url: "\/api\/v1"/);
  const published = JSON.parse(readFileSync(join(work, 'site', 'openapi.json'), 'utf8'));
  assert.deepEqual(published.servers, [{ url: '/api/v1' }]);
});

test('the external resource check catches a CDN stylesheet, script, proxy and server', () => {
  const html = '<link rel="stylesheet" href="https://cdn.example.net/x.css"><script src="//cdn.example.net/y.js"></script>';
  const problems = externalResourceProblems({
    html,
    config: "Scalar.createApiReference('#app', { proxyUrl: 'https://proxy.scalar.com' })",
    openapi: { servers: [{ url: 'https://api.example.com' }], paths: { '/x': { $ref: 'https://evil.example.net/x.yaml' } } },
  });
  assert.equal(problems.length, 5, problems.join('\n'));
  assert.deepEqual(externalResourceProblems({ html: renderIndexHtml('t', '/api/reference/'), config: renderConfigJs('/api/v1'), openapi: { servers: [{ url: '/api/v1' }] } }), []);
});

test('the decorated contract carries the generated sections only in the published copy', () => {
  const published = decorate(doc);
  const publish = published.paths['/flows/{flowId}/publish'].post;
  assert.match(publish.description, /Required permission: `flow:publish`/);
  assert.match(publish.description, /Rate class: `write`/);
  assert.match(publish.description, /Stability: `stable`/);
  assert.match(publish.description, /Idempotency: `optional`/);
  assert.match(publish.description, /\[`not-found`\]\(#description\/problem-codes\)/);
  assert.match(publish.description, /CLI: `orcon flows publish`/);
  assert.match(publish.description, /MCP: tool `publish_flow`, level `destructive`/);
  assert.doesNotMatch(doc.paths['/flows/{flowId}/publish'].post.description, /Required permission/, 'the contract itself is untouched');
  assert.match(published.info.description, /## Problem codes/);
  assert.match(published.webhooks['flow.published'].post.description, /CloudEvents type: `dev\.orcon\.flow\.published\.v1`/);
  assert.equal(operationsOf(published).length, operationsOf(doc).length);
});

test('the problem code page lists the catalog of the contract', () => {
  const text = renderProblemCodes(doc);
  assert.match(text, /^<!-- SPDX-FileCopyrightText: The Orcon Authors -->/);
  assert.match(text, /\| <a id="not-found"><\/a>`not-found` \| 404 \| Not found \|/);
  assert.equal([...text.matchAll(/<a id=/g)].length, 21);
});

test('the operation index lists operations and events', () => {
  const text = renderOperationIndex(doc);
  assert.match(text, /\| POST \| `\/api\/v1\/flows\/\{flowId\}\/publish` \| `publishFlow` \| Publish a flow draft \| stable \| `flow:publish` \| `orcon flows publish` \| `publish_flow` \(destructive\) \|/);
  assert.match(text, /\| `flow\.published` \| `dev\.orcon\.flow\.published\.v1` \|/);
});

test('the stability filter removes experimental operations and keeps the rest', () => {
  const copy = structuredClone(doc);
  copy.paths['/flows/{flowId}'].get['x-orcon-stability'] = 'experimental';
  const filtered = withoutExperimental(copy);
  assert.equal(filtered.paths['/flows/{flowId}'].get, undefined);
  assert.ok(filtered.paths['/flows/{flowId}'].patch);
  assert.equal(copy.paths['/flows/{flowId}'].get['x-orcon-stability'], 'experimental', 'the input is not modified');
});
