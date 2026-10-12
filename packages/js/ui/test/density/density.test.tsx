// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { settle } from '../harness/checks';
import { composeAll } from '../harness/stories';

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-density');
});

/** A density token as a number of pixels, read from the page the way the CSS reads it. */
function tokenPx(name: string): number {
  const probe = document.createElement('div');
  probe.style.height = `var(${name})`;
  document.body.append(probe);
  const px = probe.getBoundingClientRect().height;
  probe.remove();
  return px;
}

describe('the density gate itself', () => {
  it('reads different tokens in the two densities', () => {
    document.documentElement.removeAttribute('data-density');
    expect(tokenPx('--orcon-size-row')).toBe(40);
    document.documentElement.setAttribute('data-density', 'compact');
    expect(tokenPx('--orcon-size-row')).toBe(32);
    expect(tokenPx('--orcon-size-control')).toBe(28);
  });

  it('fine pointer is in effect', () => {
    expect(window.matchMedia('(pointer: coarse)').matches).toBe(false);
  });
});

describe.each(['comfortable', 'compact'] as const)('%s density with a fine pointer', (density) => {
  const entries = composeAll({ density });

  it.each(entries.map((entry) => [entry.label, entry] as const))('%s', async (_label, entry) => {
    const { container } = render(<entry.Story />);
    await entry.Story.play?.({ canvasElement: container });
    await settle();

    const row = tokenPx('--orcon-size-row');
    const control = tokenPx('--orcon-size-control');
    expect(row).toBe(density === 'compact' ? 32 : 40);
    expect(control).toBe(density === 'compact' ? 28 : 36);

    // Table rows are as tall as the row token.
    for (const element of document.body.querySelectorAll<HTMLElement>('[data-row]')) {
      expect(element.getBoundingClientRect().height, 'row').toBeCloseTo(row, 0);
    }
    // Controls built on the control height (buttons, inputs, selects, tabs, menu items) match the token.
    for (const element of document.body.querySelectorAll<HTMLElement>('.h-control')) {
      if (element.getBoundingClientRect().height === 0 || element.closest('[aria-hidden="true"]')) continue;
      const classes = element.className;
      // Square icon buttons and tab triggers also use h-control, so the height is the same.
      expect(element.getBoundingClientRect().height, `control ${element.tagName.toLowerCase()} (${classes.slice(0, 40)})`).toBeCloseTo(control, 0);
    }
  });
});
