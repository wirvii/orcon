// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import axe from 'axe-core';

// WCAG 2.0 to 2.2, levels A and AA. Best-practice rules are not part of the gate.
const wcagTags = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'];

/** Waits for two animation frames so layout, fonts and transitions (0 ms) have settled. */
export const settle = () =>
  new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));

/** Runs axe on the document and returns one readable line per violation (empty when clean). */
export async function axeViolations(
  root: Element = document.body,
  /** Rule ids a story turned off in parameters.a11y.config.rules, each with a written reason in the story. */
  disabledRules: readonly string[] = [],
): Promise<string[]> {
  const results = await axe.run(root, {
    runOnly: { type: 'tag', values: wcagTags },
    rules: Object.fromEntries(disabledRules.map((id) => [id, { enabled: false }])),
  });
  return results.violations.map(
    (violation) =>
      `${violation.id} (${violation.impact}): ${violation.help}\n` +
      violation.nodes
        .map((node) => `    ${node.target.join(' ')}\n    ${node.failureSummary?.replaceAll('\n', '\n    ')}`)
        .join('\n'),
  );
}

// Controls a person can operate. Inline links inside running text (data-prose) are exempt (WCAG 2.5.8).
const interactive = [
  'a[href]',
  'button',
  'input:not([type="hidden"])',
  'select',
  'textarea',
  'summary',
  '[role="button"]',
  '[role="link"]',
  '[role="checkbox"]',
  '[role="radio"]',
  '[role="switch"]',
  '[role="tab"]',
  '[role="menuitem"]',
  '[role="menuitemcheckbox"]',
  '[role="menuitemradio"]',
  '[role="option"]',
  '[role="combobox"]',
  '[role="slider"]',
].join(',');

function isHidden(element: HTMLElement): boolean {
  if (element.closest('[aria-hidden="true"], [hidden], [inert]')) return true;
  const style = getComputedStyle(element);
  return style.visibility === 'hidden' || style.display === 'none';
}

function isInactive(element: HTMLElement): boolean {
  return element.matches(':disabled') || element.getAttribute('aria-disabled') === 'true';
}

/** The area a person can hit: the control, or its label when the control is a small native box. */
function hitRect(element: HTMLElement): DOMRect {
  const rect = element.getBoundingClientRect();
  if (element instanceof HTMLInputElement && element.labels) {
    for (const label of element.labels) {
      const labelRect = label.getBoundingClientRect();
      if (labelRect.width * labelRect.height > rect.width * rect.height) return labelRect;
    }
  }
  return rect;
}

/** Interactive controls whose hit area is smaller than `min` px in either direction. */
export function targetsBelow(min: number, root: ParentNode = document.body): string[] {
  const tooSmall: string[] = [];
  for (const element of root.querySelectorAll<HTMLElement>(interactive)) {
    if (isHidden(element) || isInactive(element) || element.closest('[data-prose]')) continue;
    const rect = hitRect(element);
    if (rect.width === 0 || rect.height === 0) continue;
    if (rect.width < min - 0.5 || rect.height < min - 0.5) {
      const name = element.getAttribute('aria-label') ?? element.textContent?.trim().slice(0, 24) ?? '';
      tooSmall.push(`${element.tagName.toLowerCase()}[${name}] is ${rect.width.toFixed(1)}x${rect.height.toFixed(1)}`);
    }
  }
  return tooSmall;
}
