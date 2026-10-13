// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// A stand-in for the Orcon binary when it is not built: it serves the production build the way the
// binary must (compressed assets, immutable hashed files, index.html for every UI route), so the
// first-useful-screen measurement can run without a Go toolchain. The real measurement is against
// the binary (`make web-perf`); this server is only for `ORCON_PERF_SERVER=static`.
//
//   node perf/serve-static.mjs [port]

import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { dirname, extname, join, normalize, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createGzip } from 'node:zlib';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'dist');
const port = Number(process.argv[2] ?? 18081);

const types = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json',
  '.txt': 'text/plain; charset=utf-8',
  '.woff2': 'font/woff2',
};
const compressible = new Set(['.html', '.js', '.css', '.json', '.txt']);

createServer((request, response) => {
  const url = new URL(request.url ?? '/', 'http://localhost');
  let file = normalize(join(root, decodeURIComponent(url.pathname)));
  if (!file.startsWith(root)) {
    response.writeHead(403).end();
    return;
  }
  const isAsset = url.pathname.startsWith('/assets/');
  if (!existsSync(file) || statSync(file).isDirectory()) {
    if (isAsset || url.pathname.startsWith('/api/')) {
      response.writeHead(404).end();
      return;
    }
    file = join(root, 'index.html');
  }
  const extension = extname(file);
  const headers = {
    'Content-Type': types[extension] ?? 'application/octet-stream',
    'Cache-Control': isAsset ? 'public, max-age=31536000, immutable' : 'no-cache',
  };
  const wantsGzip = compressible.has(extension) && /\bgzip\b/.test(request.headers['accept-encoding'] ?? '');
  if (wantsGzip) {
    response.writeHead(200, { ...headers, 'Content-Encoding': 'gzip', Vary: 'Accept-Encoding' });
    createReadStream(file).pipe(createGzip()).pipe(response);
  } else {
    response.writeHead(200, headers);
    createReadStream(file).pipe(response);
  }
}).listen(port, '127.0.0.1', () => {
  console.log(`serve-static: ${root} on http://127.0.0.1:${String(port)}`);
});
