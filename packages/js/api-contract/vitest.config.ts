// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from 'vitest/config';

// The pipeline test runs the generator, which takes a few seconds.
export default defineConfig({
  test: { testTimeout: 60_000, hookTimeout: 120_000, include: ['test/**/*.test.ts'], exclude: ['test/.out/**', '**/node_modules/**'] },
});
