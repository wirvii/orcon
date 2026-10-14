// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Serves the repository read-only on 127.0.0.1 so the brand comparison page
// can be opened at http://127.0.0.1:4178/brand/review/ (node brand/review/serve.mjs).
// Local address only; GET and HEAD only; paths outside the repository are refused.

import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const HOST = '127.0.0.1';

const types = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.md': 'text/plain; charset=utf-8',
};

export function createReviewServer() {
  return createServer(async (req, res) => {
    const fail = (code, message) => {
      res.writeHead(code, { 'content-type': 'text/plain; charset=utf-8' });
      res.end(message);
    };
    if (req.method !== 'GET' && req.method !== 'HEAD') return fail(405, 'read-only');
    let pathname;
    try {
      pathname = decodeURIComponent(new URL(req.url ?? '/', `http://${HOST}`).pathname);
    } catch {
      return fail(400, 'bad request');
    }
    if (pathname === '/') {
      res.writeHead(302, { location: '/brand/review/' });
      return res.end();
    }
    const target = path.resolve(root, `.${pathname}`);
    const rel = path.relative(root, target);
    if (rel.startsWith('..') || path.isAbsolute(rel) || rel.split(path.sep).some((part) => part.startsWith('.'))) {
      return fail(403, 'outside the repository');
    }
    try {
      let file = target;
      if ((await stat(file)).isDirectory()) file = path.join(file, 'index.html');
      const body = await readFile(file);
      res.writeHead(200, {
        'content-type': types[path.extname(file)] ?? 'application/octet-stream',
        'cache-control': 'no-store',
      });
      res.end(req.method === 'HEAD' ? undefined : body);
    } catch {
      fail(404, 'not found');
    }
  });
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const port = Number(process.env.BRAND_REVIEW_PORT ?? 4178);
  createReviewServer().listen(port, HOST, () => {
    console.log(`Brand review page: http://${HOST}:${port}/brand/review/`);
  });
}
