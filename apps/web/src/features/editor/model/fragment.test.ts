// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { detectClipboardFormat, FLOW_FRAGMENT_FORMAT } from './clipboard-format';
import { exampleFlowDocument } from './example-flow';
import { editorBlock } from './flow-document';
import { buildFragment, flowFragmentSchema, fragmentSize, pasteFragment } from './fragment';

describe('telling what was pasted', () => {
  it('knows an Orcon fragment by its tag', () => {
    expect(detectClipboardFormat({ format: FLOW_FRAGMENT_FORMAT, nodes: [], edges: [], editor: {} })).toBe('orcon-fragment');
    expect(FLOW_FRAGMENT_FORMAT).toBe('orcon/flow-fragment@1');
  });

  it('knows an n8n workflow by its nodes and connections', () => {
    const workflow = { nodes: [{ type: 'n8n-nodes-base.webhook', parameters: {}, position: [0, 0] }], connections: {} };
    expect(detectClipboardFormat(workflow)).toBe('n8n-workflow');
  });

  it('does not take anything else for nodes', () => {
    expect(detectClipboardFormat(null)).toBe('unknown');
    expect(detectClipboardFormat([1, 2])).toBe('unknown');
    expect(detectClipboardFormat('text')).toBe('unknown');
    expect(detectClipboardFormat({})).toBe('unknown');
    expect(detectClipboardFormat({ nodes: [], connections: {} })).toBe('unknown');
    expect(detectClipboardFormat({ nodes: [{ type: 3 }], connections: {} })).toBe('unknown');
    expect(detectClipboardFormat({ nodes: [{ type: 'x' }] })).toBe('unknown');
    expect(detectClipboardFormat({ format: 'orcon/flow@1', nodes: [], edges: [] })).toBe('unknown');
    expect(detectClipboardFormat({ format: 'orcon/flow-fragment@2' })).toBe('unknown');
  });
});

describe('copying', () => {
  it('copies the selected nodes, the connections between them, their positions and the notes', () => {
    const fragment = buildFragment(exampleFlowDocument(), new Set(['inventory', 'slack', 'note-1']));
    expect(fragment.format).toBe(FLOW_FRAGMENT_FORMAT);
    expect(fragment.nodes.map((node) => node.id)).toEqual(['inventory', 'slack']);
    expect(fragment.edges.map((edge) => edge.id)).toEqual(['e4']);
    expect(fragment.editor.nodes).toEqual({ inventory: { x: 440, y: 60 }, slack: { x: 640, y: 60 } });
    expect(fragment.editor.stickyNotes?.map((note) => note.id)).toEqual(['note-1']);
    expect(fragmentSize(fragment)).toBe(3);
    expect(flowFragmentSchema.safeParse(fragment).success).toBe(true);
  });

  it('keeps credentials by name only', () => {
    const document = exampleFlowDocument();
    const slack = document.nodes.find((node) => node.id === 'slack');
    if (!slack) throw new Error('missing');
    slack['credentials'] = { auth: { name: 'Slack webhook', id: 'secret-id', token: 'abc' }, other: 5 };
    const fragment = buildFragment(document, new Set(['slack']));
    expect(fragment.nodes[0]?.['credentials']).toEqual({ auth: { name: 'Slack webhook' }, other: { name: undefined } });
    expect(JSON.stringify(fragment)).not.toContain('abc');
    expect(slack['credentials']).toMatchObject({ auth: { token: 'abc' } });
  });

  it('places nodes that have no recorded position', () => {
    const document = exampleFlowDocument();
    document.editor = {};
    const fragment = buildFragment(document, new Set(['validate']));
    expect(fragment.editor.nodes).toEqual({ validate: { x: 200, y: 0 } });
  });
});

