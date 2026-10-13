// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** Performance marks the budgets are measured with (orcon-web PF-3, PF-4). */
export const FIRST_USEFUL_MARK = 'orcon:first-useful';
export const EDITOR_READY_MARK = 'orcon:editor-ready';

function mark(name: string, detail: Record<string, string>): void {
  // Browsers and test environments without the User Timing API simply skip the mark.
  if (typeof performance === 'undefined' || typeof performance.mark !== 'function') return;
  performance.mark(name, { detail });
}

/**
 * Marks the moment a route shows its main content, not a skeleton. Call it from the
 * route's screen once the data is painted. The first mark of a page load is the one
 * the first-useful-screen budget measures.
 *
 * @param route The route path, for the trace: "/", "/settings/appearance".
 */
export function markFirstUsefulScreen(route: string): void {
  mark(FIRST_USEFUL_MARK, { route });
}

/** Marks the moment the flow editor reports every node initialized. */
export function markEditorReady(): void {
  mark(EDITOR_READY_MARK, {});
}
