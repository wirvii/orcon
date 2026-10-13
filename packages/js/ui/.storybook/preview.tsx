// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Preview } from '@storybook/react-vite';

import '../src/styles/index.css';
import { Providers } from '../src/providers/Providers';

const preview: Preview = {
  initialGlobals: {
    theme: 'light',
    density: 'comfortable',
    locale: 'en',
    viewport: { value: 'desktop', isRotated: false },
  },
  globalTypes: {
    theme: {
      description: 'Color theme',
      toolbar: {
        title: 'Theme',
        icon: 'circlehollow',
        dynamicTitle: true,
        items: [
          { value: 'light', title: 'Light' },
          { value: 'dark', title: 'Dark' },
          { value: 'system', title: 'System' },
        ],
      },
    },
    density: {
      description: 'Density',
      toolbar: {
        title: 'Density',
        icon: 'ellipsis',
        dynamicTitle: true,
        items: [
          { value: 'comfortable', title: 'Comfortable' },
          { value: 'compact', title: 'Compact' },
        ],
      },
    },
    locale: {
      description: 'Locale (xx-LS is the long-string pseudo-locale)',
      toolbar: {
        title: 'Locale',
        icon: 'globe',
        dynamicTitle: true,
        items: [
          { value: 'en', title: 'English' },
          { value: 'es', title: 'Español' },
          { value: 'xx-LS', title: 'Pseudo (xx-LS)' },
        ],
      },
    },
  },
  parameters: {
    layout: 'fullscreen',
    controls: { expanded: true },
    viewport: {
      options: {
        mobile: { name: 'Mobile 390', styles: { width: '390px', height: '844px' }, type: 'mobile' },
        tablet: { name: 'Tablet 1024', styles: { width: '1024px', height: '768px' }, type: 'tablet' },
        desktop: { name: 'Desktop 1440', styles: { width: '1440px', height: '900px' }, type: 'desktop' },
      },
    },
    a11y: { test: 'error' },
  },
  decorators: [
    (Story, context) => (
      <Providers
        theme={context.globals.theme}
        density={context.globals.density}
        locale={context.globals.locale}
      >
        <div className="bg-app text-fg min-h-screen p-4">
          <Story />
        </div>
      </Providers>
    ),
  ],
};

export default preview;
