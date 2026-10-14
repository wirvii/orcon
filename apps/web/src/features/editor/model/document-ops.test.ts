// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import {
  addNodeOfType,
  addStickyNote,
  applyPositions,
  connect,
  disconnect,
  edgeKindFor,
  edgeProblem,
  moveNodes,
  positionOf,
  removeNodes,
  renameNode,
  setDisabled,
  setPosition,
  updateStickyNote,
} from './document-ops';
import { exampleFlowDocument } from './example-flow';
import { findNode } from './flow-document';
import { typeDefBySlug } from './node-types';

const def = (slug: string) => {
  const found = typeDefBySlug(slug);
  if (!found) throw new Error(`no node type ${slug}`);
  return found;
};

describe('connections', () => {
  it('connects two nodes and records a sequence edge with a fresh id', () => {
    const document = exampleFlowDocument();
    const result = connect(document, 'inventory', 'main', 'rejected');
    expect(result).toEqual({ edge: { id: 'edge-inventory-rejected-1', from: { node: 'inventory', port: 'main' }, to: { node: 'rejected' }, kind: 'SEQUENCE' } });
    expect(document.edges).toHaveLength(6);
  });

  it('refuses what the format forbids and says why', () => {
    const document = exampleFlowDocument();
    expect(edgeProblem(document, 'ghost', 'main', 'done')).toBe('unknown-node');
    expect(edgeProblem(document, 'slack', 'main', 'slack')).toBe('same-node');
    expect(edgeProblem(document, 'validate', 'true', 'webhook')).toBe('into-start');
    expect(edgeProblem(document, 'webhook', 'main', 'validate')).toBe('duplicate');
    expect(edgeProblem(document, 'rejected', 'main', 'slack')).toBeUndefined();
    expect(edgeProblem(document, 'slack', 'main', 'validate')).toBe('cycle');
    expect(connect(document, 'slack', 'main', 'validate')).toEqual({ problem: 'cycle' });
    expect(document.edges).toHaveLength(5);
  });

  it('lets a cycle exist only as a back edge into a loop node', () => {
    const document = exampleFlowDocument();
    const loopId = addNodeOfType(document, def('loop'), { x: 0, y: 0 }, 'Loop');
    connect(document, 'validate', 'true', loopId);
    connect(document, loopId, 'loop', 'inventory');
    expect(edgeKindFor(document, 'inventory', loopId)).toBe('BACK');
    expect(edgeProblem(document, 'inventory', 'main', loopId)).toBeUndefined();
    const result = connect(document, 'inventory', 'main', loopId, 'back-1');
    expect(result).toEqual({ edge: expect.objectContaining({ id: 'back-1', kind: 'BACK' }) as unknown });
    // A forward connection into the loop node is a plain sequence edge.
    expect(edgeKindFor(document, 'webhook', loopId)).toBe('SEQUENCE');
    expect(edgeKindFor(document, 'webhook', 'ghost')).toBe('SEQUENCE');
  });

  it('removes a connection by id', () => {
    const document = exampleFlowDocument();
    expect(disconnect(document, 'e3')).toBe(true);
    expect(disconnect(document, 'e3')).toBe(false);
    expect(document.edges.map((edge) => edge.id)).toEqual(['e1', 'e2', 'e4', 'e5']);
  });
});

describe('removing nodes', () => {
  it('removes the nodes, their connections, their positions and the notes named', () => {
    const document = exampleFlowDocument();
    expect(removeNodes(document, ['inventory', 'note-1', 'ghost'])).toEqual(['inventory', 'note-1']);
    expect(document.nodes.map((node) => node.id)).toEqual(['webhook', 'validate', 'slack', 'rejected', 'done']);
    expect(document.edges.map((edge) => edge.id)).toEqual(['e1', 'e3', 'e5']);
    expect(document.editor.nodes?.['inventory']).toBeUndefined();
    expect(document.editor.stickyNotes).toEqual([]);
  });

  it('removes a boundary event with the node it is attached to', () => {
    const document = exampleFlowDocument();
    document.nodes.push({ id: 'timer', name: 'Too slow', element: 'boundaryEvent', type: 'orcon.timer@1', parameters: {}, attachedTo: 'slack' });
    expect(removeNodes(document, ['slack'])).toEqual(['slack', 'timer']);
  });

  it('does nothing when nothing matches', () => {
    const document = exampleFlowDocument();
    expect(removeNodes(document, ['ghost'])).toEqual([]);
    expect(document.nodes).toHaveLength(6);
  });
});

