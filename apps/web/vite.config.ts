// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

import { chunkModules } from './build/chunk-modules';
import { thirdParty } from './build/third-party';
import { routes } from './src/app/routes';

const here = dirname(fileURLToPath(import.meta.url));
const repositoryRoot = resolve(here, '../..');
const outDir = resolve(here, 'dist');
const brandAssetsInlineLimit = (file: string) => (/[\\/]assets[\\/](brand|mascot)[\\/]/.test(file) ? false : undefined);

// The build is embedded in the Go binary and served from its root, so asset paths stay absolute.
// The router plugin must come before the React plugin: it generates the committed route tree
// (src/app/routeTree.gen.ts) and splits every route into its own chunk.
export default defineConfig({
  plugins: [
    tanstackRouter({
      target: 'react',
      autoCodeSplitting: true,
      routesDirectory: './src/app',
      generatedRouteTree: './src/app/routeTree.gen.ts',
      virtualRouteConfig: routes,
      routeFileIgnorePattern: '\\.test\\.',
      routeTreeFileHeader: ['// SPDX-FileCopyrightText: The Orcon Authors', '// SPDX-License-Identifier: Apache-2.0'],
      quoteStyle: 'single',
      disableLogging: true,
    }),
    react(),
    tailwindcss(),
    // The bundle checks read what is inside each chunk. The rollup-plugin-license plugin (wired in
    // build/third-party.ts, used below) writes the notices and the list of bundled packages the
    // license check reads.
    chunkModules(repositoryRoot),
  ],
  build: {
    outDir,
    manifest: true,
    // Brand art is emitted as files, never inlined as data: URIs in the JavaScript, so a strict
    // Content-Security-Policy (img-src 'self') works. Anything else keeps Vite's default limit (4 kB).
    assetsInlineLimit: brandAssetsInlineLimit,
    rollupOptions: {
      plugins: [thirdParty(outDir, resolve(repositoryRoot, 'packages/js/ui'))],
      output: {
        // The binary embeds `dist` with go:embed, which leaves out every file whose name starts with "_" or ".".
        // The router plugin names the chunk of `$flowId.tsx` "_flowId", so a leading "_" is dropped (the hash keeps names unique).
        chunkFileNames: (chunk) => `assets/${chunk.name.replace(/^[._]+/, '') || 'chunk'}-[hash].js`,
      },
    },
  },
});
