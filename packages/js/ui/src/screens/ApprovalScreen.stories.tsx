// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { ApprovalScreen } from './ApprovalScreen';
import { FIXTURE_NOW, approvalRequest } from './fixtures';

const meta = {
  title: 'Screens/S7 Approval',
  component: ApprovalScreen,
  parameters: { layout: 'fullscreen', screen: true },
} satisfies Meta<typeof ApprovalScreen>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('button', { name: 'Approve' })).toBeEnabled();
    await expect(view.getByText('in 55 minutes')).toBeVisible();
  },
};
export const Redacted: Story = {
  args: { argumentsHidden: true },
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByText('Hidden — you need permission to view execution data')).toBeVisible();
    await expect(canvasElement.textContent).not.toContain('pay_7Q2K');
    await expect(view.getByRole('button', { name: 'Edit arguments…' })).toBeDisabled();
  },
};
export const Denying: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await userEvent.click(view.getByRole('button', { name: 'Deny…' }));
    await expect(view.getByRole('textbox', { name: /Reason for denying/ })).toBeVisible();
  },
};
export const Editing: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole('button', { name: 'Edit arguments…' }));
    await expect(within(canvasElement).getByRole('textbox', { name: /Arguments \(JSON\)/ })).toBeVisible();
  },
};
export const InvalidArguments: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await userEvent.click(view.getByRole('button', { name: 'Edit arguments…' }));
    const field = view.getByRole('textbox', { name: /Arguments \(JSON\)/ });
    await userEvent.clear(field);
    await userEvent.type(field, '{{ not json');
    await userEvent.click(view.getByRole('button', { name: 'Approve with these arguments' }));
    // The message is next to the field and in the summary, and what was typed is still there.
    await expect(view.getAllByText('These arguments are not valid JSON. Fix the text and try again.').length).toBeGreaterThan(0);
    await expect(field).toHaveValue('{ not json');
  },
};
export const Expired: Story = { args: { request: { ...approvalRequest, expiresAt: '2026-10-12T14:00:00.000Z' }, now: FIXTURE_NOW } };
export const AlreadyResolved: Story = {
  args: {
    conflict: {
      problem: { status: 409, code: 'attention-already-resolved', title: 'Already resolved', requestId: 'req_8d3f1a' },
      outcome: { decision: 'approved', by: 'Ana Ruiz', at: '2026-10-12T14:57:00.000Z' },
    },
  },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText(/Approved by Ana Ruiz/)).toBeVisible();
    await expect(within(canvasElement).getByRole('button', { name: 'Approve' })).toBeDisabled();
  },
};

export const Matrix: Story = {};
