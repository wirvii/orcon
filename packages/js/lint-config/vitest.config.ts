// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineConfig } from 'vitest/config';

// The rule tests build type-aware ESLint programs, which take a while the first time.
export default defineConfig({
  test: { testTimeout: 120_000, hookTimeout: 120_000 },
});
