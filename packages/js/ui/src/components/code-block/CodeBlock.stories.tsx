// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { CodeBlock } from './CodeBlock';

const json = '{\n  "orderId": "48213",\n  "items": 3,\n  "status": "paid"\n}';

const meta = {
  title: 'Components/CodeBlock',
  component: CodeBlock,
  args: { code: json, label: 'Request body' },
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
} satisfies Meta<typeof CodeBlock>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByRole('region', { name: 'Request body' })).toHaveTextContent('"orderId": "48213"');
  },
};
export const Copyable: Story = { args: { copyable: true } };
export const Long: Story = {
  args: { code: `GET /v1/orders?${Array.from({ length: 12 }, (_, i) => `filter${i}=value${i}`).join('&')}`, copyable: true },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="CodeBlock"
      columns={['block']}
      rows={[
        { label: 'default', cells: [<div key="d" className="w-panel-width"><CodeBlock label="Default" code={json} /></div>] },
        { label: 'copyable', cells: [<div key="c" className="w-panel-width"><CodeBlock label="Copyable" code={json} copyable /></div>] },
        {
          label: 'long',
          cells: [
            <div key="l" className="w-panel-width">
              <CodeBlock label="Long" copyable code="GET /v1/orders?filter0=value0&filter1=value1&filter2=value2&filter3=value3" />
            </div>,
          ],
        },
      ]}
    />
  ),
};
