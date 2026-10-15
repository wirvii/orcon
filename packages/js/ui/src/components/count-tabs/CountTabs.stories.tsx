// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { CountTabs, type CountTab } from './CountTabs';
import { states } from './CountTabs.meta';

const tabs: CountTab[] = [
  { id: 'flows', label: 'Flows', href: '#flows' },
  { id: 'credentials', label: 'Credentials', href: '#credentials' },
  { id: 'executions', label: 'Executions', href: '#executions' },
];
const counted: CountTab[] = [
  { id: 'flows', label: 'Flows', count: 18, href: '#flows' },
  { id: 'credentials', label: 'Credentials', count: 9, href: '#credentials' },
  { id: 'executions', label: 'Executions', href: '#executions' },
];

const meta = {
  title: 'Components/CountTabs',
  component: CountTabs,
  args: { tabs, currentId: 'flows', label: 'Sections' },
} satisfies Meta<typeof CountTabs>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('link', { name: 'Flows' })).toHaveAttribute('aria-current', 'page');
    await expect(view.getByRole('link', { name: 'Credentials' })).not.toHaveAttribute('aria-current');
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Current: Story = { args: { currentId: 'credentials' } };
export const WithCounts: Story = { args: { tabs: counted } };

const stateProps: Record<(typeof states)[number], { tabs?: CountTab[]; currentId?: string; className?: string }> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  current: { currentId: 'credentials' },
  'with-counts': { tabs: counted },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="CountTabs states"
      columns={[...states]}
      rows={[{ label: 'tabs', cells: states.map((state) => <CountTabs key={state} tabs={tabs} currentId="flows" label={`Sections ${state}`} {...stateProps[state]} />) }]}
    />
  ),
};
