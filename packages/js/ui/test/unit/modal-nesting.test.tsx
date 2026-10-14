// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { Dialog } from '../../src/components/dialog/Dialog';

import { renderWithProviders } from './helpers';

const backdrops = () => Array.from(document.querySelectorAll('[data-state="open"].bg-scrim'));

describe('a dialog opened from inside another dialog', () => {
  it('has its own layer, so the parent dims: the backdrop is at the modal layer, not under the parent panel', () => {
    renderWithProviders(
      <Dialog open title="Parent">
        <Dialog open title="Child">
          <p>Inside</p>
        </Dialog>
      </Dialog>,
    );
    expect(screen.getByRole('dialog', { name: 'Child' })).toBeInTheDocument();
    const [parent, child] = backdrops();
    expect(parent?.className).toContain('z-overlay');
    expect(child?.className).toContain('z-modal');
    expect(child?.className).not.toContain('z-overlay');
  });

  it('keeps the overlay layer for a dialog that is not nested', () => {
    renderWithProviders(<Dialog open title="Alone" />);
    expect(backdrops()[0]?.className).toContain('z-overlay');
  });
});
