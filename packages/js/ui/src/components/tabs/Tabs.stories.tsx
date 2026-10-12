// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { Tabs, type TabItem } from './Tabs';
import { states } from './Tabs.meta';

const tabs: TabItem[] = [
  { value: 'overview', label: 'Overview', content: <p>Overview of the flow.</p> },
  { value: 'executions', label: 'Executions', content: <p>Recent executions.</p> },
  { value: 'settings', label: 'Settings', content: <p>Flow settings.</p> },
];

const meta = {
  title: 'Components/Tabs',
  component: Tabs,
  args: { tabs, label: 'Flow sections' },
} satisfies Meta<typeof Tabs>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    // Arrow keys move between tabs and select them.
    await userEvent.tab();
    await userEvent.keyboard('{ArrowRight}');
    await expect(canvas.getByRole('tab', { name: 'Executions' })).toHaveAttribute('aria-selected', 'true');
    await expect(canvas.getByRole('tabpanel')).toHaveTextContent('Recent executions.');
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Selected: Story = { args: { defaultValue: 'executions' } };
export const Disabled: Story = { args: { tabs: tabs.map((tab, index) => ({ ...tab, disabled: index === 2 })) } };

const tabClass: Record<(typeof states)[number], Partial<TabItem>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  selected: {},
  disabled: { disabled: true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Tab states, one tab each (selected is the first)"
      columns={['tabs']}
      rows={[
        {
          label: 'tab list',
          cells: [
            <Tabs
              key="t"
              label="Tab states"
              tabs={states.map((state) => ({ value: state, label: state, content: <p>{state}</p>, ...tabClass[state] }))}
              defaultValue="selected"
            />,
          ],
        },
      ]}
    />
  ),
};
