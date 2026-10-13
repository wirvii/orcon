// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import { Plus } from 'lucide-react';
import { describe, expect, it } from 'vitest';

import { Avatar, initials } from '../../src/components/avatar/Avatar';
import { Badge } from '../../src/components/badge/Badge';
import { IconButton } from '../../src/components/icon-button/IconButton';
import { Kbd, KbdCombination } from '../../src/components/kbd/Kbd';
import { Link } from '../../src/components/link/Link';
import { Separator } from '../../src/components/separator/Separator';
import { Skeleton } from '../../src/components/skeleton/Skeleton';

import { renderWithProviders } from './helpers';

describe('IconButton', () => {
  it('takes its accessible name from label and hides the icon', () => {
    const { container } = renderWithProviders(<IconButton label="Add item" icon={Plus} />);
    expect(screen.getByRole('button', { name: 'Add item' })).toBeInTheDocument();
    expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  });

  it('shows the busy state instead of the icon while loading', () => {
    const { container } = renderWithProviders(<IconButton label="Add item" icon={Plus} loading />);
    const button = screen.getByRole('button', { name: 'Add item' });
    expect(button).toHaveAttribute('aria-busy', 'true');
    expect(container.querySelectorAll('svg')).toHaveLength(1);
  });
});

describe('Link', () => {
  it('opens external links in a new tab safely and says so', () => {
    renderWithProviders(
      <Link href="https://example.com" external>
        Docs
      </Link>,
    );
    const link = screen.getByRole('link', { name: 'Docs (opens in a new tab)' });
    expect(link).toHaveAttribute('target', '_blank');
    expect(link).toHaveAttribute('rel', 'noopener noreferrer');
  });

  it('translates the new-tab notice', () => {
    renderWithProviders(
      <Link href="https://example.com" external>
        Docs
      </Link>,
      { locale: 'es' },
    );
    expect(screen.getByRole('link', { name: /se abre en una pestaña nueva/ })).toBeInTheDocument();
  });

  it('has no target size classes when it sits in text', () => {
    renderWithProviders(
      <Link href="#x" inText>
        here
      </Link>,
    );
    expect(screen.getByRole('link').className).not.toContain('min-h-target');
  });
});

describe('Avatar', () => {
  it('builds initials from the first and last word', () => {
    expect(initials('Ada Lovelace')).toBe('AL');
    expect(initials('  grace  brewster  hopper ')).toBe('GH');
    expect(initials('Plato')).toBe('P');
    expect(initials('')).toBe('');
  });

  it('is an image with the person name as its name', () => {
    renderWithProviders(<Avatar name="Ada Lovelace" />);
    expect(screen.getByRole('img', { name: 'Ada Lovelace' })).toBeInTheDocument();
  });
});

describe('Badge', () => {
  it('renders its text and the tone classes', () => {
    renderWithProviders(<Badge tone="danger">Blocked</Badge>);
    const badge = screen.getByText('Blocked');
    expect(badge.className).toContain('text-status-danger');
    expect(badge.className).toContain('bg-status-danger-bg');
  });
});

describe('Kbd', () => {
  it('renders each key and hides the plus signs', () => {
    renderWithProviders(<KbdCombination keys={['Ctrl', 'K']} />);
    expect(screen.getByText('Ctrl').tagName).toBe('KBD');
    expect(screen.getByText('K').tagName).toBe('KBD');
    expect(screen.getByText('+')).toHaveAttribute('aria-hidden', 'true');
    renderWithProviders(<Kbd>Esc</Kbd>);
  });
});

describe('Separator and Skeleton', () => {
  it('Separator is decorative by default', () => {
    const { container } = renderWithProviders(<Separator />);
    expect(container.firstElementChild).toHaveAttribute('role', 'none');
  });

  it('Separator is exposed when it is meaningful', () => {
    renderWithProviders(<Separator decorative={false} />);
    expect(screen.getByRole('separator')).toBeInTheDocument();
  });

  it('Skeleton is hidden from assistive technology', () => {
    const { container } = renderWithProviders(<Skeleton />);
    expect(container.firstElementChild).toHaveAttribute('aria-hidden', 'true');
  });
});
