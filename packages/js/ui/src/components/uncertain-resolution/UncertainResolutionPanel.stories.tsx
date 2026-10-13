// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { UncertainResolutionPanel, type UncertainResolutionAction } from './UncertainResolutionPanel';

const actions: UncertainResolutionAction[] = [
  { id: 'mark-applied', label: 'Mark as charged', kind: 'mark-applied' },
  { id: 'mark-not-applied', label: 'Mark as not charged and continue', kind: 'mark-not-applied', requiresRiskAck: true, riskText: 'If the customer was already charged, they may be charged twice.' },
  { id: 'repeat', label: 'Charge again', kind: 'repeat', requiresRiskAck: true, riskText: 'If the first charge went through, the customer is charged a second time.' },
];

const manualCheck = 'Open the payments dashboard and search for payment pay_7Q2K. If it appears as "captured", the customer was charged.';

const meta = {
  title: 'Components/UncertainResolutionPanel',
  component: UncertainResolutionPanel,
  args: { nodeName: 'Charge card', manualCheck, actions },
} satisfies Meta<typeof UncertainResolutionPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    // The fixed description never calls it a failure or a success, and the manual check text is shown as received.
    await expect(view.getByText(/may or may not have applied this operation/)).toBeVisible();
    await expect(view.getByText(manualCheck)).toBeVisible();
  },
};
export const WithRiskAck: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('button', { name: 'Charge again' })).toBeDisabled();
    await expect(view.getByRole('button', { name: 'Mark as not charged and continue' })).toBeDisabled();
    await expect(view.getByRole('button', { name: 'Mark as charged' })).toBeEnabled();
  },
};
export const Acknowledged: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await userEvent.click(view.getByRole('checkbox', { name: /If the first charge went through/ }));
    await expect(view.getByRole('button', { name: 'Charge again' })).toBeEnabled();
    // Acknowledging one risk does not unlock the other action.
    await expect(view.getByRole('button', { name: 'Mark as not charged and continue' })).toBeDisabled();
  },
};
export const NoActions: Story = { args: { actions: [] } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Uncertain resolution panel"
      columns={['panel']}
      rows={[
        { label: 'with actions', cells: [<UncertainResolutionPanel key="a" nodeName="Charge card" manualCheck={manualCheck} actions={actions} />] },
        { label: 'no actions', cells: [<UncertainResolutionPanel key="b" nodeName="Charge card" actions={[]} />] },
      ]}
    />
  ),
};
