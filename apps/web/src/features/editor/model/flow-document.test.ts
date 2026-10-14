// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { EXAMPLE_FLOW_ID, exampleFlowDocument } from './example-flow';
import { cloneDocument, deepCopy, editorBlock, findNode, nextId, parseFlowDocument, takenIds } from './flow-document';

describe('the flow document', () => {
  it('accepts the sample flow and keeps members it does not know', () => {
    const source = { ...exampleFlowDocument(), ext: { 'acme.note': 1 } };
    const parsed = parseFlowDocument(source);
    expect(parsed.ok).toBe(true);
    if (parsed.ok) expect(parsed.document['ext']).toEqual({ 'acme.note': 1 });
  });

  it('names the pointer of the first problem', () => {
    const broken = exampleFlowDocument();
    const second = broken.nodes[1];
    if (!second) throw new Error('The sample flow has a second node.');
    second.type = 'not a type';
    const parsed = parseFlowDocument(broken);
    expect(parsed).toEqual({ ok: false, message: expect.stringContaining('/nodes/1/type') as string });
  });

  it('refuses a value that is not a document', () => {
    expect(parseFlowDocument(null).ok).toBe(false);
    expect(parseFlowDocument({ format: 'orcon/flow@2' }).ok).toBe(false);
  });

  it('opens the sample flow under its stable id', () => {
    expect(exampleFlowDocument().id).toBe(EXAMPLE_FLOW_ID);
    expect(findNode(exampleFlowDocument(), 'slack')?.name).toBe('Slack');
    expect(findNode(exampleFlowDocument(), 'missing')).toBeUndefined();
  });

  it('copies deeply and creates the editor members on demand', () => {
    const original = exampleFlowDocument();
    const copy = cloneDocument(original);
    expect(copy).toEqual(original);
    expect(copy.nodes[0]).not.toBe(original.nodes[0]);
    expect(deepCopy([1, { a: [2] }, null])).toEqual([1, { a: [2] }, null]);
    const bare = cloneDocument({ ...original, editor: {} });
    expect(editorBlock(bare)).toEqual({ nodes: {}, stickyNotes: [] });
  });

  it('finds the first free id of a stem across nodes, edges and notes', () => {
    const document = exampleFlowDocument();
    expect(takenIds(document)).toEqual(new Set(['webhook', 'validate', 'inventory', 'slack', 'rejected', 'done', 'e1', 'e2', 'e3', 'e4', 'e5', 'note-1']));
    expect(nextId('note', takenIds(document))).toBe('note-2');
    expect(nextId('http', takenIds(document))).toBe('http-1');
    expect(nextId('x'.repeat(80), new Set())).toBe(`${'x'.repeat(56)}-1`);
  });
});
