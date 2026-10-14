// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowDocument } from './flow-document';

const MAX_NAME = 128;

/** The name a node text starts from: "Send email 2" and "Send email" share the stem "Send email". */
function stemOf(name: string): string {
  return name.replace(/ \d+$/, '');
}

/**
 * A node name that is free in the flow (names are unique, they are what `$('Name')` reads):
 * `base` itself when nobody has it, otherwise the stem followed by the lowest number from 2 that
 * is free ("Send email 2", "Send email 3").
 */
export function uniqueName(base: string, taken: ReadonlySet<string>): string {
  if (!taken.has(base)) return base;
  const stem = stemOf(base);
  let index = 2;
  for (;;) {
    const suffix = ` ${String(index)}`;
    const candidate = `${stem.slice(0, MAX_NAME - suffix.length)}${suffix}`;
    if (!taken.has(candidate)) return candidate;
    index += 1;
  }
}

/** The names of all nodes of the document, except the node `exceptId`. */
export function namesOf(document: FlowDocument, exceptId?: string): Set<string> {
  return new Set(document.nodes.filter((node) => node.id !== exceptId).map((node) => node.name));
}
