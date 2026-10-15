// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { fireEvent, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Activity, FolderKanban, House, KeyRound, Settings } from 'lucide-react';
import { useState } from 'react';
import { expect, it, vi } from 'vitest';

import { AppShell } from '../../src/components/app-shell/AppShell';
import { ActiveToggle } from '../../src/components/active-toggle/ActiveToggle';
import { BottomTabBar } from '../../src/components/bottom-tab-bar/BottomTabBar';
import { ComingSoon } from '../../src/components/coming-soon/ComingSoon';
import { ConnectorIcon } from '../../src/components/connector-icon/ConnectorIcon';
import { Link } from '../../src/components/link/Link';
import { SidebarNav } from '../../src/components/sidebar-nav/SidebarNav';
import { StatusBadge } from '../../src/components/status-badge/StatusBadge';
import { UserMenu } from '../../src/components/user-menu/UserMenu';

import { renderWithProviders } from './helpers';

// The components of the app frame (SPEC-019 UX1). The titles are the ones the acceptance criteria name.

it('Link plain variant is not underlined', () => {
  renderWithProviders(<Link href="#docs">Documentation</Link>);
  const link = screen.getByRole('link', { name: 'Documentation' });
  expect(link).toHaveAttribute('data-variant', 'plain');
  expect(link.className).not.toMatch(/(^|\s)underline(\s|$)/);
});

it('Link prose variant is underlined', () => {
  renderWithProviders(
    <p data-prose>
      Read the{' '}
      <Link href="#docs" variant="prose" inText>
        security notes
      </Link>
      .
    </p>,
  );
  const link = screen.getByRole('link', { name: 'security notes' });
  expect(link).toHaveAttribute('data-variant', 'prose');
  expect(link.className).toMatch(/(^|\s)underline(\s|$)/);
});

it('StatusBadge uncertain shows Needs review with icon and text', () => {
  const { container } = renderWithProviders(<StatusBadge status="UNCERTAIN" />);
  const badge = container.querySelector('[data-status="UNCERTAIN"]');
  expect(badge).not.toBeNull();
  expect(badge?.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  expect(badge).toHaveTextContent('Needs review');
  // The long description for assistive technology does not change.
  expect(badge).toHaveTextContent('The external system may or may not have applied this operation.');
});

it('StatusBadge uncertain is never styled as danger or success', () => {
  const { container } = renderWithProviders(<StatusBadge status="UNCERTAIN" />);
  const classes = container.querySelector('[data-status]')?.className ?? '';
  expect(classes).toContain('status-attention');
  expect(classes).not.toMatch(/status-(danger|success)/);
});

it('ActiveToggle is disabled with a reason when never published', async () => {
  const onChange = vi.fn();
  renderWithProviders(<ActiveToggle checked={false} label="Sales agent" disabledReason="Publish the flow to be able to activate it" onCheckedChange={onChange} />);
  const toggle = screen.getByRole('switch', { name: 'Sales agent' });
  expect(toggle).toHaveAttribute('aria-disabled', 'true');
  // The reason is read with the switch.
  expect(toggle).toHaveAccessibleDescription('Publish the flow to be able to activate it');
  await userEvent.click(toggle);
  expect(onChange).not.toHaveBeenCalled();
  expect(toggle).not.toBeChecked();
});

it('ActiveToggle shows busy and does not flip before the server answers', async () => {
  const onChange = vi.fn();
  function Harness() {
    const [state, setState] = useState<{ checked: boolean; busy: boolean }>({ checked: false, busy: false });
    return (
      <ActiveToggle
        checked={state.checked}
        busy={state.busy}
        label="Sales agent"
        onCheckedChange={(next) => {
          onChange(next);
          // The request is in flight: the server has not answered.
          setState({ checked: false, busy: true });
        }}
      />
    );
  }
  renderWithProviders(<Harness />);
  const toggle = screen.getByRole('switch', { name: 'Sales agent' });
  await userEvent.click(toggle);
  expect(onChange).toHaveBeenCalledWith(true);
  expect(toggle).toHaveAttribute('aria-busy', 'true');
  expect(toggle).not.toBeChecked();
  expect(screen.getByText('Updating')).toBeVisible();
  // A second click while it is busy asks for nothing.
  await userEvent.click(toggle);
  expect(onChange).toHaveBeenCalledTimes(1);
});

it('ComingSoon renders icon title and sentence without actions', () => {
  const { container } = renderWithProviders(<ComingSoon title="Templates" description="Ready-made flows are not available yet." />);
  expect(container.querySelector('svg')).toHaveAttribute('aria-hidden', 'true');
  expect(screen.getByRole('heading', { name: 'Templates' })).toBeVisible();
  expect(screen.getByText('Ready-made flows are not available yet.')).toBeVisible();
  expect(screen.getByText('Coming soon')).toBeVisible();
  expect(screen.queryByRole('button')).toBeNull();
  expect(screen.queryByRole('link')).toBeNull();
});

it('ConnectorIcon falls back to the Plug glyph with the connector name', () => {
  renderWithProviders(<ConnectorIcon name="Acme CRM" />);
  const glyph = screen.getByRole('img', { name: 'Acme CRM' });
  expect(glyph.tagName.toLowerCase()).toBe('svg');
  expect(glyph).toHaveAttribute('data-connector-fallback');
  // A logo that does not load falls back the same way.
  renderWithProviders(<ConnectorIcon name="Broken" src="/missing.svg" />);
  const image = screen.getByRole('img', { name: 'Broken' });
  expect(image.tagName.toLowerCase()).toBe('img');
  fireEvent.error(image);
  expect(screen.getByRole('img', { name: 'Broken' }).tagName.toLowerCase()).toBe('svg');
});

it('ConnectorIcon uses the dark variant in the dark theme when present', () => {
  const light = '/icons/openai.svg';
  const dark = '/icons/openai-dark.svg';
  const view = renderWithProviders(<ConnectorIcon name="OpenAI" src={light} darkSrc={dark} />, { theme: 'dark' });
  expect(screen.getByRole('img', { name: 'OpenAI' })).toHaveAttribute('src', dark);
  view.unmount();
  renderWithProviders(<ConnectorIcon name="OpenAI" src={light} darkSrc={dark} />, { theme: 'light' });
  expect(screen.getByRole('img', { name: 'OpenAI' })).toHaveAttribute('src', light);
});

function SidebarHarness() {
  const [collapsed, setCollapsed] = useState(false);
  return (
    <>
      <SidebarNav
        label="Main navigation"
        collapsed={collapsed}
        onCollapsedChange={setCollapsed}
        currentId="home"
        sections={[
          { id: 'start', items: [{ id: 'home', label: 'Home', href: '#home', icon: House }] },
          { id: 'resources', label: 'Resources', items: [{ id: 'executions', label: 'Executions', href: '#executions', icon: Activity, count: 2, attention: true }] },
        ]}
      />
      <input aria-label="Search" />
    </>
  );
}

it('SidebarNav collapses with the bracket key and keeps accessible names', async () => {
  renderWithProviders(<SidebarHarness />);
  const nav = screen.getByRole('navigation', { name: 'Main navigation' });
  expect(nav).not.toHaveAttribute('data-collapsed');
  expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
  expect(screen.getByRole('link', { name: 'Executions 2' })).not.toHaveAttribute('aria-current');

  await userEvent.keyboard('[[');
  expect(nav).toHaveAttribute('data-collapsed', 'true');
  // Names stay available while only icons are drawn.
  expect(screen.getByRole('link', { name: 'Home' })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Executions 2' })).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Expand the sidebar' })).toHaveAttribute('aria-expanded', 'false');

  // Typing a bracket into a field is not the shortcut.
  await userEvent.click(screen.getByLabelText('Search'));
  await userEvent.keyboard('[[');
  expect(nav).toHaveAttribute('data-collapsed', 'true');

  await userEvent.click(screen.getByRole('button', { name: 'Expand the sidebar' }));
  expect(nav).not.toHaveAttribute('data-collapsed');
});

