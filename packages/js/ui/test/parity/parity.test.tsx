// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cleanup, render } from '@testing-library/react';
import { page, userEvent } from 'vitest/browser';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';

import '../../src/styles/index.css';
import { dragBetween } from '../../src/parity/drag';
import type { Driver } from '../../src/parity/env';
import { parityEnv, ParityStage } from '../../src/parity/ParityStage';
import { ownedRows } from '../../src/parity/rows';
import { parityScripts } from '../../src/parity/scripts';
import { Providers } from '../../src/providers/Providers';
import { must } from '../../src/lib/must';

// Real input in a real browser: the pointer scripts need it (a drag is a real press, move and release).
const driver: Driver = {
  click: (element) => userEvent.click(element),
  dblClick: (element) => userEvent.dblClick(element),
  hover: (element) => userEvent.hover(element),
  keyboard: (keys) => userEvent.keyboard(keys),
  type: (element, text) => userEvent.type(element, text),
  drag: dragBetween,
};

beforeAll(async () => {
  await page.viewport(1440, 900);
});

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-theme');
});

describe('every parity row of this repository has a script', () => {
  it('covers each row it owns, with a keyboard script and an end-state check', () => {
    expect(ownedRows.length).toBeGreaterThanOrEqual(26);
    for (const row of ownedRows) {
      const script = parityScripts[row.id];
      expect(script, `${row.id} has no script`).toBeDefined();
      expect(typeof script?.keyboard, `${row.id} has no keyboard script`).toBe('function');
      expect(typeof script?.verify, `${row.id} has no end-state check`).toBe('function');
      // The pointer script exists for every row but the keyboard-only one (P-30).
      if (row.id !== 'P-30') expect(typeof script?.pointer, `${row.id} has no pointer script`).toBe('function');
    }
  });
});

// Each row runs its pointer script and its keyboard script from a fresh editor, in light and in dark
// (P-30: every script of the other rows also runs in dark). Both must reach the same end state within
// the steps the table allows (the `unit` of each script checks that).
describe.each(['light', 'dark'] as const)('parity in the %s theme', (theme) => {
  describe.each(ownedRows.map((row) => [row.id, row.title] as const))('%s %s', (id) => {
    const script = must(parityScripts[id]);
    const variants = (['pointer', 'keyboard'] as const).filter((kind) => typeof script[kind] === 'function');

    it.each(variants)('%s script', async (kind) => {
      const view = render(
        <Providers theme={theme} density="comfortable" locale="en">
          <ParityStage row={id} />
        </Providers>,
      );
      const env = parityEnv(driver, view.container, id);
      await must(script[kind])(env);
      await script.verify(env);
      expect(env.units.length, `${id} ${kind} measured no unit`).toBeGreaterThan(0);
      for (const unit of env.units) expect(unit.steps, `${id} ${kind} ${unit.label}`).toBeLessThanOrEqual(unit.max);
    });
  });
});
