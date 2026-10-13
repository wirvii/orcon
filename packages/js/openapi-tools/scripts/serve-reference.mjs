// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Serves a built reference site for a manual check or a test, with the headers the binary
 * sends (ADR 0080 section 10): the content security policy, nosniff, and a 301 from
 * /api/reference to /api/reference/ that keeps the query string.
 *
 *   node scripts/serve-reference.mjs --dir dist/api-reference [--port 4010] [--mount /api/reference/] [--csp "<policy>"]
 *
 * GET /api/v1/... answers 401 so "try it" has a same-origin target. Not used by the build.
 */

import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, resolve, sep } from 'node:path';

import { must } from './doc-io.mjs';
import { isMain, parseArgs } from './lib.mjs';

/** The content security policy of the reference site (ADR 0080 section 10). */
export const REFERENCE_CSP = "default-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; base-uri 'self'; form-action 'none'; frame-ancestors 'none'; object-src 'none'";

const TYPES = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.json': 'application/json', '.txt': 'text/plain; charset=utf-8' };

/** Creates (does not start) the server. */
export function createReferenceServer({ dir, mount = '/api/reference/', csp = REFERENCE_CSP, log = () => {} }) {
  const root = resolve(dir);
  return createServer((req, res) => {
    const url = new URL(req.url, 'http://localhost');
    res.setHeader('X-Content-Type-Options', 'nosniff');
    res.setHeader('Content-Security-Policy', csp);
    log(`${req.method} ${url.pathname} cookie=${req.headers.cookie ? 'sent' : 'none'}`);
    if (url.pathname === mount.replace(/\/$/, '')) {
      res.writeHead(301, { Location: `${mount}${url.search}` });
      res.end();
      return;
    }
    if (url.pathname.startsWith('/api/v1/')) {
      res.writeHead(401, { 'Content-Type': 'application/problem+json' });
      res.end(JSON.stringify({ type: 'urn:orcon:problem:unauthenticated', title: 'Unauthenticated', status: 401, code: 'unauthenticated' }));
      return;
    }
    if (!url.pathname.startsWith(mount)) {
      res.writeHead(404);
      res.end();
      return;
    }
    const file = join(root, url.pathname.slice(mount.length) || 'index.html');
    if (!file.startsWith(root + sep) || !existsSync(file) || !statSync(file).isFile()) {
      res.writeHead(404);
      res.end();
      return;
    }
    res.writeHead(200, { 'Content-Type': TYPES[extname(file)] ?? 'application/octet-stream' });
    if (req.method === 'HEAD') res.end();
    else createReadStream(file).pipe(res);
  });
}

if (isMain(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2));
  must(flags.dir, 'usage: serve-reference.mjs --dir <dist/api-reference> [--port 4010] [--mount /api/reference/] [--csp <policy>]');
  const mount = flags.mount ?? '/api/reference/';
  const port = Number(flags.port ?? 4010);
  createReferenceServer({ dir: flags.dir, mount, csp: flags.csp, log: (line) => process.stdout.write(`${line}\n`) }).listen(port, '127.0.0.1', () =>
    process.stdout.write(`serving ${resolve(flags.dir)} at http://127.0.0.1:${port}${mount}\n`),
  );
}
