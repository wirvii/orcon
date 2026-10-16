// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest';

import { exampleFlowDocument } from './example-flow';
import { cloneDocument } from './flow-document';
import { diffCounts, pendingChanges } from './pending-changes';

const published = () => exampleFlowDocument();

test('pendingChanges counts added removed and changed nodes and connections', () => {
  const draft = cloneDocument(published());
  // One node added with its connection, one node removed (with the connection into it), one node changed.
  draft.nodes.push({ id: 'extra', name: 'Extra', element: 'task', type: 'orcon.noop@1', parameters: {} });
  draft.edges.push({ id: 'e9', from: { node: 'done', port: 'main' }, to: { node: 'extra' }, kind: 'SEQUENCE' });
  draft.nodes = draft.nodes.filter((node) => node.id !== 'rejected');
  draft.edges = draft.edges.filter((edge) => edge.to.node !== 'rejected');
  const changed = draft.nodes.find((node) => node.id === 'validate');
  if (changed) changed.parameters = { condition: '={{ $json.total > 10 }}' };

  expect(diffCounts(draft, published())).toEqual({
    nodesAdded: 1,
    nodesRemoved: 1,
    nodesChanged: 1,
    connectionsAdded: 1,
    connectionsRemoved: 1,
    connectionsChanged: 0,
  });
  expect(pendingChanges(draft, published())).toBe(5);
});

test('pendingChanges is zero when the draft equals the active version', () => {
  const draft = cloneDocument(published());
  // Moving a node is not a change to what runs.
  draft.editor.nodes = { webhook: { x: 999, y: 999 } };
  expect(pendingChanges(draft, published())).toBe(0);
});

test('pendingChanges is absent without a published version', () => {
  expect(pendingChanges(published(), undefined)).toBeUndefined();
});

test('pendingChanges counts a changed connection condition once', () => {
  const draft = cloneDocument(published());
  const edge = draft.edges.find((candidate) => candidate.id === 'e2');
  if (edge) edge.condition = '={{ true }}';
  expect(pendingChanges(draft, published())).toBe(1);
});
