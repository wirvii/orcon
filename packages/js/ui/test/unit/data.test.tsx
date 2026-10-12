// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { DataTable, type DataColumn } from '../../src/components/data-table/DataTable';
import { Pagination } from '../../src/components/pagination/Pagination';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../src/components/table/Table';

import { renderWithProviders } from './helpers';

interface Row {
  id: string;
  name: string;
}
const columns: DataColumn<Row>[] = [
  { id: 'id', header: 'ID', cell: (row) => row.id, sortable: true },
  { id: 'name', header: 'Name', cell: (row) => row.name },
];
const rows: Row[] = [
  { id: '1', name: 'Alpha' },
  { id: '2', name: 'Beta' },
];

describe('Table', () => {
  it('has column headers with scope and marks body rows as data rows', () => {
    renderWithProviders(
      <Table aria-label="Things">
        <TableHeader>
          <TableRow header>
            <TableHead>Name</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow selected>
            <TableCell>Alpha</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );
    expect(screen.getByRole('columnheader', { name: 'Name' })).toHaveAttribute('scope', 'col');
    const body = screen.getAllByRole('row')[1];
    expect(body).toHaveAttribute('data-row');
    expect(body).toHaveAttribute('data-state', 'selected');
    expect(screen.getAllByRole('row')[0]).not.toHaveAttribute('data-row');
  });

  it('keeps cells on one line unless told to wrap', () => {
    renderWithProviders(
      <Table aria-label="Things">
        <TableBody>
          <TableRow>
            <TableCell>One line</TableCell>
            <TableCell wrap>Wraps</TableCell>
          </TableRow>
        </TableBody>
      </Table>,
    );
    expect(screen.getByText('One line').className).toContain('whitespace-nowrap');
    expect(screen.getByText('Wraps').className).not.toContain('whitespace-nowrap');
  });
});

describe('DataTable', () => {
  const base = { label: 'Things', columns, rows, getRowId: (row: Row) => row.id };

  it('renders a labeled scroll region with one row per item', () => {
    renderWithProviders(<DataTable {...base} />);
    const region = screen.getByRole('region', { name: 'Things' });
    expect(region).toHaveAttribute('tabindex', '0');
    expect(within(region).getAllByRole('row')).toHaveLength(3);
  });

  it('reports sort order with aria-sort and asks the caller to sort', async () => {
    const onSortChange = vi.fn();
    renderWithProviders(<DataTable {...base} sort={{ columnId: 'id', direction: 'ascending' }} onSortChange={onSortChange} />);
    expect(screen.getByRole('columnheader', { name: /ID/ })).toHaveAttribute('aria-sort', 'ascending');
    expect(screen.getByRole('columnheader', { name: 'Name' })).not.toHaveAttribute('aria-sort');
    await userEvent.click(screen.getByRole('button', { name: 'ID' }));
    expect(onSortChange).toHaveBeenCalledWith({ columnId: 'id', direction: 'descending' });
  });

  it('starts ascending when another column is chosen', async () => {
    const onSortChange = vi.fn();
    renderWithProviders(
      <DataTable {...base} columns={[{ ...columns[0]!, sortable: true }, { ...columns[1]!, sortable: true }]} sort={{ columnId: 'id', direction: 'descending' }} onSortChange={onSortChange} />,
    );
    await userEvent.click(screen.getByRole('button', { name: 'Name' }));
    expect(onSortChange).toHaveBeenCalledWith({ columnId: 'name', direction: 'ascending' });
  });

  it('shows placeholder rows while loading and says so', () => {
    renderWithProviders(<DataTable {...base} status="loading" loadingRows={2} />);
    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
    expect(screen.getAllByText('Loading rows')[0]).toBeInTheDocument();
    expect(screen.queryByText('Alpha')).toBeNull();
  });

  it('says what is empty, with a custom message when given', () => {
    const { unmount } = renderWithProviders(<DataTable {...base} rows={[]} />);
    expect(screen.getByText('No results')).toBeInTheDocument();
    unmount();
    renderWithProviders(<DataTable {...base} rows={[]} emptyMessage="No executions yet." />);
    expect(screen.getByText('No executions yet.')).toBeInTheDocument();
  });

  it('announces a failed load and offers a retry', async () => {
    const onRetry = vi.fn();
    renderWithProviders(<DataTable {...base} status="error" onRetry={onRetry} />, { locale: 'es' });
    expect(screen.getByRole('alert')).toHaveTextContent('No se pudieron cargar las filas.');
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});

describe('Pagination', () => {
  it('shows the position in words and moves between pages', async () => {
    const onPageChange = vi.fn();
    renderWithProviders(<Pagination page={2} pageCount={5} onPageChange={onPageChange} />);
    expect(screen.getByRole('navigation', { name: 'Pagination' })).toBeInTheDocument();
    expect(screen.getByText('Page 2 of 5')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', { name: 'Next' }));
    expect(onPageChange).toHaveBeenLastCalledWith(3);
    await userEvent.click(screen.getByRole('button', { name: 'Previous' }));
    expect(onPageChange).toHaveBeenLastCalledWith(1);
  });

  it('disables Previous on the first page and Next on the last', () => {
    const { rerender } = renderWithProviders(<Pagination page={1} pageCount={3} onPageChange={() => undefined} />);
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Next' })).toBeEnabled();
    rerender(<Pagination page={3} pageCount={3} onPageChange={() => undefined} />);
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled();
  });

  it('is translated', () => {
    renderWithProviders(<Pagination page={1} pageCount={4} onPageChange={() => undefined} />, { locale: 'es' });
    expect(screen.getByText('Página 1 de 4')).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Paginación' })).toBeInTheDocument();
  });
});
