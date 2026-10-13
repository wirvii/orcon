// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { Button } from '../button/Button';

import { Popover } from './Popover';

const meta = {
  title: 'Components/Popover',
  component: Popover,
  args: {
    label: 'Run settings',
    trigger: <Button variant="secondary">Run settings</Button>,
    children: <p>Retries apply to every item that fails with a temporary error.</p>,
  },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="h-panel-width">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof Popover>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole('button', { name: 'Run settings' }));
    await expect(await within(document.body).findByRole('dialog', { name: 'Run settings' })).toBeInTheDocument();
    // Escape closes it and returns focus to the trigger.
    await userEvent.keyboard('{Escape}');
    await expect(within(canvasElement).getByRole('button', { name: 'Run settings' })).toHaveFocus();
  },
};

export const Open: Story = { args: { defaultOpen: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Popover (open)"
      columns={['default', 'open']}
      rows={[
        {
          label: 'popover',
          cells: [
            <div key="d" className="h-12">
              <Popover label="Closed popover" trigger={<Button variant="secondary">Closed</Button>}>
                <p>Content</p>
              </Popover>
            </div>,
            <div key="o" className="h-panel-width">
              <Popover label="Open popover" defaultOpen trigger={<Button variant="secondary">Open</Button>}>
                <p>Content</p>
              </Popover>
            </div>,
          ],
        },
      ]}
    />
  ),
};
