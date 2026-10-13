// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';
import { Button } from '../button/Button';

import { Banner, type BannerTone } from './Banner';
import { states } from './Banner.meta';

const meta = {
  title: 'Components/Banner',
  component: Banner,
  args: { title: 'The editor opened a newer version of this flow.', children: 'Your changes are saved as a draft.' },
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
} satisfies Meta<typeof Banner>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
export const Success: Story = { args: { tone: 'success', title: 'Flow published.', children: undefined } };
export const Warning: Story = { args: { tone: 'warning', title: 'This connector is deprecated.', children: 'Plan to move to the new connector.' } };
export const Danger: Story = { args: { tone: 'danger', title: 'The last execution failed.', children: 'The remote service did not answer.' } };
export const Attention: Story = { args: { tone: 'attention', title: 'An execution needs a decision.', children: 'It is waiting for an approval.' } };
export const Action: Story = {
  args: {
    tone: 'attention',
    title: 'An execution needs a decision.',
    action: <Button variant="secondary">Review</Button>,
  },
};

const tones: Record<(typeof states)[number], BannerTone> = {
  default: 'info',
  success: 'success',
  warning: 'warning',
  danger: 'danger',
  attention: 'attention',
  action: 'attention',
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Banner tones"
      columns={['banner']}
      rows={states.map((state) => ({
        label: state,
        cells: [
          <div key={state} className="w-panel-width">
            <Banner tone={tones[state]} title="Title of the banner" action={state === 'action' ? <Button variant="secondary">Review</Button> : undefined}>
              Supporting text.
            </Banner>
          </div>,
        ],
      }))}
    />
  ),
};
