// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

import { routes } from './src/app/routes';

const here = dirname(fileURLToPath(import.meta.url));

// The editor performance build (orcon-web PF-4): the whole app, built in the `perf` mode so that
// `/flows/01930000-0000-7000-8000-000000000500` opens the 500-node reference flow, and never shipped
// in the binary. `pnpm build:perf` writes dist-perf; `vite preview --config vite.perf.config.ts`
// serves it (with the single-page fallback) for the measurement, which opens the real editor route.
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
  ],
  build: { outDir: resolve(here, 'dist-perf'), emptyOutDir: true },
  preview: { host: '127.0.0.1', port: 4174, strictPort: true },
});
