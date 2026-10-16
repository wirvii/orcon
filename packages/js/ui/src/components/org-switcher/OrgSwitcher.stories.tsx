// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { OrgSwitcher, type OrgSwitcherProps } from './OrgSwitcher';
import { states } from './OrgSwitcher.meta';

const onSelect = fn();
const organizations = [
  { id: 'w', name: 'Wirvii' },
  { id: 'q', name: 'Quantium Ingeniería' },
  { id: 'g', name: 'Guía Migratoria' },
];

const meta = {
  title: 'Components/OrgSwitcher',
  component: OrgSwitcher,
  args: { organizations, activeId: 'w', subtitle: 'Organization', onSelect, settingsHref: '#settings' },
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
} satisfies Meta<typeof OrgSwitcher>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The name of the active organization is always on the button, and the button is named after it.
    const button = within(canvasElement).getByRole('button', { name: 'Wirvii Organization' });
    await expect(button).toHaveTextContent('Wirvii');
    await userEvent.click(button);
    const menu = await within(document.body).findByRole('menu');
    await expect(within(menu).getByRole('menuitemcheckbox', { name: 'Wirvii' })).toBeChecked();
    // Creating an organization is not offered.
    await expect(within(menu).queryByText(/create/i)).toBeNull();
    await userEvent.click(within(menu).getByRole('menuitemcheckbox', { name: 'Guía Migratoria' }));
    await expect(onSelect).toHaveBeenCalledWith('g');
  },
};
export const Hover: Story = { parameters: { pseudo: { hover: true } } };
export const FocusVisible: Story = { parameters: { pseudo: { focusVisible: true } } };
export const Collapsed: Story = { args: { collapsed: true } };
export const Open: Story = { args: { defaultOpen: true } };
export const Single: Story = { args: { organizations: organizations.slice(0, 1), settingsHref: undefined, subtitle: undefined } };

const stateProps: Record<(typeof states)[number], Partial<OrgSwitcherProps>> = {
  default: {},
  hover: { className: 'pseudo-hover' },
  'focus-visible': { className: 'pseudo-focus-visible' },
  collapsed: { collapsed: true },
  open: { defaultOpen: true },
  single: { organizations: organizations.slice(0, 1), settingsHref: undefined, subtitle: undefined },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="OrgSwitcher states"
      columns={[...states]}
      rows={[
        {
          label: 'button',
          cells: states.map((state) => (
            <div key={state} className="border-border bg-surface w-panel-width rounded-md border p-2">
              <OrgSwitcher organizations={organizations} activeId="w" subtitle="Organization" onSelect={() => undefined} settingsHref="#settings" {...stateProps[state]} />
            </div>
          )),
        },
      ]}
    />
  ),
};
