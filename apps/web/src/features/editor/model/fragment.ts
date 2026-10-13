// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { fallbackPositions, snap } from './canvas-graph';
import { FLOW_FRAGMENT_FORMAT } from './clipboard-format';
import { deepCopy, editorBlock, editorSchema, flowEdgeSchema, flowNodeSchema, nextId, takenIds, type FlowDocument, type FlowEdge, type FlowNode, type Point, type StickyNote } from './flow-document';
import { namesOf, uniqueName } from './names';

/**
 * `orcon/flow-fragment@1`: the text the editor copies (SPEC-014 R-ED-CLIP-01). Selected nodes with
 * their parameters, the connections between them, their editor positions and the sticky notes.
 * Credentials are kept by name only and pinned data is never included, so a fragment is safe to
 * share in a forum or a pull request.
 */

export const flowFragmentSchema = z.looseObject({
  format: z.literal(FLOW_FRAGMENT_FORMAT),
  nodes: z.array(flowNodeSchema),
  edges: z.array(flowEdgeSchema),
  editor: editorSchema,
});

export type FlowFragment = z.infer<typeof flowFragmentSchema>;

/** The most nodes one fragment may bring into a flow. */
export const MAX_FRAGMENT_NODES = 500;

function nameOnlyCredentials(node: FlowNode): void {
  const credentials = node['credentials'];
  if (typeof credentials !== 'object' || credentials === null) return;
  node['credentials'] = Object.fromEntries(
    Object.entries(credentials as Record<string, unknown>).map(([slot, reference]) => [slot, { name: typeof reference === 'object' && reference !== null && 'name' in reference ? reference.name : undefined }]),
  );
}

/** The fragment for the selected node and sticky note ids. */
export function buildFragment(document: FlowDocument, ids: ReadonlySet<string>): FlowFragment {
  const fallback = document.nodes.some((node) => document.editor.nodes?.[node.id] === undefined) ? fallbackPositions(document) : undefined;
  const nodes = document.nodes.filter((node) => ids.has(node.id)).map((node) => deepCopy(node));
  nodes.forEach(nameOnlyCredentials);
  const positions: Record<string, Point> = {};
  for (const node of nodes) {
    const at = document.editor.nodes?.[node.id] ?? fallback?.get(node.id) ?? { x: 0, y: 0 };
    positions[node.id] = { x: at.x, y: at.y };
  }
  return {
    format: FLOW_FRAGMENT_FORMAT,
    nodes,
    edges: document.edges.filter((edge) => ids.has(edge.from.node) && ids.has(edge.to.node)).map((edge) => deepCopy(edge)),
    editor: { nodes: positions, stickyNotes: (document.editor.stickyNotes ?? []).filter((note) => ids.has(note.id)).map((note) => ({ ...note })) },
  };
}

/** The id a copy of `id` starts from: the id without a trailing `-<n>`. */
const stemOf = (id: string): string => id.replace(/-\d+$/, '') || 'node';

export interface PasteOptions {
  /** Where the top left corner of the pasted items goes. */
  at?: Point | undefined;
  /** How far from where they were copied the items go, when `at` is not given. */
  offset?: Point | undefined;
}

export interface PasteResult {
  nodeIds: string[];
  noteIds: string[];
}

/** The ids a fragment brings: its nodes and its sticky notes. */
export const fragmentSize = (fragment: FlowFragment): number => fragment.nodes.length + (fragment.editor.stickyNotes?.length ?? 0);

/**
 * Adds a fragment to the document with new ids, names that are free in the flow ("Send email 2")
 * and positions moved to `at` (or by `offset`). One call is one history entry for the store.
 */
export function pasteFragment(document: FlowDocument, fragment: FlowFragment, options: PasteOptions): PasteResult {
  const taken = takenIds(document);
  const names = namesOf(document);
  const editor = editorBlock(document);
  const sourcePositions = fragment.editor.nodes ?? {};
  const notes = fragment.editor.stickyNotes ?? [];
  const origins = [...fragment.nodes.map((node) => sourcePositions[node.id] ?? { x: 0, y: 0 }), ...notes.map((note) => ({ x: note.x, y: note.y }))];
  const left = origins.length === 0 ? 0 : Math.min(...origins.map((point) => point.x));
  const top = origins.length === 0 ? 0 : Math.min(...origins.map((point) => point.y));
  const delta: Point = options.at ? { x: snap(options.at.x) - left, y: snap(options.at.y) - top } : (options.offset ?? { x: 40, y: 40 });

  const idMap = new Map<string, string>();
  for (const node of fragment.nodes) {
    const id = nextId(stemOf(node.id), taken);
    taken.add(id);
    idMap.set(node.id, id);
  }
  const remap = (value: unknown): unknown => (typeof value === 'string' ? (idMap.get(value) ?? value) : value);

  const nodeIds: string[] = [];
  for (const source of fragment.nodes) {
    const node: FlowNode = deepCopy(source);
    node.id = idMap.get(source.id) ?? source.id;
    node.name = uniqueName(source.name, names);
    names.add(node.name);
    if ('attachedTo' in node) node['attachedTo'] = remap(node['attachedTo']);
    document.nodes.push(node);
    const at = sourcePositions[source.id] ?? { x: 0, y: 0 };
    editor.nodes[node.id] = { x: at.x + delta.x, y: at.y + delta.y };
    nodeIds.push(node.id);
  }

  for (const source of fragment.edges) {
    const from = idMap.get(source.from.node);
    const to = idMap.get(source.to.node);
    if (!from || !to) continue;
    const id = nextId('edge', taken);
    taken.add(id);
    const edge: FlowEdge = { ...deepCopy(source), id, from: { ...source.from, node: from }, to: { ...source.to, node: to } };
    document.edges.push(edge);
  }

  const noteIds: string[] = [];
  for (const source of notes) {
    const id = nextId('note', taken);
    taken.add(id);
    const note: StickyNote = { ...source, id, x: source.x + delta.x, y: source.y + delta.y };
    editor.stickyNotes.push(note);
    noteIds.push(id);
  }
  return { nodeIds, noteIds };
}
