// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { Code, Keyboard, Sun, User } from 'lucide-react';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { UserMenu, type UserMenuProps } from './UserMenu';
import { states } from './UserMenu.meta';

const onSignOut = fn();

const items = [
  { id: 'profile', label: 'My profile', icon: User, href: '#profile' },
  { id: 'appearance', label: 'Appearance', icon: Sun, href: '#appearance' },
  { id: 'keys', label: 'API keys', icon: Code, href: '#keys' },
  { id: 'shortcuts', label: 'Keyboard shortcuts', icon: Keyboard, shortcut: '?' },
];

const meta = {
  title: 'Components/UserMenu',
  component: UserMenu,
  args: { name: 'Juan Tamayo', initials: 'JT', email: 'juan.tamayo@wirvii.com', roleLabel: 'Owner', organizationName: 'Wirvii', items, onSignOut },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="border-border bg-surface h-panel-width w-panel-width rounded-md border p-2">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof UserMenu>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The button shows the real name, the initials, the role and the organization, and is named after them.
    const button = within(canvasElement).getByRole('button', { name: 'Juan Tamayo Owner · Wirvii' });
    await expect(button).toHaveTextContent('JT');
    await userEvent.click(button);
    const menu = await within(document.body).findByRole('menu');
    await expect(within(menu).getByText('juan.tamayo@wirvii.com')).toBeVisible();
    await userEvent.click(within(menu).getByRole('menuitem', { name: 'Sign out' }));
    await expect(onSignOut).toHaveBeenCalled();
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Compact: Story = {
  args: { compact: true },
  play: async ({ canvasElement }) => {
    // The avatar alone is the button: it is named after the person and says what it opens.
    await expect(within(canvasElement).getByRole('button', { name: 'Juan Tamayo Account menu' })).toBeVisible();
  },
};
export const Open: Story = { args: { defaultOpen: true } };
export const WithoutRole: Story = { args: { roleLabel: undefined } };

const stateProps: Record<(typeof states)[number], Partial<UserMenuProps>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  compact: { compact: true },
  open: { defaultOpen: true },
  'without-role': { roleLabel: undefined },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="UserMenu states"
      columns={[...states]}
      rows={[
        {
          label: 'button',
          cells: states.map((state) => (
            <div key={state} className="border-border bg-surface w-panel-width rounded-md border p-2">
              <UserMenu name="Juan Tamayo" initials="JT" email="juan.tamayo@wirvii.com" roleLabel="Owner" organizationName="Wirvii" items={items} onSignOut={() => undefined} {...stateProps[state]} />
            </div>
          )),
        },
      ]}
    />
  ),
};
