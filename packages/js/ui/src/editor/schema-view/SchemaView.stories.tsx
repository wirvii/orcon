// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Meta, StoryObj } from '@storybook/react-vite';
import { expect, fn, userEvent, within } from 'storybook/test';

import { MatrixTable } from '../../stories/MatrixTable';
import { mixedItems, orderItems } from '../fixtures';

import { SchemaView } from './SchemaView';
import { must } from '../../lib/must';

const meta = {
  title: 'Editor/SchemaView',
  component: SchemaView,
  args: { items: orderItems },
  decorators: [
    (Story) => (
      <div className="bg-surface border-border max-w-screen-md rounded-lg border">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SchemaView>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvasElement }) => {
    const view = within(canvasElement);
    await expect(view.getByRole('list', { name: 'Data fields' })).toBeVisible();
    await expect(view.getAllByText('number').length).toBeGreaterThan(0);
  },
};
export const Usable: Story = {
  args: { onInsert: fn() },
  play: async ({ canvasElement, args }) => {
    const view = within(canvasElement);
    await expect(view.getByText('Drag a field to a parameter to use it, or press Enter on it.')).toBeVisible();
    // A field can be chosen with the keyboard: Enter calls back with its path, positions of lists included.
    const field = view.getByRole('button', { name: /Use shipping\.city in a parameter/ });
    field.focus();
    await userEvent.keyboard('{Enter}');
    await expect(args.onInsert).toHaveBeenCalledWith(['shipping', 'city']);
    await expect(view.getByRole('button', { name: /Use lines\[0\]\.sku in a parameter/ })).toBeVisible();
  },
};
export const Protected: Story = {
  args: { items: mixedItems },
  play: async ({ canvasElement }) => {
    // A protected field shows its type as hidden and a masked example, never the value.
    await expect(canvasElement.querySelectorAll('[data-schema-example="masked"]').length).toBeGreaterThan(0);
  },
};
export const LongExample: Story = {
  args: { items: [{ id: 'ord_1', note: 'x'.repeat(200) }] },
  play: async ({ canvasElement }) => {
    const example = must(canvasElement.querySelector<HTMLElement>('[data-schema-field="note"] [data-schema-example]'));
    await expect(example.textContent.length).toBeLessThanOrEqual(60);
  },
};

export const Matrix: Story = {
  decorators: [],
  render: () => (
    <MatrixTable
      caption="Schema"
      columns={['view']}
      rows={[
        { label: 'items', cells: [<div key="a" className="bg-surface border-border max-w-screen-md rounded-lg border"><SchemaView items={orderItems} /></div>] },
        { label: 'protected', cells: [<div key="b" className="bg-surface border-border max-w-screen-md rounded-lg border"><SchemaView items={mixedItems} /></div>] },
        { label: 'long example', cells: [<div key="c" className="bg-surface border-border max-w-screen-md rounded-lg border"><SchemaView items={[{ id: 'ord_1', note: 'x'.repeat(200) }]} /></div>] },
      ]}
    />
  ),
};
