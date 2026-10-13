// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { mixedItems, orderItems } from '../fixtures';
import type { PaneData } from '../model';

import { DataPane } from './DataPane';

const data: PaneData = { items: orderItems, from: 'HTTP Request', run: { index: 2, total: 5 } };

const meta = {
  title: 'Editor/DataPane',
  component: DataPane,
  args: { mode: 'output', data, onPinChange: () => undefined, onEdit: () => undefined, onExecute: () => undefined },
  decorators: [
    (Story) => (
      <div className="bg-surface border-border flex h-canvas-palette-w max-w-screen-md overflow-hidden rounded-lg border">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof DataPane>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByText('from HTTP Request')).toBeVisible();
    await expect(view.getByText('Run 2 of 5')).toBeVisible();
    await expect(view.getByText('3 items')).toBeVisible();
    await expect(view.getByRole('button', { name: 'Table' })).toHaveAttribute('aria-pressed', 'true');
    await userEvent.click(view.getByRole('button', { name: 'JSON' }));
    await expect(view.getByRole('tree', { name: 'Data as JSON' })).toBeVisible();
    await userEvent.click(view.getByRole('button', { name: 'Schema' }));
    await expect(view.getByRole('list', { name: 'Data fields' })).toBeVisible();
  },
};
export const Json: Story = { args: { defaultView: 'json' } };
export const Schema: Story = { args: { defaultView: 'schema' } };
export const InputEmpty: Story = { args: { mode: 'input', data: undefined } };
export const OutputEmpty: Story = { args: { data: undefined } };
export const Loading: Story = { args: { status: 'loading', data: undefined } };
export const ErrorState: Story = { args: { status: 'error', data: undefined, error: 'The step could not run: the request timed out.' } };
export const Pinned: Story = { args: { data: { ...data, pinned: true } } };
export const MultiOutput: Story = {
  args: {
    ports: [
      { id: 'true', label: 'true', count: 2 },
      { id: 'false', label: 'false', count: 1 },
    ],
    activePort: 'true',
    data: { items: orderItems.slice(0, 2), from: 'Validate order' },
  },
};
export const Protected: Story = { args: { data: { items: mixedItems, from: 'Gmail' } } };

export const Matrix: Story = {
  decorators: [],
  render: () => {
    const frame = (key: string, node: React.ReactNode) => (
      <div key={key} className="bg-surface border-border flex h-canvas-palette-w w-canvas-palette-w overflow-hidden rounded-lg border">
        {node}
      </div>
    );
    return (
      <MatrixTable
        caption="Data pane"
        columns={['data', 'empty', 'loading', 'error', 'pinned']}
        rows={[
          {
            label: 'output',
            cells: [
              frame('a', <DataPane mode="output" data={data} onPinChange={() => undefined} onEdit={() => undefined} />),
              frame('b', <DataPane mode="output" onExecute={() => undefined} />),
              frame('c', <DataPane mode="output" status="loading" />),
              frame('d', <DataPane mode="output" status="error" error="The step could not run." />),
              frame('e', <DataPane mode="output" data={{ ...data, pinned: true }} onPinChange={() => undefined} />),
            ],
          },
          {
            label: 'input',
            cells: [
              frame('f', <DataPane mode="input" data={{ items: orderItems, from: 'Webhook' }} />),
              frame('g', <DataPane mode="input" onExecute={() => undefined} />),
              frame('h', <DataPane mode="input" status="loading" />),
              frame('i', <DataPane mode="input" status="error" error="No data: the node before it failed." />),
              frame('j', <DataPane mode="input" data={{ items: mixedItems, from: 'Gmail' }} defaultView="json" />),
            ],
          },
        ]}
      />
    );
  },
};
