// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { Button } from '../../src/components/button/Button';

import { renderWithProviders } from './helpers';

describe('Button', () => {
  it('is a non-submitting button by default', () => {
    renderWithProviders(<Button>Save</Button>);
    expect(screen.getByRole('button', { name: 'Save' })).toHaveAttribute('type', 'button');
  });

  it('calls onClick', async () => {
    const onClick = vi.fn();
    renderWithProviders(<Button onClick={onClick}>Save</Button>);
    await userEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('ignores clicks while loading, stays focusable and says it is busy', async () => {
    const onClick = vi.fn();
    renderWithProviders(
      <Button loading onClick={onClick}>
        Save
      </Button>,
    );
    const button = screen.getByRole('button');
    expect(button).toHaveAttribute('aria-busy', 'true');
    expect(button).toHaveAttribute('aria-disabled', 'true');
    expect(button).not.toBeDisabled();
    await userEvent.click(button, { pointerEventsCheck: 0 });
    expect(onClick).not.toHaveBeenCalled();
  });

  it('does not fire when disabled', async () => {
    const onClick = vi.fn();
    renderWithProviders(
      <Button disabled onClick={onClick}>
        Save
      </Button>,
    );
    await userEvent.click(screen.getByRole('button'), { pointerEventsCheck: 0 });
    expect(onClick).not.toHaveBeenCalled();
  });

  it('renders the child element with the button styling when asChild is set', () => {
    renderWithProviders(
      <Button asChild variant="secondary">
        <a href="/flows">Flows</a>
      </Button>,
    );
    const link = screen.getByRole('link', { name: 'Flows' });
    expect(link).toHaveAttribute('href', '/flows');
    expect(link.className).toContain('border-control');
  });
});
