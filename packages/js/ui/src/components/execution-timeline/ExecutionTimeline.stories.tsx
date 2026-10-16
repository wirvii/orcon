// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ExecutionTimeline, type TimelineStep } from './ExecutionTimeline';
import { states } from './ExecutionTimeline.meta';

const ok: TimelineStep[] = [
  { id: 'a', name: 'Every hour', status: 'SUCCEEDED', startMs: 0, durationMs: 2 },
  { id: 'b', name: 'Unpaid orders', status: 'SUCCEEDED', startMs: 40, durationMs: 1100 },
  { id: 'c', name: 'Write the reminder', status: 'SUCCEEDED', startMs: 1200, durationMs: 1700 },
];
const failed: TimelineStep[] = [
  ...ok,
  { id: 'd', name: 'Send on WhatsApp', status: 'FAILED', startMs: 2900, durationMs: 900, note: '7 of 12 failed' },
  { id: 'e', name: 'Mark as reminded', status: 'NOT_RUN' },
];
const review: TimelineStep[] = [...ok, { id: 'd', name: 'Send on WhatsApp', status: 'UNCERTAIN', startMs: 2900, durationMs: 900 }];
const notRun: TimelineStep[] = [...ok, { id: 'e', name: 'Mark as reminded', status: 'NOT_RUN' }, { id: 'f', name: 'Tell the team', status: 'NOT_RUN' }];

const meta = {
  title: 'Components/ExecutionTimeline',
  component: ExecutionTimeline,
  args: { steps: ok, totalMs: 3800, label: 'Steps of the run' },
} satisfies Meta<typeof ExecutionTimeline>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const list = within(canvasElement).getByRole('list', { name: 'Steps of the run' });
    await expect(within(list).getAllByRole('listitem')).toHaveLength(3);
  },
};
export const Failed: Story = {
  args: { steps: failed },
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByText('7 of 12 failed')).toBeVisible();
    // A step that did not run has no bar and says so in words.
    await expect(view.getByText('Did not run')).toBeVisible();
  },
};
export const NeedsReview: Story = { args: { steps: review } };
export const NotRun: Story = { args: { steps: notRun } };
export const Selected: Story = { args: { steps: failed, selectedId: 'd', onSelect: fn() } };
export const Selectable: Story = {
  args: { steps: failed, onSelect: fn() },
  play: async ({ canvasElement, args }) => {
    const view = within(canvasElement);
    await userEvent.click(view.getByRole('button', { name: /Send on WhatsApp/ }));
    await expect(args.onSelect).toHaveBeenCalledWith('d');
  },
};

const stateProps: Record<(typeof states)[number], Partial<Parameters<typeof ExecutionTimeline>[0]>> = {
  default: {},
  failed: { steps: failed },
  'needs-review': { steps: review },
  'not-run': { steps: notRun },
  selected: { steps: failed, selectedId: 'd', onSelect: () => undefined },
  selectable: { steps: ok, onSelect: () => undefined },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ExecutionTimeline states"
      columns={[...states]}
      rows={[
        {
          label: 'timeline',
          cells: states.map((state) => <ExecutionTimeline key={state} steps={ok} totalMs={3800} label={`Steps ${state}`} {...stateProps[state]} />),
        },
      ]}
    />
  ),
};
