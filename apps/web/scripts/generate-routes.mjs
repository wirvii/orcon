// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Regenerates the committed route tree (src/app/routeTree.gen.ts) from the route files, with the
// same router plugin and options the build uses: it starts Vite without a port, lets the plugin
// run its generator, and stops. `pnpm generate:check` runs it and fails when the file changed.

import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { createServer } from 'vite';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..');

const server = await createServer({
  root,
  configFile: resolve(root, 'vite.config.ts'),
  appType: 'custom',
  logLevel: 'warn',
  server: { middlewareMode: true, watch: null },
});
try {
  // The plugin generates the route tree when the build starts.
  await server.pluginContainer.buildStart({});
} finally {
  await server.close();
}
console.log('generate: src/app/routeTree.gen.ts is up to date');
