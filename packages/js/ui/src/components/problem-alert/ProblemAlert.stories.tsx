// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ProblemAlert } from './ProblemAlert';
import { states } from './ProblemAlert.meta';
import type { ProblemDetails } from './problem';

const base: ProblemDetails = { title: 'The flow could not be saved.', status: 422 };

const meta = {
  title: 'Components/ProblemAlert',
  component: ProblemAlert,
  args: { problem: base },
  decorators: [
    (Story, context) =>
      context.name === 'Matrix' ? (
        <Story />
      ) : (
        <div className="max-w-screen-sm">
          <Story />
        </div>
      ),
  ],
} satisfies Meta<typeof ProblemAlert>;

export default meta;
type Story = StoryObj<typeof meta>;

/** No translation for the code: the title is shown. */
export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('alert')).toHaveTextContent('The flow could not be saved.');
  },
};

/** A code with a catalog entry shows the translated message, not the title. */
export const Translated: Story = {
  args: { problem: { ...base, code: 'attention-already-resolved', title: 'Conflict', status: 409 } },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByText(/Someone already resolved this request/)).toBeInTheDocument();
  },
};

export const Details: Story = {
  args: {
    problem: {
      ...base,
      code: 'validation-failed',
      type: 'https://orcon.example/problems/validation-failed',
      detail: 'The field "name" must not be empty.',
      instance: '/flows/12',
    },
  },
};

export const RequestId: Story = { args: { problem: { ...base, requestId: '01JB9X2ZQ4M8V3T7R5N6K1H0PD' } } };

/** HTTP 409: shows the outcome that is now in effect instead of a generic error. */
export const AlreadyResolved: Story = {
  args: {
    problem: { code: 'attention-already-resolved', title: 'Conflict', status: 409 },
    currentResult: 'Approved by Ada Lovelace a few minutes ago.',
  },
};

const problems: Record<(typeof states)[number], React.ComponentProps<typeof ProblemAlert>> = {
  default: { problem: base },
  translated: { problem: { code: 'attention-already-resolved', status: 409 } },
  details: { problem: { ...base, code: 'validation-failed', detail: 'The field "name" must not be empty.' } },
  'request-id': { problem: { ...base, requestId: '01JB9X2ZQ4M8V3T7R5N6K1H0PD' } },
  'already-resolved': { problem: { code: 'attention-already-resolved', status: 409 }, currentResult: 'Approved a few minutes ago.' },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ProblemAlert states"
      columns={['alert']}
      rows={states.map((state) => ({
        label: state,
        cells: [
          <div key={state} className="w-panel-width">
            <ProblemAlert {...problems[state]} />
          </div>,
        ],
      }))}
    />
  ),
};
