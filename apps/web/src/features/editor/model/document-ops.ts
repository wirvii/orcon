// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { fallbackPositions, snap } from './canvas-graph';
import { codePointLength, editorBlock, findNode, MAX_NODE_NOTE, nextId, takenIds, type FlowDocument, type FlowEdge, type FlowEdgeKind, type FlowNode, type Point, type StickyNote } from './flow-document';
import { namesOf, uniqueName } from './names';
import { LOOP_TYPE, START_TYPE, type NodeTypeDef } from './node-types';
import { renameReferences } from './rename-references';

/**
 * The edits of a document, as plain functions that change the document they are given. The store
 * runs each inside one Immer recipe (so it records one history entry); tests run them on a
 * `cloneDocument` copy. Nothing here knows about React or the canvas.
 */

export type EdgeProblem = 'unknown-node' | 'same-node' | 'into-start' | 'duplicate' | 'cycle';

/** The nodes that can be reached from `from` following forward (SEQUENCE) edges, `from` included. */
function forwardReach(document: FlowDocument, from: string): Set<string> {
  const reached = new Set([from]);
  const queue = [from];
  while (queue.length > 0) {
    const current = queue.pop() ?? '';
    for (const edge of document.edges) {
      if (edge.kind === 'BACK' || edge.from.node !== current || reached.has(edge.to.node)) continue;
      reached.add(edge.to.node);
      queue.push(edge.to.node);
    }
  }
  return reached;
}

/**
 * Why a connection from `source` (port `port`) to `target` is not allowed, or undefined when it is.
 * The rules are SPEC-014 R-ED-CAN-01 and the flow format: nothing goes into a start event, and a
 * cycle exists only as a BACK edge into an `orcon.loop` node (ADR-0007 section 4).
 */
export function edgeProblem(document: FlowDocument, source: string, port: string, target: string): EdgeProblem | undefined {
  const from = findNode(document, source);
  const to = findNode(document, target);
  if (!from || !to) return 'unknown-node';
  if (from.id === to.id) return 'same-node';
  if (to.type === START_TYPE) return 'into-start';
  if (document.edges.some((edge) => edge.from.node === source && edge.from.port === port && edge.to.node === target)) return 'duplicate';
  const closesCycle = forwardReach(document, target).has(source);
  if (closesCycle && to.type !== LOOP_TYPE) return 'cycle';
  return undefined;
}

/** SEQUENCE, or BACK when the connection returns to a loop node it follows from. */
export function edgeKindFor(document: FlowDocument, source: string, target: string): FlowEdgeKind {
  const to = findNode(document, target);
  return to?.type === LOOP_TYPE && forwardReach(document, target).has(source) ? 'BACK' : 'SEQUENCE';
}

/** Adds a connection. Returns the problem when it is not allowed. */
export function connect(document: FlowDocument, source: string, port: string, target: string, id?: string): { edge: FlowEdge } | { problem: EdgeProblem } {
  const problem = edgeProblem(document, source, port, target);
  if (problem) return { problem };
  const edge: FlowEdge = {
    id: id ?? nextId(`edge-${source}-${target}`, takenIds(document)),
    from: { node: source, port },
    to: { node: target },
    kind: edgeKindFor(document, source, target),
  };
  document.edges.push(edge);
  return { edge };
}

export function disconnect(document: FlowDocument, edgeId: string): boolean {
  const index = document.edges.findIndex((edge) => edge.id === edgeId);
  if (index < 0) return false;
  document.edges.splice(index, 1);
  return true;
}

/** Removes nodes and sticky notes by id, with the connections of the nodes. Returns the ids that existed. */
export function removeNodes(document: FlowDocument, ids: readonly string[]): string[] {
  const gone = new Set(ids);
  const removed = [...document.nodes.filter((node) => gone.has(node.id)).map((node) => node.id), ...(document.editor.stickyNotes ?? []).filter((note) => gone.has(note.id)).map((note) => note.id)];
  if (removed.length === 0) return removed;
  document.nodes = document.nodes.filter((node) => !gone.has(node.id));
  document.edges = document.edges.filter((edge) => !gone.has(edge.from.node) && !gone.has(edge.to.node));
  const editor = editorBlock(document);
  for (const id of removed) Reflect.deleteProperty(editor.nodes, id);
  document.editor.stickyNotes = editor.stickyNotes.filter((note) => !gone.has(note.id));
  // A node attached to a removed node (a boundary event) cannot stay.
  const attached = document.nodes.filter((node) => typeof node['attachedTo'] === 'string' && gone.has(node['attachedTo'])).map((node) => node.id);
  if (attached.length > 0) removed.push(...removeNodes(document, attached));
  return removed;
}

