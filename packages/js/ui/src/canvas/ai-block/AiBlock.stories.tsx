// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import type { AiBlockKind } from '../model';

import { AiBlock } from './AiBlock';

const meta = {
  title: 'Canvas/AiBlock',
  component: AiBlock,
  args: { kind: 'model', set: true, detail: 'Language model' },
  decorators: [
    (Story) => (
      <div className="bg-canvas inline-block rounded-lg p-6">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AiBlock>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('button', { name: 'Model: Language model. Opens the agent settings.' })).toBeVisible();
  },
};
export const Unset: Story = { args: { set: false } };
export const Hover: Story = { args: { className: 'pseudo-hover' } };
export const FocusVisible: Story = { args: { className: 'pseudo-focus-visible' } };

const kinds: AiBlockKind[] = ['model', 'memory', 'tools', 'parser', 'retrieval'];

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Agent blocks"
      columns={kinds}
      rows={[
        { label: 'set', cells: kinds.map((kind) => <div key={kind} className="bg-canvas rounded-lg p-3"><AiBlock kind={kind} set detail={kind} /></div>) },
        { label: 'unset', cells: kinds.map((kind) => <div key={kind} className="bg-canvas rounded-lg p-3"><AiBlock kind={kind} set={false} /></div>) },
      ]}
    />
  ),
};
