// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cleanup, render } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { settle, targetsBelow } from '../harness/checks';
import { composeAll } from '../harness/stories';

const MIN = 44;

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-density');
});

describe('the target gate itself', () => {
  it('runs with a coarse pointer', () => {
    // The rule only applies to a coarse pointer; fail loudly if the emulation is missing.
    expect(window.matchMedia('(pointer: coarse)').matches).toBe(true);
  });

  it('rejects a control smaller than 44 px', () => {
    // The class is not part of the theme, so the button keeps its small intrinsic size.
    render(
      <button type="button" aria-label="Tiny" className="size-icon-16">
        x
      </button>,
    );
    expect(targetsBelow(MIN)).toHaveLength(1);
  });

  it('measures a visually hidden skip link by its size on focus', () => {
    // Hidden at 1 px and still 1 px when focused: too small.
    const { unmount } = render(
      <a href="#x" className="sr-only">
        Skip
      </a>,
    );
    expect(targetsBelow(MIN)).toHaveLength(1);
    unmount();
    // Hidden, then 44 px high and wide enough when focused: fine.
    render(
      <a href="#x" className="sr-only focus:not-sr-only focus:inline-flex focus:min-h-target focus:min-w-target">
        Skip
      </a>,
    );
    expect(targetsBelow(MIN)).toEqual([]);
  });

  it('exempts inline links inside running text', () => {
    render(
      <p data-prose>
        <a href="#x">link</a>
      </p>,
    );
    expect(targetsBelow(MIN)).toEqual([]);
  });
});

describe.each(['comfortable', 'compact'] as const)('targets with a coarse pointer, %s density', (density) => {
  const entries = composeAll({ density });

  it.each(entries.map((entry) => [entry.label, entry] as const))('%s', async (_label, entry) => {
    const { container } = render(<entry.Story />);
    await entry.Story.play?.({ canvasElement: container });
    await settle();

    expect(targetsBelow(MIN)).toEqual([]);

    // Data rows follow the density tokens (40 px comfortable, 32 px compact). A row that holds a
    // control grows to the 44 px minimum target on a coarse pointer; the fine pointer density test
    // measures every row exactly.
    const token = density === 'compact' ? 32 : 40;
    for (const row of document.body.querySelectorAll<HTMLElement>('[data-row]')) {
      const holdsControl = row.querySelector('a[href], button, input, select, textarea, [role="button"], [role="checkbox"]') !== null;
      const height = row.getBoundingClientRect().height;
      if (holdsControl) {
        // The control sets the height (plus the 1 px divider).
        expect(height, 'row with a control').toBeGreaterThanOrEqual(Math.max(token, MIN) - 0.5);
        expect(height, 'row with a control').toBeLessThanOrEqual(Math.max(token, MIN) + 1.5);
      } else {
        expect(height, 'row height').toBeCloseTo(token, 0);
      }
    }
  });
});
