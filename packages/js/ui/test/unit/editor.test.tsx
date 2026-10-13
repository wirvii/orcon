// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { DataPane } from '../../src/editor/data-pane/DataPane';
import { DataTableView, VIRTUALIZE_ROWS } from '../../src/editor/data-table-view/DataTableView';
import { manyItems, mixedItems, orderItems } from '../../src/editor/fixtures';
import { JsonTreeView } from '../../src/editor/json-tree-view/JsonTreeView';
import { EXAMPLE_LIMIT, columnsOf, inferSchema, protectedValue, visibleValue } from '../../src/editor/model';
import { SchemaView } from '../../src/editor/schema-view/SchemaView';

import { renderWithProviders } from './helpers';
import { must } from '../../src/lib/must';

describe('DataTableView', () => {
  it('numbers the items and shows one column per field, values in mono text', () => {
    renderWithProviders(<DataTableView items={orderItems} />);
    const table = screen.getByRole('table');
    expect(within(table).getAllByRole('columnheader').map((header) => header.textContent)).toEqual(['#', ...columnsOf(orderItems)]);
    expect(within(table).getAllByRole('row')).toHaveLength(orderItems.length + 1);
    expect(table.className).toContain('text-mono');
    expect(within(table).getByRole('cell', { name: '1' })).toHaveClass('text-fg-subtle');
  });

  it('dims null', () => {
    renderWithProviders(<DataTableView items={orderItems} />);
    for (const cell of screen.getAllByText('null')) expect(cell).toHaveClass('text-fg-subtle');
  });

  it('collapses nested objects as "{…} 3 keys" and expands them on request', async () => {
    const user = userEvent.setup();
    renderWithProviders(<DataTableView items={orderItems} />);
    const toggle = must(screen.getAllByRole('button', { name: '{…} 3 keys, shipping' })[0]);
    expect(toggle).toHaveTextContent('{…} 3 keys');
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText(/"zip": "410001"/)).toBeInTheDocument();
    // Lists too.
    expect(screen.getAllByRole('button', { name: '[…] 2 entries, lines' })[0]).toHaveTextContent('[…] 2 entries');
  });

  it('shows a protected value as a notice, never blank and never the value', () => {
    renderWithProviders(<DataTableView items={mixedItems} />);
    expect(screen.getAllByText('Hidden — you need permission to view execution data').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Secret value').length).toBeGreaterThan(0);
    expect(screen.getAllByText('Hidden in history').length).toBeGreaterThan(0);
    expect(document.querySelectorAll('[data-redacted]').length).toBeGreaterThan(0);
  });

  it('shows a file by name, type and size, with a download link', () => {
    renderWithProviders(<DataTableView items={mixedItems} />);
    expect(screen.getByText(/invoice-2001\.pdf/)).toBeInTheDocument();
    expect(screen.getByText(/application\/pdf · 48,213 bytes/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Download invoice-2001.pdf' })).toHaveAttribute('href', '#invoice-2001.pdf');
  });

  it('has a scrollable region with an accessible name, and a sticky header on the sunken surface', () => {
    renderWithProviders(<DataTableView items={orderItems} label="Orders" />);
    const region = screen.getByRole('region', { name: 'Orders' });
    expect(region).toHaveProperty('tabIndex', 0);
    expect(region.className).toContain('overflow-auto');
    const header = must(within(region).getAllByRole('columnheader')[1]);
    expect(header).toHaveClass('sticky', 'bg-surface-sunken');
  });

  it('renders only the rows in view for a long list', () => {
    renderWithProviders(<DataTableView items={manyItems(1000)} />);
    expect(manyItems(1000).length).toBeGreaterThan(VIRTUALIZE_ROWS);
    const rows = document.querySelectorAll('tbody tr[data-item-row]');
    expect(rows.length).toBeGreaterThan(0);
    expect(rows.length).toBeLessThan(60);
  });

  it('says so when there are no items', () => {
    renderWithProviders(<DataTableView items={[]} />);
    expect(screen.getByText('No items')).toBeInTheDocument();
  });
});

describe('DataPane copy', () => {
  it('never copies a hidden value, even if one was attached by mistake', async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
    // A protected marker that carries the real value by mistake must still not reach the clipboard.
    const leaked = { ...protectedValue('secret'), value: 'sk-live-123' } as unknown;
    renderWithProviders(<DataPane mode="output" data={{ items: [{ id: 'a', token: leaked, name: 'Ana' }] }} />);
    await user.click(screen.getByRole('button', { name: 'Copy' }));
    expect(writeText).toHaveBeenCalledTimes(1);
    const copied = String(writeText.mock.calls[0]?.[0]);
    expect(copied).toContain('Ana');
    expect(copied).toContain('[hidden]');
    expect(copied).not.toContain('sk-live-123');
  });

  it('keeps hidden values hidden in the converted value', () => {
    const value = visibleValue({ a: protectedValue('permission'), b: [protectedValue('history')], c: 1 });
    expect(value).toEqual({ a: '[hidden]', b: ['[hidden]'], c: 1 });
  });
});

describe('JsonTreeView', () => {
  it('uses only the roles of the data panel: keys, strings, numbers, booleans and null', () => {
    renderWithProviders(<JsonTreeView value={[{ name: 'Ana', total: 12, paid: true, coupon: null }]} />);
    const tree = screen.getByRole('tree', { name: 'Data as JSON' });
    const classes = new Set<string>();
    for (const span of tree.querySelectorAll('[role="treeitem"] span')) for (const name of span.className.split(' ')) if (name.startsWith('text-')) classes.add(name);
    expect([...classes].sort()).toEqual(['text-fg', 'text-fg-accent', 'text-fg-link', 'text-fg-muted', 'text-fg-subtle']);
    expect(screen.getByText('"Ana"')).toHaveClass('text-fg-accent');
    expect(screen.getByText('12')).toHaveClass('text-fg-link');
    expect(screen.getByText('true')).toHaveClass('text-fg', 'font-semibold');
    expect(screen.getByText('null')).toHaveClass('text-fg-subtle');
    // No status color is ever used for data.
    expect(tree.innerHTML).not.toMatch(/status-(danger|warning|attention|success|active)/);
  });

  it('is a tree with one tab stop: arrows move, Left collapses and Right expands', async () => {
    const user = userEvent.setup();
    renderWithProviders(<JsonTreeView value={[{ shipping: { city: 'Neiva' } }]} />);
    const items = () => screen.getAllByRole('treeitem');
    expect(items().filter((item) => item.tabIndex === 0)).toHaveLength(1);
    act(() => {
      must(items()[0]).focus();
    });
    await user.keyboard('{ArrowDown}');
    await vi.waitFor(() => expect(items()[1]).toHaveFocus());
    await user.keyboard('{ArrowDown}');
    await vi.waitFor(() => expect(items()[2]).toHaveFocus());
    // Nodes below the second level start collapsed.
    expect(items()[2]).toHaveAttribute('aria-expanded', 'false');
    await user.keyboard('{ArrowRight}');
    expect(items()[2]).toHaveAttribute('aria-expanded', 'true');
    await user.keyboard('{ArrowLeft}');
    expect(items()[2]).toHaveAttribute('aria-expanded', 'false');
  });

  it('shows protected values as notices', () => {
    renderWithProviders(<JsonTreeView value={mixedItems.slice(0, 1)} />);
    expect(screen.getAllByText('Hidden — you need permission to view execution data').length).toBeGreaterThan(0);
  });
});

describe('SchemaView', () => {
  it('shows the type of each field and a short example', () => {
    renderWithProviders(<SchemaView items={orderItems} />);
    const list = screen.getByRole('list', { name: 'Data fields' });
    expect(within(list).getAllByText('number').length).toBeGreaterThan(0);
    expect(within(list).getByText('ord_1001')).toBeInTheDocument();
    expect(within(list).getAllByText('object').length).toBeGreaterThan(0);
  });

  it('cuts an example at 60 characters', () => {
    const fields = inferSchema([{ note: 'x'.repeat(300) }]);
    expect(fields[0]?.example?.length).toBeLessThanOrEqual(EXAMPLE_LIMIT);
    renderWithProviders(<SchemaView items={[{ note: 'x'.repeat(300) }]} />);
    const example = must(document.querySelector<HTMLElement>('[data-schema-field="note"] [data-schema-example]'));
    expect(example.textContent.length).toBeLessThanOrEqual(EXAMPLE_LIMIT);
  });

  it('masks the example of a sensitive field', () => {
    renderWithProviders(<SchemaView items={mixedItems} />);
    const row = must(document.querySelector('[data-schema-field="apiToken"]'));
    expect(within(row as HTMLElement).getByText('hidden')).toBeInTheDocument();
    expect(row.querySelector('[data-schema-example="masked"]')).not.toBeNull();
    expect(row.textContent).not.toMatch(/sk-/);
  });
});

describe('item counts follow ICU plurals in English and Spanish', () => {
  const pane = (count: number) => <DataPane mode="output" data={{ items: orderItems.slice(0, count).length === count ? orderItems.slice(0, count) : [] }} />;

  it.each([
    [1, '1 item'],
    [3, '3 items'],
    [0, '0 items'],
  ])('en: %i -> "%s"', (count, text) => {
    renderWithProviders(pane(count), { locale: 'en' });
    expect(screen.getByText(text)).toBeInTheDocument();
  });

  it.each([
    [1, '1 elemento'],
    [3, '3 elementos'],
    [0, '0 elementos'],
  ])('es: %i -> "%s"', (count, text) => {
    renderWithProviders(pane(count), { locale: 'es' });
    expect(screen.getByText(text)).toBeInTheDocument();
  });
});
