// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ComponentProps, ComponentType } from 'react';
import { useIntl } from 'react-intl';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { storyMessages as m } from '../../stories/messages';
import { Button } from '../button/Button';

import { AlertDialog } from './AlertDialog';

const onConfirm = fn();

type Args = Partial<ComponentProps<typeof AlertDialog>> & { short?: boolean; withTrigger?: boolean };

/** The alert dialog with its sample text from the message catalog, so the pseudo-locale lengthens it. */
function DeleteDialog({ short, withTrigger, ...props }: Args) {
  const intl = useIntl();
  return (
    <AlertDialog
      title={intl.formatMessage(m.deleteTitle)}
      impact={intl.formatMessage(short ? m.deleteShort : m.deleteImpact)}
      confirmLabel={intl.formatMessage(m.deleteFlow)}
      onConfirm={onConfirm}
      trigger={withTrigger ? <Button variant="danger">{intl.formatMessage(m.deleteFlow)}</Button> : undefined}
      {...props}
    />
  );
}

const meta: Meta<Args> = {
  title: 'Components/AlertDialog',
  component: AlertDialog as ComponentType<Args>,
  render: (args) => <DeleteDialog {...args} />,
};

export default meta;
type Story = StoryObj<Args>;

export const Default: Story = {
  args: { withTrigger: true },
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
      rows={[{ label: 'trigger', cells: [<DeleteDialog key="d" short withTrigger />] }]}
    />
  ),
};
