// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const browser = {
  enabled: true,
  provider: 'playwright',
  headless: true,
} as const;

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Pre-bundle everything the browser projects import, so Vite never re-optimizes (and reloads) mid-run.
  optimizeDeps: {
    include: [
      'axe-core',
      'class-variance-authority',
      'clsx',
      'cmdk',
      'lucide-react',
      'radix-ui',
      'react',
      'react-dom',
      'react-dom/client',
      'react/jsx-dev-runtime',
      'react-intl',
      'sonner',
      'storybook/test',
      'tailwind-merge',
      '@testing-library/react',
    ],
  },
  test: {
    projects: [
      {
        extends: true,
        test: {
          name: 'unit',
          environment: 'jsdom',
          include: ['test/unit/**/*.test.{ts,tsx}'],
          setupFiles: ['test/unit/setup.ts'],
        },
      },
      {
        // Every story, rendered in a real browser, in light and dark: zero axe violations.
        extends: true,
        test: {
          name: 'a11y',
          include: ['test/a11y/**/*.test.tsx'],
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // Fine pointer: rows and controls are exactly as tall as the density tokens say.
        extends: true,
        test: {
          name: 'density',
          include: ['test/density/**/*.test.tsx'],
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // Touch and a coarse pointer: every interactive control is at least 44x44 px.
        extends: true,
        test: {
          name: 'targets',
          include: ['test/targets/**/*.test.tsx'],
          browser: {
            ...browser,
            instances: [{ browser: 'chromium', context: { hasTouch: true, isMobile: true, reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }],
          },
        },
      },
    ],
  },
});
