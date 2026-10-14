// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { after, before, test } from 'node:test';
import vm from 'node:vm';

import { BUDGET_BYTES, buildReference, externalResourceProblems, hostsIn, renderConfigJs, renderIndexHtml, scalarScript } from '../scripts/build-reference.mjs';
import { bundle } from '../scripts/bundle.mjs';
import { decorate } from '../scripts/decorate.mjs';
import { readDocument } from '../scripts/doc-io.mjs';
import { operationsOf } from '../scripts/doc-io.mjs';
import { packageDir } from '../scripts/lib.mjs';
import { REFERENCE_CSP, createReferenceServer } from '../scripts/serve-reference.mjs';
import { renderOperationIndex } from '../scripts/operation-index.mjs';
import { renderProblemCodes } from '../scripts/problem-codes.mjs';
import { withoutExperimental } from '../scripts/stability-filter.mjs';
import { bundledPackages, expressionAllowed, licenseProblems, loadExceptions } from '../scripts/third-party.mjs';

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
  assert.deepEqual(site.files.map((f) => f.name), ['LICENSE-scalar.txt', 'index.html', 'openapi.json', 'reference-config.js', 'scalar.standalone.js', 'third-party-notices.txt']);
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

test('the page reads the contract with the browser credentials and sends "try it" calls without them', async () => {
  const config = readFileSync(join(work, 'site', 'reference-config.js'), 'utf8');
  const calls = [];
  let options;
  const context = vm.createContext({
    URL,
    Request,
    document: { baseURI: 'https://host.example/api/reference/' },
    Scalar: { createApiReference: (_selector, o) => (options = o) },
    fetch: async (input, init) => calls.push({ url: String(input.url ?? input), credentials: init?.credentials }),
  });
  vm.runInContext(config, context);
  await options.customFetch('openapi.json', {});
  await options.customFetch('https://host.example/api/reference/openapi.json', undefined);
  await options.customFetch(new Request('https://host.example/api/v1/flows'), { method: 'GET' });
  await options.customFetch('/api/v1/flows', {});
  assert.deepEqual(calls.map((c) => c.credentials), [undefined, undefined, 'omit', 'omit']);
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

test('third-party notices cover Scalar and every bundled package with an allowed license', () => {
  const text = readFileSync(join(work, 'site', 'third-party-notices.txt'), 'utf8');
  assert.match(text, /^Third-party notices of the Orcon API reference site/);
  assert.match(text, /@scalar\/api-reference@\d+\.\d+\.\d+ \(MIT\)\n\n[\s\S]*?MIT License\n\nCopyright \(c\) 2023-present Scalar/);
  assert.match(text, /\nvue@\d+\.\d+\.\d+ \(MIT\)/);
  const packages = bundledPackages(scalarScript());
  assert.ok(packages.length > 100, `${packages.length} bundled packages`);
  assert.deepEqual(licenseProblems(packages), []);
  assert.equal([...text.matchAll(/^-{80}$/gm)].length, packages.length);
});

test('license check: allowed expressions, exceptions and failures name the package', () => {
  assert.ok(expressionAllowed('MIT'));
  assert.ok(expressionAllowed('(MIT OR CC0-1.0)'));
  assert.ok(expressionAllowed('(AFL-2.1 OR BSD-3-Clause)'));
  assert.ok(expressionAllowed('MIT AND ISC'));
  assert.ok(!expressionAllowed('GPL-3.0-only'));
  assert.ok(!expressionAllowed('MIT AND GPL-3.0-only'));
  assert.ok(!expressionAllowed('0BSD'));
  const fake = [{ name: 'tslib', version: '2.8.1', pkg: { license: '0BSD' } }, { name: 'left-pad', version: '1.0.0', pkg: { license: 'GPL-3.0-only' } }];
  assert.deepEqual(licenseProblems(fake, new Set(['tslib@2.8.1'])).map((m) => m.match(/left-pad@1.0.0/)?.[0]), ['left-pad@1.0.0']);
  assert.equal(licenseProblems(fake, new Set()).length, 2);
  assert.ok(loadExceptions().has('tslib@2.8.1'), 'the repository exception file is read');
});

test('the preview server sends the reference policy and redirects /api/reference keeping the query', async () => {
  const server = createReferenceServer({ dir: join(work, 'site') });
  await new Promise((resolveListening) => server.listen(0, '127.0.0.1', resolveListening));
  try {
    const base = `http://127.0.0.1:${server.address().port}`;
    const redirect = await fetch(`${base}/api/reference?tag=flows`, { redirect: 'manual' });
    assert.equal(redirect.status, 301);
    assert.equal(redirect.headers.get('location'), '/api/reference/?tag=flows');
    const head = await fetch(`${base}/api/reference`, { method: 'HEAD', redirect: 'manual' });
    assert.equal(head.status, 301);
    const page = await fetch(`${base}/api/reference/`);
    assert.equal(page.status, 200);
    assert.equal(page.headers.get('content-security-policy'), REFERENCE_CSP);
    assert.equal(page.headers.get('x-content-type-options'), 'nosniff');
    assert.match(await page.text(), /<base href="\/api\/reference\/"/);
    assert.equal((await fetch(`${base}/api/reference/%2e%2e/%2e%2e/etc/passwd`)).status, 404);
    assert.equal((await fetch(`${base}/api/reference/third-party-notices.txt`)).status, 200);
  } finally {
    server.close();
  }
});
