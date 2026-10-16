// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test, vi } from 'vitest';

import { renderInApp } from '../../../../test/screen-harness';

import { LogsPanel, type LogStep } from './LogsPanel';

const steps: LogStep[] = [
  { id: 'a', name: 'Message received', ok: true, durationMs: 0 },
  { id: 'b', name: 'Sales agent', ok: true, durationMs: 2100, inner: [{ name: 'Memory read', durationMs: 18 }] },
];

test('LogsPanel omits agent tokens when the execution does not report them', () => {
  const view = renderInApp(<LogsPanel state="ready" open={false} onOpenChange={vi.fn()} run={{ status: 'SUCCEEDED', durationMs: 2400, when: '3 min ago' }} steps={steps} />);
  expect(screen.getByText('2 steps · 2.4 s · 3 min ago')).toBeVisible();
  expect(screen.queryByText(/tokens/)).toBeNull();
  view.unmount();

  renderInApp(<LogsPanel state="ready" open={false} onOpenChange={vi.fn()} run={{ status: 'SUCCEEDED', durationMs: 2400, tokens: 1284, when: '3 min ago' }} steps={steps} />);
  expect(screen.getByText('2 steps · 2.4 s · 1,284 tokens · 3 min ago')).toBeVisible();
});

test('LogsPanel opens to the steps with their durations and the output of the chosen step', async () => {
  const onSelect = vi.fn();
  renderInApp(
    <LogsPanel
      state="ready"
      open
      onOpenChange={vi.fn()}
      run={{ status: 'SUCCEEDED', durationMs: 2400 }}
      steps={steps}
      selectedId="b"
      onSelect={onSelect}
      output={[{ intent: 'buy' }]}
    />,
  );
  expect(screen.getByText('2.1 s')).toBeVisible();
  expect(screen.getByText('Memory read')).toBeVisible();
  expect(screen.getByText('Sales agent: output')).toBeVisible();
  expect(screen.getByText(/"intent": "buy"/)).toBeVisible();
  await userEvent.click(screen.getByRole('button', { name: /Message received/ }));
  expect(onSelect).toHaveBeenCalledWith('a');
});

test('LogsPanel says the runs are not available yet instead of failing', () => {
  renderInApp(<LogsPanel state="unavailable" open={false} onOpenChange={vi.fn()} />);
  expect(screen.getByText('The runs of a flow are not available yet on this server.')).toBeVisible();
  expect(screen.queryByRole('alert')).toBeNull();
});

test('LogsPanel says a flow with no runs has not run yet', () => {
  renderInApp(<LogsPanel state="empty" open={false} onOpenChange={vi.fn()} />);
  expect(screen.getByText('This flow has not run yet. Its runs show here.')).toBeVisible();
});