describe('pasting', () => {
  it('adds new nodes with new ids, free names and connections between them, moved to the pointer', () => {
    const document = exampleFlowDocument();
    const fragment = buildFragment(document, new Set(['inventory', 'slack', 'note-1']));
    const result = pasteFragment(document, fragment, { at: { x: 1003, y: 421 } });

    expect(result).toEqual({ nodeIds: ['inventory-1', 'slack-1'], noteIds: ['note-2'] });
    expect(document.nodes.slice(-2).map((node) => [node.id, node.name])).toEqual([
      ['inventory-1', 'HTTP Request 2'],
      ['slack-1', 'Slack 2'],
    ]);
    expect(document.edges.at(-1)).toMatchObject({ id: 'edge-1', from: { node: 'inventory-1', port: 'main' }, to: { node: 'slack-1' } });
    // The top left corner of the pasted items (note at 40,340 is the left-most and the node at 440,60 the top-most).
    expect(document.editor.nodes?.['inventory-1']).toEqual({ x: 1000 + 400, y: 420 });
    expect(document.editor.nodes?.['slack-1']).toEqual({ x: 1000 + 600, y: 420 });
    expect(document.editor.stickyNotes?.at(-1)).toMatchObject({ id: 'note-2', x: 1000, y: 700 });
  });

  it('pastes offset from the originals when no point is given, and again with new names', () => {
    const document = exampleFlowDocument();
    const fragment = buildFragment(document, new Set(['slack']));
    pasteFragment(document, fragment, {});
    pasteFragment(document, fragment, { offset: { x: 80, y: 0 } });
    expect(document.nodes.slice(-2).map((node) => node.name)).toEqual(['Slack 2', 'Slack 3']);
    expect(document.editor.nodes?.['slack-1']).toEqual({ x: 680, y: 100 });
    expect(document.editor.nodes?.['slack-2']).toEqual({ x: 720, y: 60 });
  });

  it('remaps the node a boundary event is attached to, drops connections to nodes outside the fragment and copies the unsupported placeholder', () => {
    const document = exampleFlowDocument();
    document.nodes.push({ id: 'old', name: 'Old node', element: 'task', type: 'orcon.unsupported@1', parameters: {} });
    const fragment = flowFragmentSchema.parse({
      format: FLOW_FRAGMENT_FORMAT,
      nodes: [
        { id: 'a', name: 'A', element: 'task', type: 'orcon.noop@1', parameters: {} },
        { id: 'b', name: 'B', element: 'boundaryEvent', type: 'orcon.timer@1', parameters: {}, attachedTo: 'a' },
        { id: 'old', name: 'Old node', element: 'task', type: 'orcon.unsupported@1', parameters: {} },
      ],
      edges: [
        { id: 'x', from: { node: 'a', port: 'main' }, to: { node: 'outside' }, kind: 'SEQUENCE' },
        { id: 'y', from: { node: 'a', port: 'main' }, to: { node: 'old' }, kind: 'SEQUENCE' },
      ],
      editor: {},
    });
    const result = pasteFragment(document, fragment, { at: { x: 0, y: 0 } });
    expect(result.nodeIds).toEqual(['a-1', 'b-1', 'old-1']);
    expect(document.nodes.find((node) => node.id === 'b-1')?.['attachedTo']).toBe('a-1');
    expect(document.nodes.find((node) => node.id === 'old-1')).toMatchObject({ type: 'orcon.unsupported@1', name: 'Old node 2' });
    expect(document.edges.map((edge) => edge.id)).toEqual(['e1', 'e2', 'e3', 'e4', 'e5', 'edge-1']);
  });

  it('creates the editor members and pastes nothing from an empty fragment', () => {
    const document = exampleFlowDocument();
    document.editor = {};
    const result = pasteFragment(document, flowFragmentSchema.parse({ format: FLOW_FRAGMENT_FORMAT, nodes: [], edges: [], editor: {} }), { at: { x: 40, y: 40 } });
    expect(result).toEqual({ nodeIds: [], noteIds: [] });
    expect(editorBlock(document)).toEqual({ nodes: {}, stickyNotes: [] });
  });

  it('refuses a malformed fragment', () => {
    expect(flowFragmentSchema.safeParse({ format: FLOW_FRAGMENT_FORMAT, nodes: [{ id: 'bad id' }], edges: [], editor: {} }).success).toBe(false);
  });
});
