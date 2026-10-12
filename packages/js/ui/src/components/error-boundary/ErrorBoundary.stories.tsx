// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ErrorBoundary, ErrorFallback } from './ErrorBoundary';

const meta = {
  title: 'Components/ErrorBoundary',
  component: ErrorFallback,
  args: { error: new Error('Cannot read properties of undefined (reading "nodes")'), reset: () => undefined },
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
} satisfies Meta<typeof ErrorFallback>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Details: Story = {
  play: async ({ canvasElement }) => {
    const summary = within(canvasElement).getByText('Technical details');
    summary.click();
    await expect(within(canvasElement).getByText(/Cannot read properties/)).toBeVisible();
  },
};

function Broken(): never {
  throw new Error('A node failed to render');
}

/** The boundary itself: a throwing child is replaced by the fallback. */
export const Caught: Story = {
  render: () => (
    <ErrorBoundary onError={() => undefined}>
      <Broken />
    </ErrorBoundary>
  ),
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('alert')).toHaveTextContent('Something went wrong on this screen.');
  },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ErrorBoundary fallback"
      columns={['fallback']}
      rows={[
        {
          label: 'default',
          cells: [
            <div key="d" className="w-node-width">
              <ErrorFallback error={new Error('A node failed to render')} reset={() => undefined} />
            </div>,
          ],
        },
      ]}
    />
  ),
};
