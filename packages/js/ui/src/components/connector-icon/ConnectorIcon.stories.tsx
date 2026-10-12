// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';

import { ConnectorIcon } from './ConnectorIcon';

// A 2x2 PNG stands in for a connector logo, so no network request is made.
const logo =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAEElEQVR4nGNIKekGIgYIBQAmRgWNPOJSIgAAAABJRU5ErkJggg==';

const meta = {
  title: 'Components/ConnectorIcon',
  component: ConnectorIcon,
  args: { src: logo, name: 'Example connector' },
} satisfies Meta<typeof ConnectorIcon>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    // The logo is an image element named after the connector, not inline SVG.
    await expect(within(canvasElement).getByRole('img', { name: 'Example connector' }).tagName).toBe('IMG');
    await expect(canvasElement.querySelector('svg')).toBeNull();
  },
};

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="ConnectorIcon"
      columns={['default']}
      rows={[{ label: 'frame', cells: [<ConnectorIcon key="c" src={logo} name="Example connector" />] }]}
    />
  ),
};
