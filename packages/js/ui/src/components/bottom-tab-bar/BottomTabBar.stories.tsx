// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Activity, FolderKanban, House, KeyRound, Settings } from 'lucide-react';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { BottomTabBar, type BottomTab } from './BottomTabBar';
import { states } from './BottomTabBar.meta';

const tabs: BottomTab[] = [
  { id: 'home', label: 'Home', href: '#home', icon: House },
  { id: 'flows', label: 'Flows', href: '#flows', icon: FolderKanban },
  { id: 'executions', label: 'Executions', href: '#executions', icon: Activity },
  { id: 'credentials', label: 'Credentials', href: '#credentials', icon: KeyRound },
  { id: 'settings', label: 'Settings', href: '#settings', icon: Settings },
];

const meta = {
  title: 'Components/BottomTabBar',
  component: BottomTabBar,
  args: { tabs, currentId: 'home', label: 'Main navigation' },
  decorators: [(Story) => <div className="w-full max-w-screen-sm"><Story /></div>],
} satisfies Meta<typeof BottomTabBar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const nav = within(canvasElement).getByRole('navigation', { name: 'Main navigation' });
    // Five destinations, each with its name as visible text.
    await expect(within(nav).getAllByRole('link')).toHaveLength(5);
    for (const tab of tabs) await expect(within(nav).getByRole('link', { name: tab.label })).toBeVisible();
    await expect(within(nav).getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Current: Story = { args: { currentId: 'executions' } };

const stateProps: Record<(typeof states)[number], { currentId?: string; className?: string }> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  current: { currentId: 'executions' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="BottomTabBar states"
      columns={[...states]}
      rows={[{ label: 'bar', cells: states.map((state) => <div key={state} className="w-panel-width"><BottomTabBar tabs={tabs} currentId="home" label={`Main navigation ${state}`} {...stateProps[state]} /></div>) }]}
    />
  ),
};
