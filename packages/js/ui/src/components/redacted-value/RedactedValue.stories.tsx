// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ProtectedValue, RedactedValue } from './RedactedValue';

const meta = {
  title: 'Components/RedactedValue',
  component: RedactedValue,
} satisfies Meta<typeof RedactedValue>;

export default meta;
type Story = StoryObj<typeof meta>;

/** What a person without permission sees: a notice, never an empty cell. */
export const Default: Story = {
  render: () => <ProtectedValue value="sk-live-never-shown" hidden />,
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText(/you need permission to view execution data/)).toBeInTheDocument();
    // The value is not in the page at all.
    await expect(canvasElement.textContent).not.toContain('sk-live-never-shown');
    await expect(within(canvasElement).queryByRole('button')).toBeNull();
  },
};

export const Visible: Story = {
  render: () => <ProtectedValue value="order-48213" hidden={false} />,
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Protected values"
      columns={['default', 'visible']}
      rows={[
        {
          label: 'value',
          cells: [<ProtectedValue key="h" value="secret" hidden />, <ProtectedValue key="v" value="order-48213" hidden={false} />],
        },
      ]}
    />
  ),
};
