// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
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
    <MatrixTable
      caption="Node palette panel"
      columns={['all nodes', 'triggers only']}
      rows={[
        {
          label: 'panel',
          cells: [
            <div key="a" className="bg-canvas relative h-screen w-canvas-palette-w overflow-hidden rounded-lg"><NodePalettePanel items={exampleCatalog} onPick={() => undefined} onClose={() => undefined} /></div>,
            <div key="b" className="bg-canvas relative h-screen w-canvas-palette-w overflow-hidden rounded-lg"><NodePalettePanel items={exampleCatalog} filter="triggers" onPick={() => undefined} onClose={() => undefined} /></div>,
          ],
        },
      ]}
    />
  ),
};
