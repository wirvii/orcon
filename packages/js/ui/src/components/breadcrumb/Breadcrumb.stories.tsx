// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { Breadcrumb } from './Breadcrumb';

const meta = {
  title: 'Components/Breadcrumb',
  component: Breadcrumb,
  args: {
    items: [
      { label: 'Flows', href: '#flows' },
      { label: 'Nightly sync', href: '#flow' },
      { label: 'Execution 4821' },
    ],
  },
} satisfies Meta<typeof Breadcrumb>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('navigation', { name: 'Breadcrumb' })).toBeInTheDocument();
    await expect(canvas.getByText('Execution 4821')).toHaveAttribute('aria-current', 'page');
  },
};
export const Single: Story = { args: { items: [{ label: 'Flows' }] } };
export const Long: Story = {
  args: {
    items: [
      { label: 'Flows', href: '#flows' },
      { label: 'Customer onboarding and welcome sequence', href: '#flow' },
      { label: 'Executions', href: '#executions' },
      { label: 'Execution 4821 with an unusually long identifier', href: '#execution' },
      { label: 'Node: send welcome email to every new customer' },
    ],
  },
  decorators: [(Story) => <div className="max-w-screen-sm"><Story /></div>],
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Breadcrumb"
      columns={['trail']}
      rows={[
        {
          label: 'default',
          cells: [<Breadcrumb key="d" items={[{ label: 'Flows', href: '#flows' }, { label: 'Nightly sync' }]} />],
        },
        { label: 'single', cells: [<Breadcrumb key="s" items={[{ label: 'Flows' }]} />] },
      ]}
    />
  ),
};
