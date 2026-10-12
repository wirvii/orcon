// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Workflow } from 'lucide-react';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { NavItem } from './NavItem';
import { states } from './NavItem.meta';

const meta = {
  title: 'Components/NavItem',
  component: NavItem,
  args: { icon: Workflow, label: 'Flows', href: '#flows' },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <nav aria-label="Example" className="bg-surface w-node-width rounded-md p-2">
          <Story />
        </nav>
      ),
  ],
} satisfies Meta<typeof NavItem>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Current: Story = {
  args: { current: true },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('link', { name: 'Flows' })).toHaveAttribute('aria-current', 'page');
  },
};
/** Icon only. The name stays available to assistive technology and as a tooltip. */
export const Collapsed: Story = {
  args: { collapsed: true },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('link', { name: 'Flows' })).toBeInTheDocument();
  },
};
export const Badge: Story = {
  args: { badge: 3 },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('link', { name: /Flows\s*3/ })).toBeInTheDocument();
  },
};

const stateProps: Record<(typeof states)[number], Partial<React.ComponentProps<typeof NavItem>>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  current: { current: true },
  collapsed: { collapsed: true },
  badge: { badge: 3 },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="NavItem states"
      columns={[...states]}
      rows={[
        {
          label: 'item',
          cells: states.map((state) => (
            <nav key={state} aria-label={`Navigation, ${state}`} className="bg-surface w-node-width rounded-md p-2">
              <NavItem icon={Workflow} label="Flows" href="#flows" {...stateProps[state]} />
            </nav>
          )),
        },
      ]}
    />
  ),
};
