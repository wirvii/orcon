// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { BrandMark } from '../brand-mark/BrandMark';
import { NotificationBell } from '../notification-bell/NotificationBell';

import { TopBar } from './TopBar';

const meta = {
  title: 'Components/TopBar',
  component: TopBar,
  args: { brand: <BrandMark /> },
  parameters: { layout: 'fullscreen' },
  decorators: [(Story, context) => (context.name === 'Matrix' ? <Story /> : <div className="-m-4"><Story /></div>)],
} satisfies Meta<typeof TopBar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('banner')).toBeInTheDocument();
  },
};
/** The menu button appears below the md breakpoint, where the sidebar is hidden. */
export const Menu: Story = { args: { onOpenNavigation: () => undefined }, globals: { viewport: { value: 'mobile', isRotated: false } } };
export const Actions: Story = { args: { actions: <NotificationBell count={3} /> } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="TopBar variants"
      columns={['bar']}
      rows={[
        { label: 'default', cells: [<div key="d" className="w-full min-w-panel-width"><TopBar brand={<BrandMark />} /></div>] },
        { label: 'menu', cells: [<div key="m" className="w-full min-w-panel-width"><TopBar brand={<BrandMark variant="symbol" />} onOpenNavigation={() => undefined} /></div>] },
        { label: 'actions', cells: [<div key="a" className="w-full min-w-panel-width"><TopBar brand={<BrandMark />} actions={<NotificationBell count={3} />} /></div>] },
      ]}
    />
  ),
};
