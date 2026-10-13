// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { Pagination } from './Pagination';

const meta = {
  title: 'Components/Pagination',
  component: Pagination,
  args: { page: 2, pageCount: 10, onPageChange: () => undefined },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="max-w-screen-sm">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof Pagination>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: function Render(args) {
    const [page, setPage] = useState(args.page);
    return <Pagination {...args} page={page} onPageChange={setPage} />;
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByText('Page 2 of 10')).toBeInTheDocument();
    await userEvent.click(canvas.getByRole('button', { name: 'Next' }));
    await expect(canvas.getByText('Page 3 of 10')).toBeInTheDocument();
  },
};
export const First: Story = { args: { page: 1 } };
export const Last: Story = { args: { page: 10 } };
export const Single: Story = { args: { page: 1, pageCount: 1 } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Pagination states"
      columns={['pagination']}
      rows={[
        { label: 'default', cells: [<div key="d" className="w-panel-width"><Pagination page={2} pageCount={10} onPageChange={() => undefined} /></div>] },
        { label: 'first', cells: [<div key="f" className="w-panel-width"><Pagination page={1} pageCount={10} onPageChange={() => undefined} /></div>] },
        { label: 'last', cells: [<div key="l" className="w-panel-width"><Pagination page={10} pageCount={10} onPageChange={() => undefined} /></div>] },
        { label: 'single', cells: [<div key="s" className="w-panel-width"><Pagination page={1} pageCount={1} onPageChange={() => undefined} /></div>] },
      ]}
    />
  ),
};
