// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { UncertainResolutionPanel, type UncertainResolutionAction } from '../../src/components/uncertain-resolution/UncertainResolutionPanel';
import { catalogs } from '../../src/i18n/locales';

import { renderWithProviders } from './helpers';

const actions: UncertainResolutionAction[] = [
  { id: 'mark-applied', label: 'Mark as charged', kind: 'mark-applied' },
  { id: 'mark-not-applied', label: 'Mark as not charged and continue', kind: 'mark-not-applied', requiresRiskAck: true, riskText: 'The customer may be charged twice.' },
  { id: 'repeat', label: 'Charge again', kind: 'repeat', requiresRiskAck: true, riskText: 'The first charge may have gone through.' },
];

describe('UNCERTAIN actions', () => {
  it('never makes a repeat the primary action, and no action at all is primary', () => {
    const { container } = renderWithProviders(<UncertainResolutionPanel nodeName="Charge card" actions={actions} />);
    const primary = Array.from(container.querySelectorAll('button')).filter((button) => button.className.includes('bg-action-primary'));
    expect(primary).toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Charge again' }).className).not.toContain('bg-action-primary');
  });

  it('keeps actions with requiresRiskAck disabled until the person ticks the acknowledgment', async () => {
    const user = userEvent.setup();
    const onResolve = vi.fn();
    renderWithProviders(<UncertainResolutionPanel nodeName="Charge card" actions={actions} onResolve={onResolve} />);
    const repeat = screen.getByRole('button', { name: 'Charge again' });
    expect(repeat).toBeDisabled();
    await user.click(repeat);
    expect(onResolve).not.toHaveBeenCalled();

    await user.click(screen.getByRole('checkbox', { name: /The first charge may have gone through/ }));
    expect(repeat).toBeEnabled();
    await user.click(repeat);
    expect(onResolve).toHaveBeenCalledWith('repeat');
    // The other risky action keeps its own acknowledgment.
    expect(screen.getByRole('button', { name: 'Mark as not charged and continue' })).toBeDisabled();
  });

  it('lets a safe resolution run at once', async () => {
    const user = userEvent.setup();
    const onResolve = vi.fn();
    renderWithProviders(<UncertainResolutionPanel nodeName="Charge card" actions={actions} onResolve={onResolve} />);
    await user.click(screen.getByRole('button', { name: 'Mark as charged' }));
    expect(onResolve).toHaveBeenCalledWith('mark-applied');
  });

  it('shows only the resolutions it received and the cancel button', () => {
    renderWithProviders(<UncertainResolutionPanel nodeName="Charge card" actions={[]} />);
    expect(screen.getByText('This connector offers no resolution. You can cancel the execution.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel the execution' })).toBeEnabled();
    expect(screen.queryByRole('checkbox')).toBeNull();
  });

  it('shows the connector manual check as plain text, never as markup', () => {
    const { container } = renderWithProviders(
      <UncertainResolutionPanel nodeName="Charge card" actions={[]} manualCheck={'Check <b>the</b> dashboard'} />,
    );
    const text = container.querySelector('[data-manual-check]');
    expect(text?.textContent).toBe('Check <b>the</b> dashboard');
    expect(text?.querySelector('b')).toBeNull();
  });

  it('describes the situation without naming a failure or a success, in either language', () => {
    for (const locale of ['en', 'es'] as const) {
      const text = `${catalogs[locale]['uncertain.title']} ${catalogs[locale]['status.uncertain.description']}`.toLowerCase();
      expect(text).not.toMatch(/fail|succe|fall[óo]|error|éxito|exitos/);
    }
  });
});
