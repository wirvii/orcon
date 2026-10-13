// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// The ErrorBoundary story throws on purpose. React reports a caught render error to
// the console even when a boundary handles it, so that one expected report is dropped.
const original = console.error;
console.error = (...args: unknown[]) => {
  const text = args.map(String).join(' ');
  if (text.includes('<Broken>') || text.includes('A node failed to render')) return;
  original(...args);
};
