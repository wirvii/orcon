// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import tailwindcss from '@tailwindcss/vite';
import type { StorybookConfig } from '@storybook/react-vite';

const config: StorybookConfig = {
  stories: ['../src/**/*.stories.@(ts|tsx)'],
  addons: ['@storybook/addon-a11y', 'storybook-addon-pseudo-states'],
  framework: { name: '@storybook/react-vite', options: {} },
  core: { disableTelemetry: true },
  viteFinal: (viteConfig) => ({
    ...viteConfig,
    plugins: [...(viteConfig.plugins ?? []), tailwindcss()],
    // Brand art stays a file, never a data: URI (same rule as the app build).
    build: {
      ...viteConfig.build,
      assetsInlineLimit: (file: string) => (/[\\/]assets[\\/](brand|mascot)[\\/]/.test(file) ? false : undefined),
    },
  }),
};

export default config;
