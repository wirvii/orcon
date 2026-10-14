// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { detectClipboardFormat, FLOW_FRAGMENT_FORMAT } from './clipboard-format';
import { exampleFlowDocument } from './example-flow';
import { editorBlock } from './flow-document';
import { buildFragment, COORDINATE_LIMIT, flowFragmentSchema, fragmentSize, pasteFragment } from './fragment';

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

    expect(result).toEqual({ nodeIds: ['inventory-1', 'slack-1'], noteIds: ['note-2'], droppedEdges: 0, placeholders: 0 });
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
    expect(result).toEqual({ nodeIds: [], noteIds: [], droppedEdges: 0, placeholders: 0 });
    expect(editorBlock(document)).toEqual({ nodes: {}, stickyNotes: [] });
  });

  describe('a fragment is untrusted text: it passes the rules of a connection made by hand', () => {
    const node = (id: string, type: string, element = 'task') => ({ id, name: id.toUpperCase(), element, type, parameters: {} });
    const edge = (id: string, from: string, to: string, port = 'main') => ({ id, from: { node: from, port }, to: { node: to }, kind: 'SEQUENCE' });
    const fragmentOf = (nodes: unknown[], edges: unknown[], editor: unknown = {}) => flowFragmentSchema.parse({ format: FLOW_FRAGMENT_FORMAT, nodes, edges, editor });

    it('drops a connection that closes a cycle and says how many', () => {
      const document = exampleFlowDocument();
      const result = pasteFragment(document, fragmentOf([node('a', 'orcon.http@1'), node('b', 'orcon.http@1')], [edge('1', 'a', 'b'), edge('2', 'b', 'a')]), { at: { x: 0, y: 0 } });
      expect(result.droppedEdges).toBe(1);
      expect(document.edges.slice(5).map((added) => [added.from.node, added.to.node, added.kind])).toEqual([['a-1', 'b-1', 'SEQUENCE']]);
    });

    it('keeps a cycle that goes back into a loop node, as a back edge', () => {
      const document = exampleFlowDocument();
      const result = pasteFragment(document, fragmentOf([node('l', 'orcon.loop@1'), node('b', 'orcon.http@1')], [edge('1', 'l', 'b', 'loop'), edge('2', 'b', 'l')]), { at: { x: 0, y: 0 } });
      expect(result.droppedEdges).toBe(0);
      expect(document.edges.at(-1)).toMatchObject({ kind: 'BACK' });
    });

    it('drops a connection into a trigger, a connection to itself and a duplicate', () => {
      const document = exampleFlowDocument();
      const result = pasteFragment(
        document,
        fragmentOf([node('a', 'orcon.http@1'), node('t', 'orcon.start@1', 'startEvent'), node('b', 'orcon.http@1')], [edge('1', 'a', 't'), edge('2', 'a', 'a'), edge('3', 'a', 'b'), edge('4', 'a', 'b')]),
        { at: { x: 0, y: 0 } },
      );
      expect(result.droppedEdges).toBe(3);
      expect(document.edges).toHaveLength(6);
    });

    it('adds a node of a type this editor does not know as the disabled placeholder, keeping the original type', () => {
      const document = exampleFlowDocument();
      const result = pasteFragment(document, fragmentOf([node('e', 'evil.unknown@9'), node('k', 'orcon.http@1')], []), { at: { x: 0, y: 0 } });
      expect(result.placeholders).toBe(1);
      expect(document.nodes.find((added) => added.id === 'e-1')).toMatchObject({ type: 'orcon.unsupported@1', disabled: true, ext: { 'orcon.import': { originalType: 'evil.unknown@9' } } });
      expect(document.nodes.find((added) => added.id === 'k-1')?.type).toBe('orcon.http@1');
    });

    it('keeps a type the flow already uses', () => {
      const document = exampleFlowDocument();
      document.nodes.push({ id: 'z', name: 'Z', element: 'task', type: 'acme.custom@1', parameters: {} });
      expect(pasteFragment(document, fragmentOf([node('c', 'acme.custom@1')], []), { at: { x: 0, y: 0 } }).placeholders).toBe(0);
    });

    it('snaps positions to the grid and keeps them within reach, however far the fragment says', () => {
      const document = exampleFlowDocument();
      pasteFragment(
        document,
        fragmentOf([node('a', 'orcon.http@1'), node('b', 'orcon.http@1')], [], {
          nodes: { a: { x: 1e9, y: 3.7 }, b: { x: -1e12, y: 5e8 } },
          stickyNotes: [{ id: 'n', x: 1e15, y: 7, w: 1e9, h: 80, color: 1, body: '' }],
        }),
        { offset: { x: 33, y: 47 } },
      );
      const positions = [document.editor.nodes?.['a-1'], document.editor.nodes?.['b-1'], document.editor.stickyNotes?.at(-1)];
      for (const at of positions) {
        expect(Math.abs(at?.x ?? Infinity)).toBeLessThanOrEqual(COORDINATE_LIMIT);
        expect(Math.abs(at?.y ?? Infinity)).toBeLessThanOrEqual(COORDINATE_LIMIT);
        expect(Math.abs(at?.x ?? 1) % 20).toBe(0);
        expect(Math.abs(at?.y ?? 1) % 20).toBe(0);
      }
      expect(document.editor.stickyNotes?.at(-1)?.w).toBe(5000);
    });
  });

  it('refuses a malformed fragment', () => {
    expect(flowFragmentSchema.safeParse({ format: FLOW_FRAGMENT_FORMAT, nodes: [{ id: 'bad id' }], edges: [], editor: {} }).success).toBe(false);
  });
});

describe('a Call sub-flow node', () => {
  it('pastes as itself, with its call block, and not as the placeholder of an unknown type', () => {
    const document = exampleFlowDocument();
    const call = { target: '01930000-0000-7000-8000-00000000ed61', version: 3, mode: 'WAIT', perItem: true };
    const fragment = flowFragmentSchema.parse({
      format: FLOW_FRAGMENT_FORMAT,
      nodes: [{ id: 'ship', name: 'Ship order', element: 'callActivity', type: 'orcon.callFlow@1', parameters: {}, call }],
      edges: [],
      editor: { nodes: { ship: { x: 0, y: 0 } } },
    });
    const result = pasteFragment(document, fragment, { offset: { x: 0, y: 0 } });
    expect(result.placeholders).toBe(0);
    expect(document.nodes.find((node) => node.id === result.nodeIds[0])).toMatchObject({ type: 'orcon.callFlow@1', element: 'callActivity', call });
  });
});
