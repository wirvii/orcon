// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Mascot } from './Mascot';

const meta = {
  title: 'Components/Mascot',
  component: Mascot,
  parameters: {
    docs: { description: { component: 'Neutral placeholder until the production mascot artwork is approved.' } },
  },
} satisfies Meta<typeof Mascot>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: { pose: 'welcome' } };
export const Thinking: Story = { args: { pose: 'thinking' } };
export const ThumbsUp: Story = { args: { pose: 'thumbs-up' } };
export const Explorer: Story = { args: { pose: 'explorer' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Mascot poses (placeholder frame)"
      columns={['welcome', 'thinking', 'thumbs-up', 'explorer']}
      rows={[
        {
          label: 'placeholder',
          cells: (['welcome', 'thinking', 'thumbs-up', 'explorer'] as const).map((pose) => <Mascot key={pose} pose={pose} />),
        },
      ]}
    />
  ),
};
