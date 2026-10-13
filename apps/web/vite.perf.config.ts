// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

const here = dirname(fileURLToPath(import.meta.url));

// The editor performance harness (orcon-web PF-4): the design system's canvas with the reference
// flow, built apart from the app and never shipped in the binary. `pnpm build:perf` writes
// dist-perf; `vite preview --config vite.perf.config.ts` serves it for the measurement.
export default defineConfig({
  root: resolve(here, 'perf/harness'),
  plugins: [react(), tailwindcss()],
  build: { outDir: resolve(here, 'dist-perf'), emptyOutDir: true },
  preview: { host: '127.0.0.1', port: 4174, strictPort: true },
});
