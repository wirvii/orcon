// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { Button } from '../button/Button';

import { AlertDialog } from './AlertDialog';

const onConfirm = fn();

const meta = {
  title: 'Components/AlertDialog',
  component: AlertDialog,
  args: {
    title: 'Delete this flow?',
    impact: 'The flow and its schedule are removed. Past executions stay in the history. This cannot be undone.',
    confirmLabel: 'Delete flow',
    onConfirm,
  },
} satisfies Meta<typeof AlertDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: { trigger: <Button variant="danger">Delete flow</Button> },
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole('button', { name: 'Delete flow' }));
    const dialog = await within(document.body).findByRole('alertdialog', { name: 'Delete this flow?' });
    // The impact text is the description; focus starts on Cancel so Enter does not confirm.
    await expect(dialog).toHaveAccessibleDescription(/This cannot be undone/);
    await expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus();
    await userEvent.keyboard('{Escape}');
    await expect(onConfirm).not.toHaveBeenCalled();
  },
};

export const Open: Story = { args: { defaultOpen: true } };
export const Destructive: Story = { args: { defaultOpen: true, destructive: true } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="AlertDialog (the open states are their own stories)"
      columns={['default']}
      rows={[
        {
          label: 'trigger',
          cells: [
            <AlertDialog
              key="d"
              title="Delete this flow?"
              impact="This cannot be undone."
              confirmLabel="Delete flow"
              onConfirm={() => undefined}
              trigger={<Button variant="danger">Delete flow</Button>}
            />,
          ],
        },
      ]}
    />
  ),
};
