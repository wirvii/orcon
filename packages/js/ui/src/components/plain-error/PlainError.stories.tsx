// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { PlainError, type PlainErrorProps } from './PlainError';
import { states } from './PlainError.meta';

const base: PlainErrorProps = {
  title: 'WhatsApp refused 7 messages: the 24-hour window is closed.',
  detail: 'These customers have not written to you for more than a day. WhatsApp only lets you write to them with an approved template.',
  original: 'Error 131047: Re-engagement message\nMessage failed to send because more than 24 hours have passed since the customer last replied.',
};

const meta = {
  title: 'Components/PlainError',
  component: PlainError,
  args: base,
} satisfies Meta<typeof PlainError>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('heading', { name: /24-hour window/ })).toBeVisible();
    await expect(view.getByRole('region', { name: 'Original error' })).toBeVisible();
    // Nothing to apply: the fix is only text, never a button.
    await expect(view.queryByRole('button', { name: /editor/i })).toBeNull();
  },
};
export const TitleOnly: Story = { args: { detail: undefined, original: undefined } };
export const WithFix: Story = {
  args: { fix: 'Use the approved template when the last message of the customer is older than 24 hours.' },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText('How to fix it')).toBeVisible();
  },
};

const stateProps: Record<(typeof states)[number], Partial<PlainErrorProps>> = {
  default: {},
  'title-only': { detail: undefined, original: undefined },
  'with-fix': { fix: 'Use the approved template.' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="PlainError states"
      columns={[...states]}
      rows={[{ label: 'error', cells: states.map((state) => <PlainError key={state} {...base} {...stateProps[state]} />) }]}
    />
  ),
};
