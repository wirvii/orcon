// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';
import { Button } from '../button/Button';
import { Mascot as MascotFrame } from '../mascot/Mascot';

import { EmptyState } from './EmptyState';
import { states } from './EmptyState.meta';

const meta = {
  title: 'Components/EmptyState',
  component: EmptyState,
  args: {
    title: 'No flows yet',
    description: 'A flow connects a trigger to the steps that run after it.',
  },
} satisfies Meta<typeof EmptyState>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Action: Story = { args: { action: <Button>Create a flow</Button> } };
export const Mascot: Story = { args: { mascot: <MascotFrame pose="thinking" />, action: <Button>Create a flow</Button> } };

const props: Record<(typeof states)[number], Partial<React.ComponentProps<typeof EmptyState>>> = {
  default: {},
  action: { action: <Button>Create a flow</Button> },
  mascot: { mascot: <MascotFrame pose="thinking" />, action: <Button>Create a flow</Button> },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="EmptyState variants"
      columns={[...states]}
      rows={[
        {
          label: 'empty state',
          cells: states.map((state) => (
            <div key={state} className="w-node-width">
              <EmptyState title="No flows yet" description="A flow connects a trigger to the steps that run after it." headingLevel={3} {...props[state]} />
            </div>
          )),
        },
      ]}
    />
  ),
};
