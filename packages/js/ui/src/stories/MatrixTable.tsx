// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ReactNode } from 'react';

export interface MatrixRow {
  label: string;
  cells: ReactNode[];
}

export interface MatrixTableProps {
  caption: string;
  columns: string[];
  rows: MatrixRow[];
}

/**
 * Story-only layout for the `Matrix` story of a component: variants down the
 * side, states across the top. It is a real table so the grid is readable by
 * assistive technology. Not exported from the package.
 */
export function MatrixTable({ caption, columns, rows }: MatrixTableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="border-collapse text-sm">
        <caption className="text-fg-muted pb-2 text-left">{caption}</caption>
        <thead>
          <tr>
            <td />
            {columns.map((column) => (
              <th key={column} scope="col" className="text-fg-muted px-3 pb-2 text-left font-medium">
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.label}>
              <th scope="row" className="text-fg-muted py-2 pr-3 text-left font-medium">
                {row.label}
              </th>
              {row.cells.map((cell, index) => (
                <td key={columns[index]} className="px-3 py-2 align-middle">
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
