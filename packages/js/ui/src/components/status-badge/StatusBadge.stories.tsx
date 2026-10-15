// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { StatusBadge } from './StatusBadge';
import { states } from './StatusBadge.meta';
import { statusMap, type StatusKey } from './status-map';

const meta = {
  title: 'Components/StatusBadge',
  component: StatusBadge,
  args: { status: 'QUEUED' },
} satisfies Meta<typeof StatusBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: { status: 'QUEUED' } };
export const Running: Story = { args: { status: 'RUNNING' } };
export const Waiting: Story = { args: { status: 'WAITING' } };
export const Suspended: Story = { args: { status: 'SUSPENDED' } };
export const Canceling: Story = { args: { status: 'CANCELING' } };
export const Canceled: Story = { args: { status: 'CANCELED' } };
export const Compensating: Story = { args: { status: 'COMPENSATING' } };
export const Succeeded: Story = { args: { status: 'SUCCEEDED' } };
export const SucceededWithErrors: Story = { args: { status: 'SUCCEEDED_WITH_ERRORS' } };
export const Failed: Story = { args: { status: 'FAILED' } };
export const Uncertain: Story = {
  args: { status: 'UNCERTAIN' },
  play: async ({ canvasElement }) => {
    const badge = within(canvasElement).getByText('Needs review').closest('[data-status]');
    await expect(badge).toHaveAccessibleDescription(
      'The external system may or may not have applied this operation. Review it before continuing.',
    );
  },
};
export const RetryScheduled: Story = { args: { status: 'RETRY_SCHEDULED' } };
export const NotDispatched: Story = { args: { status: 'NOT_DISPATCHED' } };
export const NotRun: Story = { args: { status: 'NOT_RUN' } };
export const AwaitingPerson: Story = { args: { status: 'AWAITING_PERSON' } };

const keys: Record<(typeof states)[number], StatusKey> = {
  default: 'QUEUED',
  running: 'RUNNING',
  waiting: 'WAITING',
  suspended: 'SUSPENDED',
  canceling: 'CANCELING',
  canceled: 'CANCELED',
  compensating: 'COMPENSATING',
  succeeded: 'SUCCEEDED',
  'succeeded-with-errors': 'SUCCEEDED_WITH_ERRORS',
  failed: 'FAILED',
  uncertain: 'UNCERTAIN',
  'retry-scheduled': 'RETRY_SCHEDULED',
  'not-dispatched': 'NOT_DISPATCHED',
  'not-run': 'NOT_RUN',
  'awaiting-person': 'AWAITING_PERSON',
};

/** Every state, grouped by color family. */
export const Matrix: Story = {
  render: () => {
    const families = ['neutral', 'active', 'attention', 'success', 'warning', 'danger'] as const;
    return (
      <MatrixTable
        caption="Status map by color family"
        columns={['states']}
        rows={families.map((family) => ({
          label: family,
          cells: [
            <div key={family} className="flex flex-wrap gap-2">
              {states
                .filter((state) => statusMap[keys[state]].family === family)
                .map((state) => (
                  <StatusBadge key={state} status={keys[state]} />
                ))}
            </div>,
          ],
        }))}
      />
    );
  },
};
