// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useEffect } from 'react';
import { toast } from 'sonner';
import { expect, waitFor, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';


import { Toaster } from './Toast';
import { states } from './Toast.meta';

/** The toast store is global to the page, so a story removes its toasts when it unmounts. */
function DismissOnUnmount() {
  useEffect(() => () => void toast.dismiss(), []);
  return null;
}

const meta = {
  title: 'Components/Toast',
  component: Toaster,
  decorators: [
    (Story) => (
      <>
        <Story />
        <DismissOnUnmount />
        <p className="text-fg-muted">Toasts appear in the bottom corner and never take focus.</p>
      </>
    ),
  ],
} satisfies Meta<typeof Toaster>;

export default meta;
type Story = StoryObj<typeof meta>;

const show = (state: (typeof states)[number]) => {
  const options = { duration: Number.POSITIVE_INFINITY };
  if (state === 'success') toast.success('Flow saved', options);
  else if (state === 'warning') toast.warning('Saved with a warning', { ...options, description: 'One node has no output.' });
  else if (state === 'danger') toast.error('Could not copy the link', options);
  else if (state === 'action') toast('Flow deleted', { ...options, action: { label: 'Undo', onClick: () => undefined } });
  else toast('Link copied', options);
};

export const Default: Story = {
  play: async () => {
    show('default');
    const message = await within(document.body).findByText('Link copied');
    // The toast fades in after it mounts.
    await waitFor(() => expect(message).toBeVisible());
  },
};
export const Success: Story = { play: async () => show('success') };
export const Warning: Story = { play: async () => show('warning') };
export const Danger: Story = { play: async () => show('danger') };
export const Action: Story = { play: async () => show('action') };

export const Matrix: Story = {
  play: async () => {
    for (const state of states) show(state);
  },
  render: () => (
    <MatrixTable
      caption="Toast states (shown together in the corner)"
      columns={['states']}
      rows={[{ label: 'toasts', cells: [<Toaster key="t" />] }]}
    />
  ),
};
