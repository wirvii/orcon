// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { CanvasGraph, CanvasNode, CanvasNote } from './canvas-graph';
import { connect, disconnect, edgeProblem, removeNodes, renameNode, setDisabled, setPosition, type EdgeProblem, type RenameProblem } from './document-ops';
import { deepCopy, editorBlock, findNode, nextId, takenIds, type FlowDocument, type FlowNode, type StickyNote } from './flow-document';
import { namesOf, uniqueName } from './names';
import { NODE_TYPES, slugOfAddedId, typeDefBySlug } from './node-types';

/**
 * Turns a change the canvas reports into edits of the document. The canvas (`WorkflowCanvas`) keeps
 * its own copy of the graph and reports each change as a whole new graph; this compares the new
 * graph with the one it replaced and applies the difference to the document, so the document stays
 * the source of truth. What the document cannot hold is refused and reported, and the screen then
 * rebuilds the canvas from the document.
 */

export interface EditReport {
  renamed: { id: string; from: string; to: string; references: number }[];
  renameProblems: { id: string; problem: RenameProblem }[];
  edgeProblems: { id: string; problem: EdgeProblem }[];
  /** New nodes whose name was taken: the document gave them a free name, which the canvas must show too. */
  adjustedNames: { id: string; name: string }[];
  /** Nodes whose output the canvas pinned; pinning arrives with the test data slices and is not applied. */
  pinIgnored: string[];
  /** The canvas shows something the document does not hold (a refused change, a pin): it must be rebuilt from the document. */
  divergent: boolean;
}

const sameSize = (a: CanvasNote, b: CanvasNote) => a.size.w === b.size.w && a.size.h === b.size.h;

/** The existing node a new node is a copy of: its id is that node's id followed by `-<n>`. */
function copySource(document: FlowDocument, id: string): FlowNode | undefined {
  let best: FlowNode | undefined;
  for (const node of document.nodes) {
    if (!id.startsWith(`${node.id}-`) || !/^\d+$/.test(id.slice(node.id.length + 1))) continue;
    if (!best || node.id.length > best.id.length) best = node;
  }
  return best;
}

function createNode(document: FlowDocument, canvasNode: CanvasNode, report: EditReport): void {
  const copy = copySource(document, canvasNode.id);
  const slug = slugOfAddedId(canvasNode.id);
  const def = slug ? typeDefBySlug(slug) : undefined;
  const fallback = NODE_TYPES.find((candidate) => candidate.slug === 'noop');
  let node: FlowNode;
  if (copy) {
    node = { ...deepCopy(copy), id: canvasNode.id };
  } else {
    const used = def ?? fallback;
    if (!used) return;
    node = { id: canvasNode.id, name: canvasNode.name, element: used.element, type: used.type, ...used.initial() };
  }
  const name = uniqueName(canvasNode.name, namesOf(document));
  if (name !== canvasNode.name) report.adjustedNames.push({ id: canvasNode.id, name });
  node.name = name;
  if (canvasNode.disabled === true) node.disabled = true;
  else delete node.disabled;
  document.nodes.push(node);
  setPosition(document, node.id, canvasNode.position);
}

function changeNode(document: FlowDocument, before: CanvasNode, after: CanvasNode, report: EditReport): void {
  if (after.name !== before.name) {
    const result = renameNode(document, after.id, after.name);
    if ('problem' in result) {
      report.renameProblems.push({ id: after.id, problem: result.problem });
      report.divergent = true;
    } else {
      report.renamed.push({ id: after.id, from: before.name, to: after.name, references: result.references });
    }
  }
  if ((after.disabled === true) !== (before.disabled === true)) {
    // A placeholder is always disabled and cannot be enabled.
    if (findNode(document, after.id)?.type === 'orcon.unsupported@1') report.divergent = true;
    else setDisabled(document, after.id, after.disabled === true);
  }
  if (after.position.x !== before.position.x || after.position.y !== before.position.y) setPosition(document, after.id, after.position);
  // Pinned output belongs to the later test-data slices; the canvas must not show a pin the document does not have.
  if ((after.pinned === true) !== (before.pinned === true)) {
    report.pinIgnored.push(after.id);
    report.divergent = true;
  }
}

