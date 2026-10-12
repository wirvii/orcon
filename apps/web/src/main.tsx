// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import '@orcon/ui/styles.css';

import { ErrorBoundary, Providers, WelcomeScreen } from '@orcon/ui';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

const root = document.getElementById('root');
if (!root) throw new Error('The page has no #root element to mount the app in.');

createRoot(root).render(
  <StrictMode>
    <Providers>
      <ErrorBoundary>
        <WelcomeScreen />
      </ErrorBoundary>
    </Providers>
  </StrictMode>,
);
