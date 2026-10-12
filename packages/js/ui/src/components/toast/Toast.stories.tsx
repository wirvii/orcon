// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useEffect } from 'react';
import { useIntl } from 'react-intl';
import { toast } from 'sonner';
import { expect, waitFor, within } from 'storybook/test';

import { storyMessages as m } from '../../stories/messages';

import { Toaster } from './Toast';
import { states } from './Toast.meta';

type State = (typeof states)[number];

/** Shows the toasts of the given states once, with text from the message catalog, and removes them on unmount (the toast store is global to the page). */
function ShowToasts({ shown }: { shown: readonly State[] }) {
  const intl = useIntl();
  useEffect(() => {
    const options = { duration: Number.POSITIVE_INFINITY };
    for (const state of shown) {
      const id = { id: state };
      if (state === 'success') toast.success(intl.formatMessage(m.toastSaved), { ...options, ...id });
      else if (state === 'warning')
        toast.warning(intl.formatMessage(m.toastWarning), { ...options, ...id, description: intl.formatMessage(m.toastWarningText) });
      else if (state === 'danger') toast.error(intl.formatMessage(m.toastDanger), { ...options, ...id });
      else if (state === 'action')
        toast(intl.formatMessage(m.toastAction), { ...options, ...id, action: { label: intl.formatMessage(m.toastUndo), onClick: () => undefined } });
      else toast(intl.formatMessage(m.toastDefault), { ...options, ...id });
    }
    return () => void toast.dismiss();
  }, [intl, shown]);
  return null;
}

function Note({ message }: { message: typeof m.toastNote }) {
  const intl = useIntl();
  return <p className="text-fg-muted">{intl.formatMessage(message)}</p>;
}

const single = (state: State): Story => ({
  render: () => (
    <>
      <Toaster />
      <ShowToasts shown={[state]} />
      <Note message={m.toastNote} />
    </>
  ),
});

const meta = {
  title: 'Components/Toast',
  component: Toaster,
} satisfies Meta<typeof Toaster>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  ...single('default'),
  play: async () => {
    const message = await within(document.body).findByText('Link copied');
    // The toast fades in after it mounts.
    await waitFor(() => expect(message).toBeVisible());
  },
};
export const Success: Story = single('success');
export const Warning: Story = single('warning');
export const Danger: Story = single('danger');
export const Action: Story = single('action');

/** All five toasts at once; the stack shows every one of them. */
export const Matrix: Story = {
  render: () => (
    <>
      <Toaster visibleToasts={states.length} />
      <ShowToasts shown={states} />
      <Note message={m.toastMatrix} />
    </>
  ),
};
