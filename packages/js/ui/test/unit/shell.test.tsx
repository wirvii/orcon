// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Inbox, Settings, Workflow } from 'lucide-react';
import { describe, expect, it, vi } from 'vitest';

import { AppShell, type NavEntry } from '../../src/components/app-shell/AppShell';
import { AttentionInboxItem } from '../../src/components/attention-inbox-item/AttentionInboxItem';
import { Combobox } from '../../src/components/combobox/Combobox';
import { CommandPalette } from '../../src/components/command-palette/CommandPalette';
import { NavItem } from '../../src/components/nav-item/NavItem';
import { NotificationBell } from '../../src/components/notification-bell/NotificationBell';
import { TopBar } from '../../src/components/top-bar/TopBar';

import { renderWithProviders } from './helpers';

describe('NavItem', () => {
  it('marks the current page with aria-current and a heavier weight', () => {
    renderWithProviders(<NavItem icon={Workflow} label="Flows" href="/flows" current />);
    const link = screen.getByRole('link', { name: 'Flows' });
    expect(link).toHaveAttribute('aria-current', 'page');
    expect(link.className).toContain('font-semibold');
  });

  it('keeps the name when collapsed to an icon', () => {
    renderWithProviders(<NavItem icon={Workflow} label="Flows" href="/flows" collapsed />);
    expect(screen.getByRole('link', { name: 'Flows' })).toBeInTheDocument();
    expect(screen.getByText('Flows')).toHaveClass('sr-only');
  });

  it('reads the badge after the name', () => {
    renderWithProviders(<NavItem icon={Inbox} label="Attention" href="/inbox" badge={3} />);
    expect(screen.getByRole('link', { name: 'Attention 3' })).toBeInTheDocument();
  });
});

