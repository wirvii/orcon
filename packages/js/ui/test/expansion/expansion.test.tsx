// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cleanup, render } from '@testing-library/react';
import { page } from '@vitest/browser/context';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';

import { settle } from '../harness/checks';
import { composeAll } from '../harness/stories';

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-density');
});

/** Text containers whose text is wider than the box, unless the overflow is declared (data-truncate) or scrolls on purpose. */
function overflowing(root: ParentNode): string[] {
  const found: string[] = [];
  for (const element of root.querySelectorAll<HTMLElement>('body *')) {
    if (element.closest('[data-truncate]') || element.closest('[aria-hidden="true"]')) continue;
    if (!Array.from(element.childNodes).some((node) => node.nodeType === Node.TEXT_NODE && node.textContent?.trim())) continue;
    const style = getComputedStyle(element);
    // A region that scrolls on purpose (code block, table region) is a declared scroller, not a clipped label.
    if (style.overflowX === 'auto' || style.overflowX === 'scroll') continue;
    // Visually hidden text (sr-only, 1 px) is for screen readers and is not drawn.
    if (element.clientWidth <= 1 && element.clientHeight <= 1) continue;
    if (element.scrollWidth > element.clientWidth + 1 && element.clientWidth > 0) {
      found.push(`${element.tagName.toLowerCase()}.${String(element.className).split(' ').slice(0, 3).join('.')} "${element.textContent?.trim().slice(0, 40)}" (${element.scrollWidth} > ${element.clientWidth})`);
    }
  }
  return found;
}

describe('the expansion gate itself', () => {
  it('reports clipped text and ignores declared truncation', () => {
    const { unmount } = render(
      <div className="w-16 overflow-hidden whitespace-nowrap">A label that is much wider than its box</div>,
    );
    expect(overflowing(document.body)).toHaveLength(1);
    unmount();
    render(
      <div data-truncate className="w-16 overflow-hidden whitespace-nowrap">
        A label that is much wider than its box
      </div>,
    );
    expect(overflowing(document.body)).toEqual([]);
  });
});

describe.each([
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'phone', width: 390, height: 844 },
] as const)('xx-LS pseudo-locale (text at least 30 % longer) on a $name viewport', ({ width, height }) => {
  beforeAll(async () => {
    await page.viewport(width, height);
  });

  const entries = composeAll({ locale: 'xx-LS' });

  it.each(entries.map((entry) => [entry.label, entry] as const))('%s keeps its text inside its boxes', async (_label, entry) => {
    render(<entry.Story />);
    // Play functions assert on English text, so they are not run here; this gate only measures the layout.
    await settle();
    expect(overflowing(document.body)).toEqual([]);
  });
});
