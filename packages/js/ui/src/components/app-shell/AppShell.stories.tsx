// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Inbox, Play, Settings, Workflow } from 'lucide-react';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { BrandMark } from '../brand-mark/BrandMark';
import { EmptyState } from '../empty-state/EmptyState';
import { NotificationBell } from '../notification-bell/NotificationBell';

import { AppShell, type NavEntry } from './AppShell';

const navigation: NavEntry[] = [
  { id: 'flows', href: '#flows', label: 'Flows', icon: Workflow },
  { id: 'executions', href: '#executions', label: 'Executions', icon: Play },
  { id: 'inbox', href: '#inbox', label: 'Attention', icon: Inbox, badge: 3 },
  { id: 'settings', href: '#settings', label: 'Settings', icon: Settings },
];

const page = (
  <>
    <h1 className="text-xl font-semibold">Flows</h1>
    <EmptyState title="No flows yet" description="A flow connects a trigger to the steps that run after it." headingLevel={2} />
  </>
);

const meta = {
  title: 'Components/AppShell',
  component: AppShell,
  parameters: { layout: 'fullscreen' },
  args: {
    navigation,
    currentId: 'flows',
    brand: <BrandMark />,
    actions: <NotificationBell count={3} />,
    children: page,
  },
  decorators: [(Story, context) => (context.name === 'Matrix' ? <Story /> : <div className="-m-4"><Story /></div>)],
} satisfies Meta<typeof AppShell>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Desktop: sidebar with names. From md to lg it becomes a rail; below md a bottom bar. */
export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('banner')).toBeInTheDocument();
    await expect(canvas.getByRole('main')).toBeInTheDocument();
    await expect(canvas.getAllByRole('link', { name: 'Flows' })[0]).toHaveAttribute('aria-current', 'page');
  },
};
/** The drawer is how phones reach every section and the badges. */
export const Drawer: Story = {
  args: { defaultDrawerOpen: true },
  globals: { viewport: { value: 'mobile', isRotated: false } },
};
export const Badge: Story = { args: { currentId: 'inbox' } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="AppShell (the responsive layouts are shown by the viewport toolbar and the visual suite)"
      columns={['shell']}
      rows={[{ label: 'default', cells: [<div key="d" className="border-border h-node-width w-full overflow-hidden rounded-md border"><AppShell navigation={navigation} currentId="flows" brand={<BrandMark />}>{page}</AppShell></div>] }]}
    />
  ),
};
