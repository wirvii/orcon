// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import tailwindcss from '@tailwindcss/vite';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

import { routes } from './src/app/routes';

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
  ],
  build: { outDir: 'dist' },
});
