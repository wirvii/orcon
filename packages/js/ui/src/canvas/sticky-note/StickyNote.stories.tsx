// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';
import type { StickyColor } from '../model';

import { StickyNote } from './StickyNote';

const note = { id: 'note-1', color: 1 as StickyColor, size: { w: 240, h: 120 }, body: 'Orders above **5,000** need a manual review first.\n- Handled by the review flow\n- Ask finance when unsure' };

const meta = {
  title: 'Canvas/StickyNote',
  component: StickyNote,
  args: { note },
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg px-8 pt-16 pb-8">
        <div className="h-canvas-node-wide w-panel-width">
          <Story />
        </div>
      </div>
    ),
  ],
} satisfies Meta<typeof StickyNote>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Selected: Story = { args: { selected: true, editable: true } };
export const Editing: Story = { args: { editable: true, defaultEditing: true } };
export const Empty: Story = { args: { note: { ...note, body: '' } } };
export const Color2: Story = { args: { note: { ...note, color: 2 } } };
export const Color3: Story = { args: { note: { ...note, color: 3 } } };
export const Color4: Story = { args: { note: { ...note, color: 4 } } };
export const Color5: Story = { args: { note: { ...note, color: 5 } } };
export const Color6: Story = { args: { note: { ...note, color: 6 } } };

const colors: StickyColor[] = [1, 2, 3, 4, 5, 6];

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Sticky note colors"
      columns={colors.map((color) => `color ${color}`)}
      rows={[
        {
          label: 'note',
          cells: colors.map((color) => (
            <div key={color} className="bg-canvas rounded-lg p-4">
              <div className="h-canvas-node-wide w-panel-width">
                <StickyNote note={{ ...note, color }} editable />
              </div>
            </div>
          )),
        },
      ]}
    />
  ),
};
