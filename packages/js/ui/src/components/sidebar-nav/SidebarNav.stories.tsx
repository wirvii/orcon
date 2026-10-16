// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Activity, CircleHelp, House, KeyRound, LayoutTemplate, Plus, Settings } from 'lucide-react';
import { useState } from 'react';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { Button } from '../button/Button';
import { OrgSwitcher } from '../org-switcher/OrgSwitcher';
import { UserMenu } from '../user-menu/UserMenu';

import { SidebarButton, SidebarLink, SidebarNav, useSidebarCollapsed, type SidebarNavProps, type SidebarSection } from './SidebarNav';
import { states } from './SidebarNav.meta';

const tile = (letter: string) => (
  <span aria-hidden className="bg-status-success-bg text-status-success inline-flex size-icon-20 shrink-0 items-center justify-center rounded-base text-2xs font-bold">
    {letter}
  </span>
);

const sections: SidebarSection[] = [
  { id: 'start', items: [{ id: 'home', label: 'Home', href: '#home', icon: House }] },
  {
    id: 'projects',
    label: 'Projects',
    items: [
      { id: 'sales', label: 'Ventas WhatsApp', href: '#sales', leading: tile('V'), count: 7 },
      { id: 'support', label: 'Soporte', href: '#support', leading: tile('S'), count: 4 },
    ],
  },
  {
    id: 'resources',
    label: 'Resources',
    items: [
      { id: 'templates', label: 'Templates', href: '#templates', icon: LayoutTemplate },
      { id: 'credentials', label: 'Credentials', href: '#credentials', icon: KeyRound },
      { id: 'executions', label: 'Executions', href: '#executions', icon: Activity, count: 2, attention: true },
    ],
  },
];

function Header() {
  const collapsed = useSidebarCollapsed();
  return (
    <OrgSwitcher
      organizations={[
        { id: 'w', name: 'Wirvii' },
        { id: 'q', name: 'Quantium' },
      ]}
      activeId="w"
      subtitle="Organization"
      collapsed={collapsed}
      onSelect={() => undefined}
      settingsHref="#org-settings"
    />
  );
}

function Primary() {
  const collapsed = useSidebarCollapsed();
  return (
    <Button className="w-full" aria-label={collapsed ? 'Create flow' : undefined}>
      <Plus aria-hidden className="size-icon-16" />
      {collapsed ? null : 'Create flow'}
    </Button>
  );
}

function Footer() {
  const collapsed = useSidebarCollapsed();
  return (
    <>
      <SidebarButton label="Help" icon={CircleHelp} />
      <SidebarLink item={{ id: 'settings', label: 'Settings', href: '#settings', icon: Settings }} />
      <div className="border-border mt-1 border-t pt-2">
        <UserMenu name="Juan Tamayo" initials="JT" email="juan.tamayo@wirvii.com" roleLabel="Owner" organizationName="Wirvii" items={[]} onSignOut={() => undefined} compact={collapsed} />
      </div>
    </>
  );
}

function Harness(props: Partial<SidebarNavProps>) {
  const [collapsed, setCollapsed] = useState(props.collapsed ?? false);
  return (
    <div className="flex h-screen">
      <SidebarNav label="Main navigation" sections={sections} currentId="home" header={<Header />} primary={<Primary />} footer={<Footer />} {...props} collapsed={collapsed} onCollapsedChange={setCollapsed} />
    </div>
  );
}

const meta = {
  title: 'Components/SidebarNav',
  component: SidebarNav,
  parameters: { layout: 'fullscreen' },
  args: { label: 'Main navigation', sections, collapsed: false, onCollapsedChange: () => undefined },
  render: (args) => <Harness {...args} />,
} satisfies Meta<typeof SidebarNav>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    // Only the destination that owns the address is marked.
    await expect(view.getByRole('link', { name: 'Soporte 4' })).not.toHaveAttribute('aria-current');
    // The bracket key collapses the sidebar to icons and every destination keeps its name.
    await userEvent.keyboard('[[');
    await expect(view.getByRole('navigation', { name: 'Main navigation' })).toHaveAttribute('data-collapsed', 'true');
    await expect(view.getByRole('link', { name: 'Home' })).toBeInTheDocument();
    await expect(view.getByRole('button', { name: 'Expand the sidebar' })).toBeInTheDocument();
    await userEvent.keyboard('[[');
    await expect(view.getByRole('button', { name: 'Collapse the sidebar' })).toBeInTheDocument();
  },
};
export const Collapsed: Story = { args: { collapsed: true } };
export const Current: Story = { args: { currentId: 'sales' } };
export const Attention: Story = { args: { currentId: 'executions' } };
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };

const stateProps: Record<(typeof states)[number], Partial<SidebarNavProps>> = {
  default: {},
  collapsed: { collapsed: true },
  current: { currentId: 'sales' },
  attention: { currentId: 'executions' },
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="SidebarNav states"
      columns={[...states]}
      rows={[
        {
          label: 'sidebar',
          cells: states.map((state) => (
            <div key={state} className="border-border flex h-screen overflow-hidden rounded-md border">
              <Harness {...stateProps[state]} />
            </div>
          )),
        },
      ]}
    />
  ),
};
