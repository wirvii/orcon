// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FlowDocument, FlowNode, Point } from './flow-document';
import { LOOP_TYPE, START_TYPE, UNSUPPORTED_TYPE, subtitleOf, typeDefOf, type CanvasIconName, type CanvasKind, type CanvasLogic } from './node-types';

/**
 * The shapes the design system canvas (`WorkflowCanvas` in `@orcon/ui`) works with, as plain data.
 * `model/` may not import the UI package, so they are declared here; the screen layer checks that
 * they stay assignable in both directions to the package's `Graph`.
 */

/** One column of the layout: a tile (100) and a gap of one tile (SPEC-007 canvas.md#layout). */
export const COLUMN_PITCH = 200;
export const ROW_PITCH = 160;
export const GRID = 20;

export const snap = (value: number, grid = GRID): number => Math.round(value / grid) * grid;

export interface CanvasPort {
  id: string;
  label?: string | undefined;
}

export interface CanvasNode {
  id: string;
  kind: CanvasKind;
  name: string;
  subtitle?: string | undefined;
  logicType?: CanvasLogic | undefined;
  icon?: CanvasIconName | undefined;
  disabled?: boolean | undefined;
  pinned?: boolean | undefined;
  inputs: CanvasPort[];
  outputs: CanvasPort[];
  position: Point;
}

export interface CanvasEdge {
  id: string;
  source: string;
  sourceOutput: string;
  target: string;
  targetInput: string;
  loopBack?: boolean | undefined;
}

export interface CanvasNote {
  id: string;
  position: Point;
  size: { w: number; h: number };
  color: 1 | 2 | 3 | 4 | 5 | 6;
  title?: string | undefined;
  body: string;
}

export interface CanvasGraph {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  notes?: CanvasNote[] | undefined;
}

/** The single input port every node has: the flow format names no input port (`to` has only the node). */
export const INPUT_PORT = 'main';

const asColor = (value: number): CanvasNote['color'] => (value >= 1 && value <= 6 ? (Math.round(value) as CanvasNote['color']) : 1);

/**
 * Where nodes without a recorded position go: layer by layer from the start nodes, one column per
 * layer. Only used for documents written without editor positions (imports, other clients); the
 * first edit records every position (see `applyCanvasEdit`).
 */
export function fallbackPositions(document: FlowDocument): Map<string, Point> {
  const layerOf = new Map<string, number>();
  const outgoing = new Map<string, string[]>();
  for (const edge of document.edges) {
    if (edge.kind === 'BACK') continue;
    const list = outgoing.get(edge.from.node) ?? [];
    list.push(edge.to.node);
    outgoing.set(edge.from.node, list);
  }
  const hasIncoming = new Set(document.edges.filter((edge) => edge.kind !== 'BACK').map((edge) => edge.to.node));
  const roots = document.nodes.filter((node) => !hasIncoming.has(node.id));
  // Longest path from a root, bounded by the node count so a cycle in a bad document cannot loop.
  let frontier = roots.map((node) => node.id);
  roots.forEach((node) => layerOf.set(node.id, 0));
  for (let step = 0; step < document.nodes.length && frontier.length > 0; step += 1) {
    const next: string[] = [];
    for (const id of frontier) {
      for (const target of outgoing.get(id) ?? []) {
        const layer = (layerOf.get(id) ?? 0) + 1;
        if (layer > (layerOf.get(target) ?? -1)) {
          layerOf.set(target, layer);
          next.push(target);
        }
      }
    }
    frontier = next;
  }
  const rows = new Map<number, number>();
  const positions = new Map<string, Point>();
  for (const node of document.nodes) {
    const layer = layerOf.get(node.id) ?? 0;
    const row = rows.get(layer) ?? 0;
    rows.set(layer, row + 1);
    positions.set(node.id, { x: layer * COLUMN_PITCH, y: row * ROW_PITCH });
  }
  return positions;
}

function presentationOf(node: FlowNode, extraOutputs: readonly string[]) {
  if (node.type === UNSUPPORTED_TYPE) {
    return { kind: 'placeholder' as const, logic: undefined, icon: 'help' as const, outputs: [{ id: 'main' }] };
  }
  const def = typeDefOf(node);
  const kind: CanvasKind = def?.kind ?? (node.element === 'startEvent' ? 'trigger' : 'action');
  const base = def ? [...def.outputs] : node.element === 'endEvent' ? [] : [{ id: 'main' }];
  const outputs: { id: string; label?: string }[] = base.map((port) => ('label' in port ? { id: port.id, label: port.label } : { id: port.id }));
  const wanted = [...extraOutputs, ...(node['onError'] === 'ERROR_OUTPUT' ? ['error'] : [])];
  for (const id of wanted) if (!outputs.some((port) => port.id === id)) outputs.push({ id });
  return { kind, logic: def?.logic, icon: def?.icon ?? (kind === 'placeholder' ? ('help' as const) : ('globe' as const)), outputs };
}

/**
 * The canvas view of a document. `describe` supplies the line under a node when its parameters
 * give none (the localized one-line description of its type).
 */