function syncNotes(document: FlowDocument, previous: readonly CanvasNote[], next: readonly CanvasNote[]): void {
  const before = new Map(previous.map((note) => [note.id, note]));
  const after = new Map(next.map((note) => [note.id, note]));
  const gone = previous.filter((note) => !after.has(note.id)).map((note) => note.id);
  if (gone.length > 0) removeNodes(document, gone);
  const editor = editorBlock(document);
  for (const note of next) {
    const old = before.get(note.id);
    if (old === note) continue;
    const stored = editor.stickyNotes.find((candidate) => candidate.id === note.id);
    if (!stored) {
      const created: StickyNote = { id: note.id, x: note.position.x, y: note.position.y, w: note.size.w, h: note.size.h, color: note.color, body: note.body.slice(0, 10_000) };
      if (note.title !== undefined) created.title = note.title;
      editor.stickyNotes.push(created);
      continue;
    }
    if (old && (old.position.x !== note.position.x || old.position.y !== note.position.y)) {
      stored.x = note.position.x;
      stored.y = note.position.y;
    }
    if (old && !sameSize(old, note)) {
      stored.w = note.size.w;
      stored.h = note.size.h;
    }
    if (old?.color !== note.color) stored.color = note.color;
    if (old?.body !== note.body) stored.body = note.body.slice(0, 10_000);
    if (old?.title !== note.title) {
      if (note.title === undefined) delete stored.title;
      else stored.title = note.title;
    }
  }
}

/**
 * Applies the change from `previous` to `next` to the document and says what could not be applied.
 * Every node ends up with a recorded position, so a node that had none keeps the place it is shown at.
 */
export function applyCanvasEdit(document: FlowDocument, previous: CanvasGraph, next: CanvasGraph): EditReport {
  const report: EditReport = { renamed: [], renameProblems: [], edgeProblems: [], adjustedNames: [], pinIgnored: [], divergent: false };
  const before = new Map(previous.nodes.map((node) => [node.id, node]));
  const after = new Map(next.nodes.map((node) => [node.id, node]));

  const gone = previous.nodes.filter((node) => !after.has(node.id)).map((node) => node.id);
  if (gone.length > 0) removeNodes(document, gone);

  for (const node of next.nodes) {
    const old = before.get(node.id);
    if (!old) createNode(document, node, report);
    else if (old !== node) changeNode(document, old, node, report);
  }

  const edgesBefore = new Set(previous.edges.map((edge) => edge.id));
  const edgesAfter = new Set(next.edges.map((edge) => edge.id));
  for (const edge of previous.edges) if (!edgesAfter.has(edge.id)) disconnect(document, edge.id);
  for (const edge of next.edges) {
    if (edgesBefore.has(edge.id)) continue;
    const problem = edgeProblem(document, edge.source, edge.sourceOutput, edge.target);
    if (problem) {
      report.edgeProblems.push({ id: edge.id, problem });
      report.divergent = true;
      continue;
    }
    const taken = takenIds(document);
    const valid = /^[A-Za-z0-9_-]{1,64}$/.test(edge.id) && !taken.has(edge.id);
    const id = valid ? edge.id : nextId('edge', taken);
    if (!valid) report.divergent = true;
    connect(document, edge.source, edge.sourceOutput, edge.target, id);
  }

  syncNotes(document, previous.notes ?? [], next.notes ?? []);

  // Every node keeps the place it is drawn at, so the next edit does not move the nodes that never had one.
  const editor = editorBlock(document);
  for (const node of next.nodes) editor.nodes[node.id] ??= { x: node.position.x, y: node.position.y };
  return report;
}
