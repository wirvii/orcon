// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { exampleCatalog } from '../fixtures';

import { NodePalettePanel } from './NodePalettePanel';

const meta = {
  title: 'Canvas/NodePalettePanel',
  component: NodePalettePanel,
  args: { items: exampleCatalog, onPick: () => undefined, onClose: () => undefined },
  decorators: [
    (Story) => (
      <div className="bg-canvas relative h-screen w-screen max-w-screen-sm overflow-hidden rounded-lg">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof NodePalettePanel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The search field has focus when the panel opens.
    await expect(within(canvasElement).getByRole('combobox', { name: 'Search nodes' })).toHaveFocus();
  },
};
export const TriggersOnly: Story = {
  args: { filter: 'triggers' },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).queryByText('HTTP Request')).toBeNull();
  },
};
export const NoMatch: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.type(within(canvasElement).getByRole('combobox', { name: 'Search nodes' }), 'zzzz');
    await expect(await within(canvasElement).findByText('No node matches your search')).toBeVisible();
  },
};
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <div role="group" aria-label="Node palette panel" className="flex flex-wrap items-start gap-6 p-4">
      {[
        { label: 'all nodes', filter: undefined },
        { label: 'triggers only', filter: 'triggers' as const },
      ].map(({ label, filter }) => (
        <figure key={label} className="m-0 flex shrink-0 flex-col gap-2">
          <figcaption className="text-fg-muted text-sm font-medium">{label}</figcaption>
          <div className="bg-canvas relative h-screen w-canvas-palette-w shrink-0 overflow-hidden rounded-lg">
            <NodePalettePanel items={exampleCatalog} filter={filter} onPick={() => undefined} onClose={() => undefined} />
          </div>
        </figure>
      ))}
    </div>
  ),
};
