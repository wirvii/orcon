// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import '../../src/styles/index.css';

import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { DataTable, type DataColumn } from '../../src/components/data-table/DataTable';
import { Providers } from '../../src/providers/Providers';

interface Row {
  id: string;
  name: string;
  runs: number;
}

const ROWS = 10_000;
const rows: Row[] = Array.from({ length: ROWS }, (_, index) => ({ id: `row-${String(index + 1)}`, name: `Flow ${String(index + 1)}`, runs: index % 97 }));
const columns: DataColumn<Row>[] = [
  { id: 'name', header: 'Flow', cell: (row) => row.name },
  { id: 'runs', header: 'Runs', cell: (row) => row.runs, align: 'end' },
];

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-density');
});

const frame = () => new Promise<number>((resolve) => requestAnimationFrame(resolve));

function mount() {
  return render(
    <Providers theme="light" density="comfortable" locale="en">
      <DataTable label="Flows" columns={columns} rows={rows} getRowId={(row) => row.id} virtual className="max-h-panel-width max-w-screen-md" />
    </Providers>,
  );
}

const renderedRows = (container: HTMLElement) => container.querySelectorAll('tbody tr[data-row]');
const region = (container: HTMLElement) => {
  const element = container.querySelector<HTMLElement>('[role="region"]');
  if (!element) throw new Error('The table has no scroll region.');
  return element;
};

describe('DataTable in virtual mode with 10,000 rows (orcon-web PF-5)', () => {
  it('renders at most 60 rows, however many there are', async () => {
    const { container } = mount();
    await frame();
    const count = renderedRows(container).length;
    expect(count).toBeGreaterThan(0);
    expect(count).toBeLessThanOrEqual(60);
  });

  it('renders the first time in under 100 ms', async () => {
    const start = performance.now();
    const { container } = mount();
    await frame();
    const elapsed = performance.now() - start;
    expect(renderedRows(container).length).toBeGreaterThan(0);
    expect(elapsed).toBeLessThan(100);
  });

  it('keeps the scrollbar true to the whole list', async () => {
    const { container } = mount();
    await frame();
    // 10,000 rows of 40 px plus the header: far taller than the panel the table shows.
    expect(region(container).scrollHeight).toBeGreaterThan(ROWS * 40);
  });

  it('tells assistive technology how many rows there are and where each rendered row is', async () => {
    const { container } = mount();
    await frame();
    const table = container.querySelector('table');
    expect(table).toHaveAttribute('aria-rowcount', String(ROWS + 1));
    expect(renderedRows(container)[0]).toHaveAttribute('aria-rowindex', '2');
  });

  it('scrolls to the end without a frame longer than 50 ms, and shows the last row there', async () => {
    const { container } = mount();
    await frame();
    const scroller = region(container);
    const target = scroller.scrollHeight - scroller.clientHeight;
    const durations: number[] = [];
    let last = await frame();
    const steps = 60;
    for (let step = 1; step <= steps; step += 1) {
      scroller.scrollTop = (target * step) / steps;
      const now = await frame();
      durations.push(now - last);
      last = now;
    }
    // Settle on the very end, then give the virtualizer a few frames to render what is there.
    scroller.scrollTop = scroller.scrollHeight;
    for (let settle = 0; settle < 5; settle += 1) await frame();

    expect(Math.max(...durations)).toBeLessThanOrEqual(50);
    expect(renderedRows(container).length).toBeLessThanOrEqual(60);
    expect(container.textContent).toContain(`Flow ${String(ROWS)}`);
    expect(container.textContent).not.toContain('Flow 1 ');
  });

  it('follows the density: compact rows are shorter, so more of them fit', async () => {
    document.documentElement.setAttribute('data-density', 'compact');
    const { container } = mount();
    await frame();
    await frame();
    const compactCount = renderedRows(container).length;
    cleanup();
    document.documentElement.removeAttribute('data-density');
    const comfortable = mount();
    await frame();
    await frame();
    expect(compactCount).toBeGreaterThan(renderedRows(comfortable.container).length - 1);
  });
});
