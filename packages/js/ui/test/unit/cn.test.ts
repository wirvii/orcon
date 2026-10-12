// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { cn } from '../../src/lib/cn';

describe('cn', () => {
  it('joins truthy values and drops falsy ones', () => {
    const off = Math.random() > 2;
    expect(cn('flex', off && 'hidden', undefined, 'gap-2')).toBe('flex gap-2');
  });

  it('lets the last of two conflicting token utilities win', () => {
    expect(cn('h-control', 'h-row')).toBe('h-row');
    expect(cn('px-control-x', 'px-2')).toBe('px-2');
    expect(cn('bg-surface', 'bg-surface-sunken')).toBe('bg-surface-sunken');
  });

  it('keeps a text color and a text size together', () => {
    expect(cn('text-fg', 'text-sm')).toBe('text-fg text-sm');
    expect(cn('text-fg', 'text-md')).toBe('text-fg text-md');
    expect(cn('text-fg', 'text-fg-muted')).toBe('text-fg-muted');
  });
});
