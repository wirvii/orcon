// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const quiet = ['test/harness/quiet.ts'];

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
      '@testing-library/user-event',
      '@xyflow/react',
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
          setupFiles: quiet,
          include: ['test/a11y/**/*.test.tsx'],
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // The flow canvas end to end, in a real browser: keyboard operation of every graph operation.
        extends: true,
        test: {
          name: 'canvas',
          setupFiles: quiet,
          include: ['test/canvas/**/*.test.tsx'],
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // Every parity row of the editor: its pointer script and its keyboard script, in light and in dark.
        extends: true,
        test: {
          name: 'parity',
          setupFiles: quiet,
          include: ['test/parity/**/*.test.tsx'],
          testTimeout: 30_000,
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // The only looping motion is on running elements: with motion allowed, nothing else animates.
        extends: true,
        test: {
          name: 'canvas-motion',
          setupFiles: quiet,
          include: ['test/canvas-motion/**/*.test.tsx'],
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'no-preference', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // Fine pointer: rows and controls are exactly as tall as the density tokens say.
        extends: true,
        test: {
          name: 'density',
          setupFiles: quiet,
          include: ['test/density/**/*.test.tsx'],
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // Pseudo-locale xx-LS: no text container overflows unless truncation is declared.
        extends: true,
        test: {
          name: 'expansion',
          setupFiles: quiet,
          include: ['test/expansion/**/*.test.tsx'],
          browser: { ...browser, instances: [{ browser: 'chromium', context: { reducedMotion: 'reduce', timezoneId: 'UTC', locale: 'en-US' } }] },
        },
      },
      {
        // Touch and a coarse pointer: every interactive control is at least 44x44 px.
        extends: true,
        test: {
          name: 'targets',
          setupFiles: quiet,
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