/** Records the position of a node or sticky note. */
export function setPosition(document: FlowDocument, id: string, at: Point): void {
  const editor = editorBlock(document);
  const note = editor.stickyNotes.find((candidate) => candidate.id === id);
  if (note) {
    note.x = at.x;
    note.y = at.y;
  } else if (findNode(document, id)) {
    // The entry may hold a note: only the position changes.
    editor.nodes[id] = { ...editor.nodes[id], x: at.x, y: at.y };
  }
}

/** The recorded position of a node or note. */
export function positionOf(document: FlowDocument, id: string): Point | undefined {
  const note = document.editor.stickyNotes?.find((candidate) => candidate.id === id);
  if (note) return { x: note.x, y: note.y };
  return document.editor.nodes?.[id];
}

/** Moves nodes and notes by a distance; every moved item lands on the 20 px grid (R-ED-CAN-02). */
export function moveNodes(document: FlowDocument, ids: readonly string[], dx: number, dy: number): void {
  for (const id of ids) {
    const at = positionOf(document, id);
    if (at) setPosition(document, id, { x: snap(at.x + dx), y: snap(at.y + dy) });
  }
}

export type RenameProblem = 'missing' | 'empty' | 'taken';

/** Renames a node and rewrites the references to it in expressions. Returns how many references changed. */
export function renameNode(document: FlowDocument, id: string, name: string): { references: number } | { problem: RenameProblem } {
  const node = findNode(document, id);
  if (!node) return { problem: 'missing' };
  const next = name.trim();
  if (next === '') return { problem: 'empty' };
  if (next === node.name) return { references: 0 };
  if (namesOf(document, id).has(next)) return { problem: 'taken' };
  const previous = node.name;
  node.name = next;
  return { references: renameReferences(document, previous, next) };
}

export function setDisabled(document: FlowDocument, id: string, disabled: boolean): void {
  const node = findNode(document, id);
  if (!node) return;
  if (disabled) node.disabled = true;
  else delete node.disabled;
}

/** Sets one parameter of a node; `undefined` removes it. Returns false when the node does not exist. */
export function setParameter(document: FlowDocument, id: string, name: string, value: unknown): boolean {
  const node = findNode(document, id);
  if (!node) return false;
  if (value === undefined) Reflect.deleteProperty(node.parameters, name);
  else node.parameters[name] = value;
  return true;
}

/** The behaviour and binding blocks a node's configuration form writes (orcon-flow-1.md "Node blocks", ADR-0034 §1, ADR-0057 §4). */
export type NodeBlock = 'retry' | 'timeouts' | 'onError' | 'concurrency' | 'attention' | 'call' | 'connector' | 'credentials';

/** Replaces one block of a node; `undefined` removes it. Returns false when the node does not exist. */
export function setNodeBlock(document: FlowDocument, id: string, block: NodeBlock, value: unknown): boolean {
  const node = findNode(document, id);
  if (!node) return false;
  if (value === undefined) Reflect.deleteProperty(node, block);
  else node[block] = value;
  return true;
}

/**
 * Sets the note of a node (`editor.nodes[<id>].note`); an empty or `undefined` text removes it, so the
 * document never holds an empty note. Returns false when the node does not exist or the text is longer
 * than the limit.
 */
export function setNodeNote(document: FlowDocument, id: string, text: string | undefined): boolean {
  if (!findNode(document, id)) return false;
  const editor = editorBlock(document);
  const entry = editor.nodes[id];
  if (text === undefined || text === '') {
    if (entry) Reflect.deleteProperty(entry, 'note');
    return true;
  }
  if (codePointLength(text) > MAX_NODE_NOTE) return false;
  if (entry) entry.note = text;
  else {
    // A node with no recorded position keeps where the canvas draws it by default.
    const at = fallbackPositions(document).get(id) ?? { x: 0, y: 0 };
    editor.nodes[id] = { x: at.x, y: at.y, note: text };
  }
  return true;
}