describe('NotificationBell', () => {
  it('names the count in words, in both languages and with the right plural', () => {
    const { unmount } = renderWithProviders(<NotificationBell count={0} />);
    expect(screen.getByRole('button', { name: 'Nothing needs your attention' })).toBeInTheDocument();
    unmount();
    const one = renderWithProviders(<NotificationBell count={1} />);
    expect(screen.getByRole('button', { name: '1 item needs your attention' })).toBeInTheDocument();
    one.unmount();
    const many = renderWithProviders(<NotificationBell count={3} />);
    expect(screen.getByRole('button', { name: '3 items need your attention' })).toBeInTheDocument();
    many.unmount();
    renderWithProviders(<NotificationBell count={3} />, { locale: 'es' });
    expect(screen.getByRole('button', { name: '3 elementos requieren tu atención' })).toBeInTheDocument();
  });

  it('shows the same number that the name says, so the visible text is part of the name', () => {
    renderWithProviders(<NotificationBell count={142} />);
    expect(screen.getByRole('button')).toHaveTextContent('142');
    expect(screen.getByRole('button', { name: '142 items need your attention' })).toBeInTheDocument();
  });

  it('shows no badge without items', () => {
    renderWithProviders(<NotificationBell count={0} />);
    expect(screen.getByRole('button')).toHaveTextContent('');
  });

  it('calls onClick', async () => {
    const onClick = vi.fn();
    renderWithProviders(<NotificationBell count={2} onClick={onClick} />);
    await userEvent.click(screen.getByRole('button'));
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});

describe('AttentionInboxItem', () => {
  const props = {
    title: 'Approve the refund of order 48213',
    href: '/approvals/1',
    flowName: 'Refund review',
    createdAt: '2026-03-14T13:40:00.000Z',
    now: '2026-03-14T15:00:00.000Z',
  };

  it('is a named article with the cause as a link, the flow and the age', () => {
    renderWithProviders(<AttentionInboxItem {...props} />);
    const article = screen.getByRole('article', { name: props.title });
    expect(within(article).getByRole('link', { name: props.title })).toHaveAttribute('href', '/approvals/1');
    expect(within(article).getByText('Refund review')).toBeInTheDocument();
    expect(within(article).getByText('1 hour ago')).toBeInTheDocument();
  });

  it('shows escalation, who acknowledged and overdue as text, not only color', () => {
    renderWithProviders(<AttentionInboxItem {...props} escalationLevel={2} acknowledgedBy="Ada Lovelace" overdue />);
    expect(screen.getByText('Escalation level 2')).toBeInTheDocument();
    expect(screen.getByText('Acknowledged by Ada Lovelace')).toBeInTheDocument();
    expect(screen.getByText('Overdue')).toBeInTheDocument();
  });

  it('selects for a batch action with a checkbox named after the cause', async () => {
    const onSelectedChange = vi.fn();
    renderWithProviders(<AttentionInboxItem {...props} onSelectedChange={onSelectedChange} />);
    await userEvent.click(screen.getByRole('checkbox', { name: `Select ${props.title}` }));
    expect(onSelectedChange).toHaveBeenCalledWith(true);
  });

  it('has no checkbox unless selection is possible', () => {
    renderWithProviders(<AttentionInboxItem {...props} />);
    expect(screen.queryByRole('checkbox')).toBeNull();
  });

  it('is translated', () => {
    renderWithProviders(<AttentionInboxItem {...props} overdue escalationLevel={1} />, { locale: 'es' });
    expect(screen.getByText('Vencido')).toBeInTheDocument();
    expect(screen.getByText('Nivel de escalamiento 1')).toBeInTheDocument();
  });
});

describe('TopBar and AppShell', () => {
  const navigation: NavEntry[] = [
    { id: 'flows', href: '#flows', label: 'Flows', icon: Workflow },
    { id: 'inbox', href: '#inbox', label: 'Attention', icon: Inbox, badge: 3 },
    { id: 'settings', href: '#settings', label: 'Settings', icon: Settings },
  ];

  it('TopBar is the banner landmark and offers a menu button only when it can open navigation', async () => {
    const onOpenNavigation = vi.fn();
    const { unmount } = renderWithProviders(<TopBar brand="Orcon" />);
    expect(screen.getByRole('banner')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Open navigation menu' })).toBeNull();
    unmount();
    renderWithProviders(<TopBar brand="Orcon" onOpenNavigation={onOpenNavigation} />);
    await userEvent.click(screen.getByRole('button', { name: 'Open navigation menu' }));
    expect(onOpenNavigation).toHaveBeenCalledTimes(1);
  });

  it('AppShell has a skip link to the main landmark and marks the current section', () => {
    renderWithProviders(
      <AppShell navigation={navigation} currentId="inbox">
        <p>Page</p>
      </AppShell>,
    );
    expect(screen.getByRole('link', { name: 'Skip to main content' })).toHaveAttribute('href', '#main-content');
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main-content');
    for (const link of screen.getAllByRole('link', { name: /Attention/ })) {
      expect(link).toHaveAttribute('aria-current', 'page');
    }
  });

  it('AppShell opens the navigation drawer from the top bar and lists every section there', async () => {
    renderWithProviders(
      <AppShell navigation={navigation} currentId="flows">
        <p>Page</p>
      </AppShell>,
    );
    expect(screen.queryByRole('dialog')).toBeNull();
    await userEvent.click(screen.getByRole('button', { name: 'Open navigation menu' }));
    const drawer = await screen.findByRole('dialog', { name: 'Navigation' });
    expect(within(drawer).getAllByRole('link')).toHaveLength(3);
    await userEvent.click(within(drawer).getByRole('link', { name: 'Settings' }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('AppShell offers at most five sections in the bottom bar', () => {
    const many: NavEntry[] = Array.from({ length: 7 }, (_, index) => ({ id: `s${index}`, href: `/s${index}`, label: `Section ${index}`, icon: Workflow }));
    renderWithProviders(
      <AppShell navigation={many}>
        <p>Page</p>
      </AppShell>,
    );
    const bars = screen.getAllByRole('navigation', { hidden: true });
    const bottom = bars.find((bar) => bar.className.includes('bottom-0'));
    expect(bottom && within(bottom).getAllByRole('link', { hidden: true })).toHaveLength(5);
  });
});

describe('Combobox', () => {
  const options = [
    { value: 'slack', label: 'Slack' },
    { value: 'gmail', label: 'Gmail', keywords: ['email'] },
    { value: 'stripe', label: 'Stripe' },
  ];

  it('opens a searchable list, filters by label and keyword, and chooses with Enter', async () => {
    const onValueChange = vi.fn();
    renderWithProviders(<Combobox options={options} placeholder="Choose" searchLabel="Search connectors" aria-label="Connector" onValueChange={onValueChange} />);
    const trigger = screen.getByRole('combobox', { name: 'Connector' });
    expect(trigger).toHaveTextContent('Choose');
    await userEvent.click(trigger);
    await userEvent.type(await screen.findByRole('combobox', { name: 'Search connectors' }), 'email');
    expect(screen.getByRole('option', { name: 'Gmail' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Stripe' })).toBeNull();
    await userEvent.keyboard('{Enter}');
    expect(onValueChange).toHaveBeenCalledWith('gmail');
    expect(trigger).toHaveTextContent('Gmail');
  });

  it('says when nothing matches', async () => {
    renderWithProviders(<Combobox options={options} placeholder="Choose" searchLabel="Search connectors" aria-label="Connector" defaultOpen />);
    await userEvent.type(await screen.findByRole('combobox', { name: 'Search connectors' }), 'zzz');
    expect(screen.getByText('No matches')).toBeInTheDocument();
    expect(screen.queryByRole('listbox')).toBeNull();
  });

  it('marks the chosen option', () => {
    renderWithProviders(<Combobox options={options} placeholder="Choose" searchLabel="Search" aria-label="Connector" defaultValue="slack" defaultOpen />);
    expect(screen.getByRole('combobox', { name: 'Connector' })).toHaveTextContent('Slack');
  });
});

describe('CommandPalette', () => {
  const commands = [
    { id: 'new', label: 'Create a flow', group: 'Flows', shortcut: ['N'], onSelect: vi.fn() },
    { id: 'runs', label: 'Go to executions', group: 'Navigate', keywords: ['history'], onSelect: vi.fn() },
  ];

  it('opens with Ctrl+K, filters, runs the chosen command and closes', async () => {
    commands[1]!.onSelect.mockClear();
    renderWithProviders(<CommandPalette commands={commands} />);
    expect(screen.queryByRole('dialog')).toBeNull();
    await userEvent.keyboard('{Control>}k{/Control}');
    const search = await screen.findByRole('combobox', { name: 'Type a command or search' });
    await userEvent.type(search, 'history');
    expect(screen.queryByRole('option', { name: /Create a flow/ })).toBeNull();
    await userEvent.keyboard('{Enter}');
    expect(commands[1]!.onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('opens with Cmd+K and closes with Escape', async () => {
    renderWithProviders(<CommandPalette commands={commands} />);
    await userEvent.keyboard('{Meta>}k{/Meta}');
    expect(await screen.findByRole('dialog', { name: 'Command palette' })).toBeInTheDocument();
    await userEvent.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('can turn the shortcut off', async () => {
    renderWithProviders(<CommandPalette commands={commands} enableShortcut={false} />);
    await userEvent.keyboard('{Control>}k{/Control}');
    expect(screen.queryByRole('dialog')).toBeNull();
  });

  it('shows the shortcut keys of a command and groups under headings', () => {
    renderWithProviders(<CommandPalette commands={commands} defaultOpen />);
    expect(screen.getByText('N').tagName).toBe('KBD');
    expect(screen.getByText('Flows')).toBeInTheDocument();
    expect(screen.getByText('Navigate')).toBeInTheDocument();
  });

  it('says when no command matches', async () => {
    renderWithProviders(<CommandPalette commands={commands} defaultOpen defaultSearch="zzzz" />);
    expect(await screen.findByText('No matching commands')).toBeInTheDocument();
  });
});
