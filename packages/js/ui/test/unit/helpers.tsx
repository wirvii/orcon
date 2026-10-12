// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { render, type RenderResult } from '@testing-library/react';
import type { ReactElement, ReactNode } from 'react';

import { Providers, type ProvidersProps } from '../../src/providers/Providers';

/** Renders inside theme, density and locale providers (light, comfortable, en unless overridden). */
export function renderWithProviders(
  ui: ReactElement,
  { theme = 'light', density = 'comfortable', locale = 'en' }: Omit<ProvidersProps, 'children'> = {},
): RenderResult {
  const wrap = (element: ReactNode) => (
    <Providers theme={theme} density={density} locale={locale}>
      {element}
    </Providers>
  );
  const result = render(wrap(ui));
  return { ...result, rerender: (next: ReactNode) => result.rerender(wrap(next)) };
}
