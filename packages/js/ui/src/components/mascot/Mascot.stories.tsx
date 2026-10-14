// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Mascot } from './Mascot';

const meta = {
  title: 'Components/Mascot',
  component: Mascot,
  parameters: {
    docs: {
      description: {
        component:
          'The approved cut-outs of the primary brand board on a soft rounded panel, light and dark theme. Decorative and lazy-loaded; allowed contexts only.',
      },
    },
  },
} satisfies Meta<typeof Mascot>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: { pose: 'welcome' } };
export const Welcome: Story = { args: { pose: 'welcome' } };
export const Happy: Story = { args: { pose: 'happy' } };
export const Thinking: Story = { args: { pose: 'thinking' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Mascot poses"
      columns={['welcome', 'happy', 'thinking']}
      rows={[
        {
          label: 'approved cut-out',
          cells: (['welcome', 'happy', 'thinking'] as const).map((pose) => <Mascot key={pose} pose={pose} />),
        },
      ]}
    />
  ),
};
