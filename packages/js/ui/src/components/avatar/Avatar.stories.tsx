// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';

import { MatrixTable } from '../../stories/MatrixTable';

import { Avatar } from './Avatar';

// A 2x2 PNG, so the image state renders without a network request.
const image =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAIAAAD91JpzAAAAEElEQVR4nGNIKekGIgYIBQAmRgWNPOJSIgAAAABJRU5ErkJggg==';

const meta = {
  title: 'Components/Avatar',
  component: Avatar,
  args: { name: 'Ada Lovelace' },
  argTypes: { size: { control: 'inline-radio', options: ['sm', 'md', 'lg'] } },
} satisfies Meta<typeof Avatar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Initials, shown when there is no image. */
export const Default: Story = {};
export const Image: Story = { args: { src: image } };

export const Matrix: Story = {
  render: () => (
    <MatrixTable
      caption="Avatar sizes"
      columns={['default', 'image']}
      rows={(['sm', 'md', 'lg'] as const).map((size) => ({
        label: size,
        cells: [<Avatar key="d" name="Ada Lovelace" size={size} />, <Avatar key="i" name="Grace Hopper" size={size} src={image} />],
      }))}
    />
  ),
};
