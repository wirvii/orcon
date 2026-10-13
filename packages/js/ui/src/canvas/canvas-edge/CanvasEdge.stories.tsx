// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { EdgePreview } from '../../stories/CanvasStage';
import { MatrixTable } from '../../stories/MatrixTable';


const meta = {
  title: 'Canvas/CanvasEdge',
  component: EdgePreview,
} satisfies Meta<typeof EdgePreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: {} };
export const Design: Story = { args: { data: {} } };
export const Succeeded: Story = { args: { data: { state: 'succeeded', items: 3 } } };
export const Running: Story = { args: { data: { state: 'running', items: 3 } } };
export const Failed: Story = { args: { data: { state: 'failed', items: 0 } } };
export const Uncertain: Story = { args: { data: { state: 'uncertain' } } };
export const NotRun: Story = { args: { data: { state: 'not-run', items: 0 } } };
export const Active: Story = { args: { data: { state: 'succeeded', items: 1, active: true } } };
export const LoopBack: Story = { args: { loopBack: true, data: { state: 'succeeded', items: 2 } } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Connection states"
      columns={['preview']}
      rows={[
        { label: 'design time', cells: [<EdgePreview key="d" />] },
        { label: 'succeeded', cells: [<EdgePreview key="s" data={{ state: 'succeeded', items: 1 }} />] },
        { label: 'running', cells: [<EdgePreview key="r" data={{ state: 'running', items: 3 }} />] },
        { label: 'failed', cells: [<EdgePreview key="f" data={{ state: 'failed', items: 0 }} />] },
        { label: 'uncertain', cells: [<EdgePreview key="u" data={{ state: 'uncertain' }} />] },
        { label: 'not run', cells: [<EdgePreview key="n" data={{ state: 'not-run', items: 0 }} />] },
        { label: 'active', cells: [<EdgePreview key="a" data={{ state: 'succeeded', items: 1, active: true }} />] },
        { label: 'loop back', cells: [<EdgePreview key="l" loopBack data={{ state: 'succeeded', items: 2 }} />] },
      ]}
    />
  ),
};