describe('positions', () => {
  it('moves nodes and notes by a distance and lands them on the 20 px grid', () => {
    const document = exampleFlowDocument();
    setPosition(document, 'slack', { x: 647, y: 63 });
    moveNodes(document, ['slack', 'note-1', 'ghost'], 25, -10);
    expect(positionOf(document, 'slack')).toEqual({ x: 680, y: 60 });
    expect(positionOf(document, 'note-1')).toEqual({ x: 60, y: 340 });
    expect(positionOf(document, 'ghost')).toBeUndefined();
  });

  it('keeps a position that is off the grid until the node is moved', () => {
    const document = exampleFlowDocument();
    setPosition(document, 'slack', { x: 647, y: 63 });
    expect(positionOf(document, 'slack')).toEqual({ x: 647, y: 63 });
  });

  it('ignores a position for something that does not exist and moves layout results onto the grid', () => {
    const document = exampleFlowDocument();
    setPosition(document, 'ghost', { x: 1, y: 1 });
    expect(document.editor.nodes?.['ghost']).toBeUndefined();
    applyPositions(document, new Map([['done', { x: 901, y: 18 }], ['note-1', { x: 61, y: 79 }]]));
    expect(positionOf(document, 'done')).toEqual({ x: 900, y: 20 });
    expect(positionOf(document, 'note-1')).toEqual({ x: 60, y: 80 });
  });
});

describe('renaming and disabling', () => {
  it('renames and rewrites references in one step', () => {
    const document = exampleFlowDocument();
    expect(renameNode(document, 'inventory', 'Check stock')).toEqual({ references: 1 });
    expect(findNode(document, 'inventory')?.name).toBe('Check stock');
    expect(String(findNode(document, 'slack')?.parameters['body'])).toContain(`$('Check stock')`);
  });

  it('refuses an empty, a missing and a taken name, and accepts the same name', () => {
    const document = exampleFlowDocument();
    expect(renameNode(document, 'inventory', '   ')).toEqual({ problem: 'empty' });
    expect(renameNode(document, 'ghost', 'x')).toEqual({ problem: 'missing' });
    expect(renameNode(document, 'inventory', 'Slack')).toEqual({ problem: 'taken' });
    expect(renameNode(document, 'inventory', 'HTTP Request')).toEqual({ references: 0 });
  });

  it('disables and enables a node', () => {
    const document = exampleFlowDocument();
    setDisabled(document, 'slack', true);
    expect(findNode(document, 'slack')?.disabled).toBe(true);
    setDisabled(document, 'slack', false);
    expect(findNode(document, 'slack')).not.toHaveProperty('disabled');
    expect(() => {
      setDisabled(document, 'ghost', true);
    }).not.toThrow();
  });
});

describe('adding', () => {
  it('adds a node of a type with its starting blocks, a free name and a position on the grid', () => {
    const document = exampleFlowDocument();
    const id = addNodeOfType(document, def('if'), { x: 449, y: 71 }, 'Validate order');
    expect(findNode(document, id)).toMatchObject({ id: 'add-if-1', name: 'Validate order 2', element: 'exclusiveGateway', type: 'orcon.if@1', routing: 'EXCLUSIVE' });
    expect(positionOf(document, id)).toEqual({ x: 440, y: 80 });
    expect(addNodeOfType(document, def('if'), { x: 0, y: 0 }, 'Another')).toBe('add-if-2');
  });

  it('adds and changes sticky notes', () => {
    const document = exampleFlowDocument();
    const id = addStickyNote(document, { x: 101, y: 99 });
    expect(id).toBe('note-2');
    expect(document.editor.stickyNotes?.[1]).toEqual({ id: 'note-2', x: 100, y: 100, w: 220, h: 120, color: 1, body: '' });
    expect(updateStickyNote(document, id, { body: 'hello', color: 3, title: 'Note' })).toBe(true);
    expect(document.editor.stickyNotes?.[1]).toMatchObject({ body: 'hello', color: 3, title: 'Note' });
    expect(updateStickyNote(document, id, { title: undefined })).toBe(true);
    expect(document.editor.stickyNotes?.[1]).not.toHaveProperty('title');
    expect(updateStickyNote(document, 'ghost', { body: 'x' })).toBe(false);
  });
});
