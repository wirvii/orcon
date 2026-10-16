// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowDocument, FlowEdge } from './flow-document';

/** A stable text of a value: object members in key order, so two equal values give the same text. */
function canonical(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  if (typeof value === 'object' && value !== null) {
    const members = Object.entries(value)
      .filter(([, member]) => member !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
    return `{${members.map(([key, member]) => `${JSON.stringify(key)}:${canonical(member)}`).join(',')}}`;
  }
  return JSON.stringify(value);
}

/** A connection is told apart by where it leaves and where it goes (SPEC-014 section 10); its condition, default flag and order are what can change. */
const edgeKey = (edge: FlowEdge) => `${edge.from.node}\u0000${edge.from.port}\u0000${edge.to.node}\u0000${edge.kind}`;
const edgeBody = (edge: FlowEdge) => canonical({ condition: edge.condition, default: edge.default, order: (edge as { order?: unknown }).order });

export interface PendingChanges {
  readonly nodesAdded: number;
  readonly nodesRemoved: number;
  readonly nodesChanged: number;
  readonly connectionsAdded: number;
  readonly connectionsRemoved: number;
  readonly connectionsChanged: number;
}

/** What differs between the draft and the published version, counted the way the Publish button says it. */
export function diffCounts(draft: FlowDocument, published: FlowDocument): PendingChanges {
  const before = new Map(published.nodes.map((node) => [node.id, canonical(node)]));
  const after = new Map(draft.nodes.map((node) => [node.id, canonical(node)]));
  let nodesAdded = 0;
  let nodesChanged = 0;
  for (const [id, text] of after) {
    const old = before.get(id);
    if (old === undefined) nodesAdded += 1;
    else if (old !== text) nodesChanged += 1;
  }
  const nodesRemoved = [...before.keys()].filter((id) => !after.has(id)).length;

  const oldEdges = new Map(published.edges.map((edge) => [edgeKey(edge), edgeBody(edge)]));
  const newEdges = new Map(draft.edges.map((edge) => [edgeKey(edge), edgeBody(edge)]));
  let connectionsAdded = 0;
  let connectionsChanged = 0;
  for (const [key, body] of newEdges) {
    const old = oldEdges.get(key);
    if (old === undefined) connectionsAdded += 1;
    else if (old !== body) connectionsChanged += 1;
  }
  const connectionsRemoved = [...oldEdges.keys()].filter((key) => !newEdges.has(key)).length;
  return { nodesAdded, nodesRemoved, nodesChanged, connectionsAdded, connectionsRemoved, connectionsChanged };
}

/**
 * The number on the Publish button (SPEC-019 R-UX-EDT-02): nodes added, removed or changed plus connections
 * added, removed or changed between the draft and the version that is published, worked out in the browser.
 * It is `undefined` without a published version (the button then says only "Publish") and 0 when they are equal.
 */
export function pendingChanges(draft: FlowDocument, published: FlowDocument | undefined): number | undefined {
  if (published === undefined) return undefined;
  const counts = diffCounts(draft, published);
  return counts.nodesAdded + counts.nodesRemoved + counts.nodesChanged + counts.connectionsAdded + counts.connectionsRemoved + counts.connectionsChanged;
}
