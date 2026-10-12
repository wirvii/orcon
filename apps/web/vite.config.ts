// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// The build is embedded in the Go binary and served from its root, so asset paths stay absolute.
export default defineConfig({
  plugins: [tailwindcss()],
  build: { outDir: 'dist' },
});
