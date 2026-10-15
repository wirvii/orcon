// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Activity, House, Inbox, KeyRound, Play, Settings, Workflow } from 'lucide-react';
import { useState } from 'react';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { BottomTabBar } from '../bottom-tab-bar/BottomTabBar';
import { BrandMark } from '../brand-mark/BrandMark';
import { EmptyState } from '../empty-state/EmptyState';
import { NotificationBell } from '../notification-bell/NotificationBell';
import { SidebarNav, type SidebarSection } from '../sidebar-nav/SidebarNav';

import { AppShell, type NavEntry } from './AppShell';
import { PhoneTopBar } from './PhoneTopBar';

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

const productSections: SidebarSection[] = [
  { id: 'start', items: [{ id: 'home', label: 'Home', href: '#home', icon: House }] },
  {
    id: 'resources',
    label: 'Resources',
    items: [
      { id: 'credentials', label: 'Credentials', href: '#credentials', icon: KeyRound },
      { id: 'executions', label: 'Executions', href: '#executions', icon: Activity },
      { id: 'settings', label: 'Settings', href: '#settings', icon: Settings },
    ],
  },
];

function ProductShell({ fill = false }: { fill?: boolean }) {
  const [collapsed, setCollapsed] = useState(false);
  return (
    <AppShell
      fill={fill}
      sidebar={<SidebarNav label="Main navigation" sections={productSections} currentId="home" collapsed={collapsed} onCollapsedChange={setCollapsed} />}
      phoneBar={<PhoneTopBar context="Wirvii" title="Home" />}
      tabBar={
        <BottomTabBar
          label="Main navigation"
          currentId="home"
          tabs={productSections.flatMap((section) => section.items).map((item) => ({ id: item.id, label: item.label, href: item.href, icon: item.icon ?? House }))}
        />
      }
    >
      {page}
    </AppShell>
  );
}

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
/** The product frame: the sidebar from md up; on a phone the top bar and the bottom tab bar. */
export const Product: Story = {
  render: () => <ProductShell />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByRole('main')).toBeInTheDocument();
    await expect(canvas.getAllByRole('navigation', { name: 'Main navigation' }).length).toBeGreaterThan(0);
  },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="AppShell (the responsive layouts are shown by the viewport toolbar and the visual suite)"
      columns={['shell']}
      rows={[{ label: 'default', cells: [<div key="d" className="border-border h-panel-width w-full overflow-hidden rounded-md border"><AppShell navigation={navigation} currentId="flows" brand={<BrandMark />}>{page}</AppShell></div>] }]}
    />
  ),
};
