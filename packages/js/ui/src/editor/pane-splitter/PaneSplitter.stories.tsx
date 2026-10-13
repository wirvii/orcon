// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { PaneSplitter } from './PaneSplitter';

function Demo({ start = 420, className }: { start?: number; className?: string }) {
  const [size, setSize] = useState(start);
  return (
    <div className="bg-surface border-border flex h-canvas-node-wide w-canvas-palette-w items-stretch rounded-lg border">
      <div className="text-fg-muted flex flex-1 items-center justify-center text-sm">{size}</div>
      <PaneSplitter label="Resize the panes" value={size} min={280} max={640} onChange={setSize} direction={-1} className={className} />
      <div className="text-fg-muted flex flex-1 items-center justify-center text-sm">{size}</div>
    </div>
  );
}

const meta = {
  title: 'Editor/PaneSplitter',
  component: PaneSplitter,
  args: { label: 'Resize the panes', value: 420, min: 280, max: 640, onChange: () => undefined },
  render: () => <Demo />,
} satisfies Meta<typeof PaneSplitter>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const handle = within(canvasElement).getByRole('separator', { name: 'Resize the panes' });
    await expect(handle).toHaveAttribute('aria-valuenow', '420');
    handle.focus();
    // Arrow keys move it 20 px per press; direction -1 means the left arrow makes the pane larger.
    await userEvent.keyboard('{ArrowLeft}');
    await expect(handle).toHaveAttribute('aria-valuenow', '440');
    await userEvent.keyboard('{ArrowRight}{ArrowRight}');
    await expect(handle).toHaveAttribute('aria-valuenow', '400');
  },
};
export const Hover: Story = { render: () => <Demo className="pseudo-hover" /> };
export const FocusVisible: Story = { render: () => <Demo className="pseudo-focus-visible" /> };
export const AtMinimum: Story = { render: () => <Demo start={280} /> };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Pane splitter"
      columns={['default', 'hover', 'focus-visible', 'at minimum']}
      rows={[{ label: 'handle', cells: [<Demo key="a" />, <Demo key="b" className="pseudo-hover" />, <Demo key="c" className="pseudo-focus-visible" />, <Demo key="d" start={280} />] }]}
    />
  ),
};
