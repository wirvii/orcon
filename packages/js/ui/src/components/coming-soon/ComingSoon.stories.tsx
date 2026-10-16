// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Mail } from 'lucide-react';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ComingSoon } from './ComingSoon';

const meta = {
  title: 'Components/ComingSoon',
  component: ComingSoon,
  args: { title: 'Templates', description: 'Ready-made flows are not available yet. For now, start from a blank flow or import one from n8n.' },
} satisfies Meta<typeof ComingSoon>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('heading', { name: 'Templates' })).toBeVisible();
    await expect(view.getByText('Coming soon')).toBeVisible();
    // There is nothing to press: no button and no link.
    await expect(view.queryByRole('button')).toBeNull();
    await expect(view.queryByRole('link')).toBeNull();
  },
};

export const CustomIcon: Story = { args: { icon: Mail, title: 'Email sign-in reset', description: 'Resetting a password by email needs the email service, which is not part of the server yet.' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ComingSoon"
      columns={['default', 'custom icon']}
      rows={[
        {
          label: 'card',
          cells: [
            <ComingSoon key="a" title="Templates" description="Ready-made flows are not available yet." />,
            <ComingSoon key="b" icon={Mail} title="Password reset" description="Resetting by email needs the email service." />,
          ],
        },
      ]}
    />
  ),
};