/**
 * The credential a node holds at `path` of its parameters: the identifier of a stored credential, which the
 * engine reads from there when the step runs (`credential` for most steps, `model.credential` for a model).
 */
export function credentialAt(node: Pick<FlowNode, 'parameters'>, path: readonly string[]): string | undefined {
  let value: unknown = node.parameters;
  for (const key of path) {
    if (typeof value !== 'object' || value === null || Array.isArray(value)) return undefined;
    value = (value as Record<string, unknown>)[key];
  }
  return typeof value === 'string' && value !== '' ? value : undefined;
}

/**
 * Chooses the credential of a node by identifier at `path` of its parameters; `undefined` clears it. The object a
 * path goes through keeps its other members (a model keeps its provider and name), and is dropped when clearing
 * leaves it empty. Choosing what the node already holds is not an edit. Returns false when the node does not exist.
 */
export function setCredentialParameter(document: FlowDocument, id: string, path: readonly string[], value: string | undefined): boolean {
  const node = findNode(document, id);
  if (!node) return false;
  const [head, ...rest] = path;
  if (head === undefined) return false;
  if (credentialAt(node, path) === value) return true;
  if (rest.length === 0) return setParameter(document, id, head, value);
  const held = node.parameters[head];
  const object: Record<string, unknown> = typeof held === 'object' && held !== null && !Array.isArray(held) ? { ...(held as Record<string, unknown>) } : {};
  const last = rest[rest.length - 1] ?? head;
  if (value === undefined) Reflect.deleteProperty(object, last);
  else object[last] = value;
  return setParameter(document, id, head, Object.keys(object).length === 0 ? undefined : object);
}

/** Pins a node to another version of its connector: one edit, whatever the version range holds. */
export function pinConnector(document: FlowDocument, id: string, version: string, digest: string): boolean {
  return setNodeBlock(document, id, 'connector', { version, digest });
}

/** Adds a node of a type at a position and returns its id (`add-<slug>-<n>`). */
export function addNodeOfType(document: FlowDocument, def: NodeTypeDef, at: Point, name: string, id?: string): string {
  const nodeId = id ?? nextId(`add-${def.slug}`, takenIds(document));
  const initial = def.initial();
  const node: FlowNode = { id: nodeId, name: uniqueName(name, namesOf(document)), element: def.element, type: def.type, ...initial };
  document.nodes.push(node);
  editorBlock(document).nodes[nodeId] = { x: snap(at.x), y: snap(at.y) };
  return nodeId;
}

export const DEFAULT_NOTE_SIZE = { w: 220, h: 120 } as const;

/** Adds an empty sticky note at a position and returns its id. */
export function addStickyNote(document: FlowDocument, at: Point, id?: string): string {
  const noteId = id ?? nextId('note', takenIds(document));
  const note: StickyNote = { id: noteId, x: snap(at.x), y: snap(at.y), w: DEFAULT_NOTE_SIZE.w, h: DEFAULT_NOTE_SIZE.h, color: 1, body: '' };
  editorBlock(document).stickyNotes.push(note);
  return noteId;
}

export type NotePatch = Partial<Pick<StickyNote, 'x' | 'y' | 'w' | 'h' | 'color' | 'title' | 'body'>>;

export function updateStickyNote(document: FlowDocument, id: string, patch: NotePatch): boolean {
  const note = document.editor.stickyNotes?.find((candidate) => candidate.id === id);
  if (!note) return false;
  Object.assign(note, patch);
  if (patch.title === undefined && 'title' in patch) delete note.title;
  return true;
}

/** Moves the items of a layout to their new positions. */
export function applyPositions(document: FlowDocument, positions: ReadonlyMap<string, Point>): void {
  for (const [id, at] of positions) setPosition(document, id, { x: snap(at.x), y: snap(at.y) });
}
