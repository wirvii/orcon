// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Info } from 'lucide-react';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { IconButton } from '../icon-button/IconButton';

import { Tooltip } from './Tooltip';

const meta = {
  title: 'Components/Tooltip',
  component: Tooltip,
  args: { content: 'Shown in the flow list', children: <IconButton variant="secondary" icon={Info} label="About the name" /> },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="flex h-16 items-center justify-center pt-10">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof Tooltip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // Keyboard focus shows the tooltip as well as hover does.
    await userEvent.tab();
    await expect(await within(document.body).findByRole('tooltip')).toHaveTextContent('Shown in the flow list');
    await expect(within(canvasElement).getByRole('button', { name: 'About the name' })).toHaveAccessibleDescription('Shown in the flow list');
  },
};

export const Open: Story = { args: { defaultOpen: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Tooltip (open)"
      columns={['default', 'open']}
      rows={[
        {
          label: 'tooltip',
          cells: [
            <div key="d" className="h-12 pt-8">
              <Tooltip content="Shown in the flow list">
                <IconButton variant="secondary" icon={Info} label="About the name, closed" />
              </Tooltip>
            </div>,
            <div key="o" className="h-12 pt-8">
              <Tooltip content="Shown in the flow list" defaultOpen>
                <IconButton variant="secondary" icon={Info} label="About the name, open" />
              </Tooltip>
            </div>,
          ],
        },
      ]}
    />
  ),
};