it('UserMenu shows the display name initials role and organization', async () => {
  const onSignOut = vi.fn();
  renderWithProviders(
    <UserMenu name="Juan Tamayo" initials="JT" email="juan.tamayo@wirvii.com" roleLabel="Owner" organizationName="Wirvii" items={[{ id: 'profile', label: 'My profile', href: '#profile' }]} onSignOut={onSignOut} />,
  );
  // The name of the button is what is drawn on it: the real name, then the role and the organization.
  const button = screen.getByRole('button', { name: 'Juan Tamayo Owner · Wirvii' });
  expect(button).toHaveTextContent('JT');
  expect(button).toHaveTextContent('Juan Tamayo');
  expect(button).toHaveTextContent('Owner · Wirvii');

  await userEvent.click(button);
  const menu = await screen.findByRole('menu');
  expect(within(menu).getByText('juan.tamayo@wirvii.com')).toBeVisible();
  expect(within(menu).getByRole('menuitem', { name: 'My profile' })).toHaveAttribute('href', '#profile');
  await userEvent.click(within(menu).getByRole('menuitem', { name: 'Sign out' }));
  expect(onSignOut).toHaveBeenCalled();
});

it('BottomTabBar shows five labelled destinations', () => {
  const tabs = [
    { id: 'home', label: 'Home', href: '#home', icon: House },
    { id: 'flows', label: 'Flows', href: '#flows', icon: FolderKanban },
    { id: 'executions', label: 'Executions', href: '#executions', icon: Activity },
    { id: 'credentials', label: 'Credentials', href: '#credentials', icon: KeyRound },
    { id: 'settings', label: 'Settings', href: '#settings', icon: Settings },
  ];
  renderWithProviders(<BottomTabBar label="Main navigation" tabs={tabs} currentId="flows" />);
  const nav = screen.getByRole('navigation', { name: 'Main navigation' });
  const links = within(nav).getAllByRole('link');
  expect(links).toHaveLength(5);
  for (const tab of tabs) expect(within(nav).getByRole('link', { name: tab.label })).toBeVisible();
  expect(within(nav).getByRole('link', { name: 'Flows' })).toHaveAttribute('aria-current', 'page');
});

it('AppShell without sections draws no sidebar, no bottom bar and no menu button', () => {
  renderWithProviders(
    <AppShell brand={<span>Orcon</span>}>
      <p>A page outside an organization</p>
    </AppShell>,
  );
  expect(screen.queryByRole('navigation')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Open navigation menu' })).toBeNull();
  expect(screen.getByRole('main')).toHaveTextContent('A page outside an organization');
});
