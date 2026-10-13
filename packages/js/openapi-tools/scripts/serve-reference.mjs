// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Serves a built reference site for a manual check, with the headers the binary sends
 * (content security policy, nosniff). Not used by the build.
 *
 *   node scripts/serve-reference.mjs --dir dist/api-reference [--port 4010] [--mount /api/reference/]
 *        [--csp "default-src 'self'; connect-src 'self'"]
 *
 * The site is mounted at --mount; GET /api/v1/... answers 401 so "try it" has a target.
 */

import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, resolve, sep } from 'node:path';

import { must } from './doc-io.mjs';
import { parseArgs } from './lib.mjs';

const { flags } = parseArgs(process.argv.slice(2));
must(flags.dir, 'usage: serve-reference.mjs --dir <dist/api-reference> [--port 4010] [--mount /api/reference/] [--csp <policy>]');
const dir = resolve(flags.dir);
const port = Number(flags.port ?? 4010);
const mount = flags.mount ?? '/api/reference/';
const csp = flags.csp ?? "default-src 'self'; connect-src 'self'";
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.json': 'application/json', '.txt': 'text/plain; charset=utf-8' };

createServer((req, res) => {
  const url = new URL(req.url, 'http://localhost');
  res.setHeader('X-Content-Type-Options', 'nosniff');
  res.setHeader('Content-Security-Policy', csp);
  process.stdout.write(`${req.method} ${url.pathname} cookie=${req.headers.cookie ? 'sent' : 'none'}\n`);
  if (url.pathname === mount.replace(/\/$/, '')) {
    res.writeHead(301, { Location: mount });
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
  const rel = url.pathname.slice(mount.length) || 'index.html';
  const file = join(dir, rel);
  if (!file.startsWith(dir + sep) || !existsSync(file) || !statSync(file).isFile()) {
    res.writeHead(404);
    res.end();
    return;
  }
  res.writeHead(200, { 'Content-Type': types[extname(file)] ?? 'application/octet-stream' });
  createReadStream(file).pipe(res);
}).listen(port, '127.0.0.1', () => process.stdout.write(`serving ${dir} at http://127.0.0.1:${port}${mount}\n`));
