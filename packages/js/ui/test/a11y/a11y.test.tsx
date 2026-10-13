// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cleanup, render } from '@testing-library/react';
import { page } from '@vitest/browser/context';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';

import { axeViolations, settle } from '../harness/checks';
import { composeAll } from '../harness/stories';

afterEach(() => {
  cleanup();
  document.documentElement.removeAttribute('data-theme');
  document.documentElement.removeAttribute('data-density');
});

describe('the axe gate itself', () => {
  it('rejects a control without an accessible name', async () => {
    render(<button type="button" />);
    const violations = await axeViolations();
    expect(violations.join('\n')).toContain('button-name');
  });

  it('rejects an image without alternative text', async () => {
    render(<img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" />);
    const violations = await axeViolations();
    expect(violations.join('\n')).toContain('image-alt');
  });
});

const viewports = [
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'phone', width: 390, height: 844 },
] as const;

describe.each(viewports)('axe on a $name viewport ($width px)', ({ width, height }) => {
  beforeAll(async () => {
    await page.viewport(width, height);
  });

  describe.each(['light', 'dark'] as const)('in the %s theme', (theme) => {
    const entries = composeAll({ theme });

    it('finds stories to check', () => {
      expect(entries.length).toBeGreaterThan(0);
    });

    it.each(entries.map((entry) => [entry.label, entry] as const))('%s has no violations', async (_label, entry) => {
      const { container } = render(<entry.Story />);
      await entry.Story.play?.({ canvasElement: container });
      await settle();

      const rules = (entry.Story.parameters?.['a11y'] as { config?: { rules?: { id: string; enabled?: boolean }[] } } | undefined)?.config?.rules ?? [];
      const violations = await axeViolations(
        document.body,
        rules.filter((rule) => rule.enabled === false).map((rule) => rule.id),
      );
      expect(violations, violations.join('\n')).toEqual([]);
    });
  });
});