export function toCanvasGraph(document: FlowDocument, describe?: (node: FlowNode) => string | undefined): CanvasGraph {
  const recorded = document.editor.nodes ?? {};
  const fallback = document.nodes.some((node) => recorded[node.id] === undefined) ? fallbackPositions(document) : undefined;
  const portsUsed = new Map<string, string[]>();
  for (const edge of document.edges) {
    const list = portsUsed.get(edge.from.node) ?? [];
    if (!list.includes(edge.from.port)) list.push(edge.from.port);
    portsUsed.set(edge.from.node, list);
  }
  const nodes = document.nodes.map((node): CanvasNode => {
    const view = presentationOf(node, portsUsed.get(node.id) ?? []);
    const position = recorded[node.id] ?? fallback?.get(node.id) ?? { x: 0, y: 0 };
    return {
      id: node.id,
      kind: view.kind,
      name: node.name,
      subtitle: subtitleOf(node) ?? describe?.(node),
      logicType: view.logic,
      icon: view.icon,
      disabled: node.disabled === true || node.type === UNSUPPORTED_TYPE ? true : undefined,
      inputs: view.kind === 'trigger' ? [] : [{ id: INPUT_PORT }],
      outputs: view.outputs,
      position: { x: position.x, y: position.y },
    };
  });
  const edges = document.edges.map(
    (edge): CanvasEdge => ({
      id: edge.id,
      source: edge.from.node,
      sourceOutput: edge.from.port,
      target: edge.to.node,
      targetInput: INPUT_PORT,
      loopBack: edge.kind === 'BACK' ? true : undefined,
    }),
  );
  const notes = (document.editor.stickyNotes ?? []).map(
    (note): CanvasNote => ({
      id: note.id,
      position: { x: note.x, y: note.y },
      size: { w: note.w, h: note.h },
      color: asColor(note.color),
      title: note.title,
      body: note.body,
    }),
  );
  return { nodes, edges, notes };
}

/** True when the node is the start of the flow (nothing can be connected into it). */
export const isStartNode = (node: Pick<FlowNode, 'type'>): boolean => node.type === START_TYPE;
export const isLoopNode = (node: Pick<FlowNode, 'type'>): boolean => node.type === LOOP_TYPE;

/**
 * True when the canvas shows what the document holds: the same nodes with the same names, enabled
 * state and positions, the same connections and the same sticky notes. The screen checks it when
 * the canvas went back or forward by itself (undo, redo) and rebuilds the canvas when it fails.
 */
export function graphAgrees(document: FlowDocument, graph: CanvasGraph): boolean {
  const notes = document.editor.stickyNotes ?? [];
  const shownNotes = graph.notes ?? [];
  if (document.nodes.length !== graph.nodes.length || document.edges.length !== graph.edges.length || notes.length !== shownNotes.length) return false;
  const recorded = document.editor.nodes ?? {};
  const shown = new Map(graph.nodes.map((node) => [node.id, node]));
  for (const node of document.nodes) {
    const seen = shown.get(node.id);
    if (seen?.name !== node.name || seen.pinned === true) return false;
    if ((seen.disabled === true) !== (node.disabled === true || node.type === UNSUPPORTED_TYPE)) return false;
    const at = recorded[node.id];
    if (at && (at.x !== seen.position.x || at.y !== seen.position.y)) return false;
  }
  const edges = new Map(graph.edges.map((edge) => [edge.id, edge]));
  for (const edge of document.edges) {
    const seen = edges.get(edge.id);
    if (seen?.source !== edge.from.node || seen.sourceOutput !== edge.from.port || seen.target !== edge.to.node) return false;
  }
  const shownById = new Map(shownNotes.map((note) => [note.id, note]));
  return notes.every((note) => {
    const seen = shownById.get(note.id);
    return (
      seen?.position.x === note.x &&
      seen.position.y === note.y &&
      seen.size.w === note.w &&
      seen.size.h === note.h &&
      seen.color === note.color &&
      seen.body === note.body &&
      seen.title === note.title
    );
  });
}

export interface Viewport {
  x: number;
  y: number;
  zoom: number;
}

const TRANSFORM = /translate\(\s*(-?[\d.e+-]+)px\s*,\s*(-?[\d.e+-]+)px\s*\)\s*scale\(\s*([\d.e+-]+)\s*\)/;

/** Reads the viewport out of the CSS transform the canvas puts on its content (`translate(12px, 8px) scale(0.8)`). */
export function parseViewportTransform(transform: string): Viewport | undefined {
  const match = TRANSFORM.exec(transform);
  if (!match) return undefined;
  const [x, y, zoom] = [Number(match[1]), Number(match[2]), Number(match[3])];
  return [x, y, zoom].every(Number.isFinite) && zoom > 0 ? { x, y, zoom } : undefined;
}

/** The canvas coordinate under a point of the page, given the canvas box and its viewport. */
export function clientToFlow(client: Point, box: { left: number; top: number }, viewport: Viewport): Point {
  return { x: (client.x - box.left - viewport.x) / viewport.zoom, y: (client.y - box.top - viewport.y) / viewport.zoom };
}

/** The size of a tile at 100 %: 100 wide (200 for an agent), taller when it has more than three ports (canvas.md#node-tile). */
export function nodeBox(node: Pick<CanvasNode, 'kind' | 'inputs' | 'outputs'>): { w: number; h: number } {
  const ports = Math.max(node.outputs.length, node.inputs.length);
  return { w: node.kind === 'agent' ? 200 : 100, h: Math.max(100, 26 * ports + 22) };
}
