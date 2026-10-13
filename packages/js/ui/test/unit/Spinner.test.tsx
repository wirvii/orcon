// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Spinner } from '../../src/components/spinner/Spinner';

import { renderWithProviders } from './helpers';

describe('Spinner', () => {
  it('has an accessible name from the catalog', () => {
    renderWithProviders(<Spinner />);
    expect(screen.getByRole('img', { name: 'Loading' })).toBeInTheDocument();
  });

  it('translates the name', () => {
    renderWithProviders(<Spinner />, { locale: 'es' });
    expect(screen.getByRole('img', { name: 'Cargando' })).toBeInTheDocument();
  });

  it('is hidden from assistive technology when decorative', () => {
    const { container } = renderWithProviders(<Spinner decorative />);
    expect(screen.queryByRole('img')).toBeNull();
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });
});
