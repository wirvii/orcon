// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { StatusBadge } from '../../src/components/status-badge/StatusBadge';
import { statusKeys, statusMap, statusMessages } from '../../src/components/status-badge/status-map';
import { catalogs } from '../../src/i18n/locales';

import { renderWithProviders } from './helpers';

describe('StatusBadge', () => {
  it.each(statusKeys)('%s shows an icon and visible text', (status) => {
    const { container } = renderWithProviders(<StatusBadge status={status} />);
    const badge = container.querySelector('[data-status]');
    expect(badge).not.toBeNull();
    // Meaning is never carried by color alone: a decorative icon plus text.
    expect(badge?.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
    expect(badge?.textContent.trim().length).toBeGreaterThan(0);
  });

  it('covers every state of the normative map with a label in en and es', () => {
    for (const entry of Object.values(statusMap)) {
      const id = entry.label.id as string;
      expect(catalogs.en[id], id).toBeTruthy();
      expect(catalogs.es[id], id).toBeTruthy();
    }
  });

  it('UNCERTAIN uses the attention family, never danger or success', () => {
    expect(statusMap.UNCERTAIN.family).toBe('attention');
    const { container } = renderWithProviders(<StatusBadge status="UNCERTAIN" />);
    const classes = container.querySelector('[data-status]')?.className ?? '';
    expect(classes).toContain('status-attention');
    expect(classes).not.toMatch(/status-(danger|success)/);
    expect(statusMap.UNCERTAIN.label).toBe(statusMessages.uncertain);
  });

  it('UNCERTAIN is never named as a failure or a success, in either language', () => {
    for (const locale of ['en', 'es'] as const) {
      const { unmount } = renderWithProviders(<StatusBadge status="UNCERTAIN" />, { locale });
      const text = document.body.textContent;
      expect(text).not.toMatch(/fail|succe|fall|correct|éxito/i);
      unmount();
    }
  });

  it('UNCERTAIN exposes the fixed description to assistive technology', () => {
    const { container } = renderWithProviders(<StatusBadge status="UNCERTAIN" />);
    expect(container.querySelector('[data-status]')).toHaveAccessibleDescription(
      'The external system may or may not have applied this operation. Review it before continuing.',
    );
  });

  it('keeps the success, warning and danger families for their own states', () => {
    expect(statusMap.SUCCEEDED.family).toBe('success');
    expect(statusMap.SUCCEEDED_WITH_ERRORS.family).toBe('warning');
    expect(statusMap.FAILED.family).toBe('danger');
    expect(statusMap.ERROR_FINAL.family).toBe('danger');
  });

  it('reserves the attention family for UNCERTAIN and states that need a person', () => {
    const attention = statusKeys.filter((key) => statusMap[key].family === 'attention').sort();
    expect(attention).toEqual(['AWAITING_PERSON', 'SUSPENDED', 'UNCERTAIN']);
  });

  it('only the running indicator turns, and only without reduced motion', () => {
    const turning = statusKeys.filter((key) => statusMap[key].spins);
    expect(turning).toEqual(['RUNNING']);
    const { container } = renderWithProviders(<StatusBadge status="RUNNING" />);
    expect(container.querySelector('svg')?.getAttribute('class')).toContain('motion-safe:animate-spin-slow');
    expect(screen.getByText('Running')).toBeInTheDocument();
  });
});
