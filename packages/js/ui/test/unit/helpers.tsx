// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { render, type RenderResult } from '@testing-library/react';
import type { ReactElement } from 'react';

import { Providers, type ProvidersProps } from '../../src/providers/Providers';

/** Renders inside theme, density and locale providers (light, comfortable, en unless overridden). */
export function renderWithProviders(
  ui: ReactElement,
  { theme = 'light', density = 'comfortable', locale = 'en' }: Omit<ProvidersProps, 'children'> = {},
): RenderResult {
  return render(
    <Providers theme={theme} density={density} locale={locale}>
      {ui}
    </Providers>,
  );
}
