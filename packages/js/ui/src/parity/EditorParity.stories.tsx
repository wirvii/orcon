// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { ParityStage } from './ParityStage';
import { playParity } from './play';

// One story per row of the parity table that this repository owns (canvas.md#parity). Each plays
// its keyboard script and checks the end state; the visual suite takes a baseline of that end state
// in light and dark. The browser test runs the pointer script of each row too.
const meta = {
  title: 'Editor/Parity',
  component: ParityStage,
  parameters: { layout: 'fullscreen' },
} satisfies Meta<typeof ParityStage>;

export default meta;
type Story = StoryObj<typeof meta>;

export const P01: Story = {
  name: 'P-01 First step on an empty canvas',
  args: { row: 'P-01' },
  play: ({ canvasElement }) => playParity('P-01', canvasElement),
};

export const P02: Story = {
  name: 'P-02 Add a node after a node',
  args: { row: 'P-02' },
  play: ({ canvasElement }) => playParity('P-02', canvasElement),
};

export const P03: Story = {
  name: 'P-03 Add after a dangling output',
  args: { row: 'P-03' },
  play: ({ canvasElement }) => playParity('P-03', canvasElement),
};

export const P04: Story = {
  name: 'P-04 Insert a node on a connection',
  args: { row: 'P-04' },
  play: ({ canvasElement }) => playParity('P-04', canvasElement),
};

export const P05: Story = {
  name: 'P-05 Delete a connection',
  args: { row: 'P-05' },
  play: ({ canvasElement }) => playParity('P-05', canvasElement),
};

export const P06: Story = {
  name: 'P-06 Connect two nodes',
  args: { row: 'P-06' },
  play: ({ canvasElement }) => playParity('P-06', canvasElement),
};

export const P07: Story = {
  name: 'P-07 Move and multi-select',
  args: { row: 'P-07' },
  play: ({ canvasElement }) => playParity('P-07', canvasElement),
};

export const P08: Story = {
  name: 'P-08 Node quick actions',
  args: { row: 'P-08' },
  play: ({ canvasElement }) => playParity('P-08', canvasElement),
};

export const P09: Story = {
  name: 'P-09 Select and open a node',
  args: { row: 'P-09' },
  play: ({ canvasElement }) => playParity('P-09', canvasElement),
};

export const P10: Story = {
  name: 'P-10 Node details view',
  args: { row: 'P-10' },
  play: ({ canvasElement }) => playParity('P-10', canvasElement),
};

export const P11: Story = {
  name: 'P-11 Switch data view',
  args: { row: 'P-11' },
  play: ({ canvasElement }) => playParity('P-11', canvasElement),
};

export const P13: Story = {
  name: 'P-13 Execute one step',
  args: { row: 'P-13' },
  play: ({ canvasElement }) => playParity('P-13', canvasElement),
};

export const P14: Story = {
  name: 'P-14 Execute the workflow',
  args: { row: 'P-14' },
  play: ({ canvasElement }) => playParity('P-14', canvasElement),
};

export const P15: Story = {
  name: 'P-15 Pin output data',
  args: { row: 'P-15' },
  play: ({ canvasElement }) => playParity('P-15', canvasElement),
};

export const P16: Story = {
  name: 'P-16 Disable a node',
  args: { row: 'P-16' },
  play: ({ canvasElement }) => playParity('P-16', canvasElement),
};

export const P17: Story = {
  name: 'P-17 Read a failed node',
  args: { row: 'P-17' },
  play: ({ canvasElement }) => playParity('P-17', canvasElement),
};

export const P18: Story = {
  name: 'P-18 Read an uncertain node (Orcon improvement)',
  args: { row: 'P-18' },
  play: ({ canvasElement }) => playParity('P-18', canvasElement),
};

export const P19: Story = {
  name: 'P-19 Sticky notes',
  args: { row: 'P-19' },
  play: ({ canvasElement }) => playParity('P-19', canvasElement),
};

export const P20: Story = {
  name: 'P-20 Zoom',
  args: { row: 'P-20' },
  play: ({ canvasElement }) => playParity('P-20', canvasElement),
};

export const P21: Story = {
  name: 'P-21 Minimap',
  args: { row: 'P-21' },
  play: ({ canvasElement }) => playParity('P-21', canvasElement),
};

export const P22: Story = {
  name: 'P-22 Pan',
  args: { row: 'P-22' },
  play: ({ canvasElement }) => playParity('P-22', canvasElement),
};

export const P25: Story = {
  name: 'P-25 Rename a node',
  args: { row: 'P-25' },
  play: ({ canvasElement }) => playParity('P-25', canvasElement),
};

export const P27: Story = {
  name: 'P-27 Find a node type',
  args: { row: 'P-27' },
  play: ({ canvasElement }) => playParity('P-27', canvasElement),
};

export const P28: Story = {
  name: 'P-28 Multi-output node',
  args: { row: 'P-28' },
  play: ({ canvasElement }) => playParity('P-28', canvasElement),
};

export const P29: Story = {
  name: 'P-29 Trigger node',
  args: { row: 'P-29' },
  play: ({ canvasElement }) => playParity('P-29', canvasElement),
};

export const P30: Story = {
  name: 'P-30 Same experience in dark theme and by keyboard',
  args: { row: 'P-30' },
  play: ({ canvasElement }) => playParity('P-30', canvasElement),
};
