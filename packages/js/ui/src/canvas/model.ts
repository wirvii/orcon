// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { tokens, type ScaleTokenName } from '@orcon/tokens';

import type { StatusKey } from '../components/status-badge/status-map';

const px = (key: ScaleTokenName): number => Number.parseFloat(tokens.scales[key]);

/**
 * Canvas geometry at 100 % zoom, read from the tokens (the library needs numbers to place ports
 * and compare positions). The formulas follow canvas.md#node-tile and canvas.md#logic-nodes.
 */
export const metrics = {
  tile: px('canvas.node-size'),
  wide: px('canvas.node-wide'),
  grid: px('canvas.grid-step'),
  badge: px('canvas.badge'),
  minZoom: px('canvas.zoom.min'),
  maxZoom: px('canvas.zoom.max'),
  lodDetail: px('canvas.lod.detail'),
  lodMinimal: px('canvas.lod.minimal'),
  minimapW: px('canvas.minimap-w'),
  minimapH: px('canvas.minimap-h'),
} as const;

/** Snap grid of the canvas, in px. */
export const GRID = metrics.grid;
/** Default distance between two columns: a tile and a gap of one tile. */
export const COLUMN_PITCH = metrics.tile * 2;
/** Alt + Shift + arrow moves a node this many grid steps. */
export const COARSE_STEPS = 4;
/** Vertical pitch of three or more ports, and the height the extra ports add to a tile. */
const PORT_PITCH = 26;
const PORT_INSET = 24;
const PORT_PADDING = 22;

export type NodeKind = 'action' | 'logic' | 'trigger' | 'agent' | 'placeholder';
export type LogicType = 'if' | 'switch' | 'filter' | 'merge' | 'loop' | 'wait';
export type Direction = 'left' | 'right' | 'up' | 'down';
/** Name of a built-in node icon (Lucide). Connector nodes use a logo instead. */
export type NodeIconName =
  | 'webhook'
  | 'globe'
  | 'mail'
  | 'message'
  | 'database'
  | 'clock'
  | 'file'
  | 'code'
  | 'play'
  | 'bot'
  | 'zap'
  | 'help';

export interface PortSpec {
  id: string;
  /** Visible label of an output ("true", "false", a case name). Optional for single ports. */
  label?: string | undefined;
}

export interface ConnectorRef {
  /** URL of the connector's square logo. */
  src: string;
  name: string;
}

export type AiBlockKind = 'model' | 'memory' | 'tools' | 'parser' | 'retrieval';

export interface AiBlockSpec {
  kind: AiBlockKind;
  /** The block is configured. An unset block is drawn as a dashed circle with a plus. */
  set: boolean;
  /** Name of what is configured (shown in the tooltip and the accessible name). */
  detail?: string | undefined;
}

export type DiffMark = 'added' | 'changed' | 'removed';

export interface GraphNode {
  id: string;
  kind: NodeKind;
  name: string;
  /** The operation the node performs ("Send message", "If total > 0"). One line under the name. */
  subtitle?: string | undefined;
  /** Only for kind "logic". */
  logicType?: LogicType | undefined;
  /** Built-in icon. Ignored when a connector logo is given. */
  icon?: NodeIconName | undefined;
  connector?: ConnectorRef | undefined;
  /** Execution overlay: the state of the node in the execution being shown. */
  status?: StatusKey | undefined;
  /** Execution overlay: a short error code shown in the Failed chip ("401"). */
  statusCode?: string | undefined;
  /** Execution overlay: how many items of the node are UNCERTAIN. */
  uncertainCount?: number | undefined;
  disabled?: boolean | undefined;
  /** The node's output is pinned: later runs reuse it. */
  pinned?: boolean | undefined;
  /** Compare mode: how this node differs from the other version. */
  diff?: DiffMark | undefined;
  /** Configuration blocks of an agent node. */
  ai?: AiBlockSpec[] | undefined;
  inputs: PortSpec[];
  outputs: PortSpec[];
  position: { x: number; y: number };
}

export type EdgeState = 'succeeded' | 'running' | 'failed' | 'uncertain' | 'not-run' | 'waiting';

export interface GraphEdge {
  id: string;
  source: string;
  sourceOutput: string;
  target: string;
  targetInput: string;
  /** Execution overlay: how many items travelled this connection. */
  items?: number | undefined;
  /** Execution overlay: how the connection looks in the execution being shown. Absent at design time. */
  state?: EdgeState | undefined;
  /** The connection goes back to an earlier node (a loop). */
  loopBack?: boolean | undefined;
}

export type StickyColor = 1 | 2 | 3 | 4 | 5 | 6;

export interface StickyNoteData {
  id: string;
  position: { x: number; y: number };
  size: { w: number; h: number };
  color: StickyColor;
  title?: string | undefined;
  body: string;
}

export interface Graph {
  nodes: GraphNode[];
  edges: GraphEdge[];
  notes?: StickyNoteData[] | undefined;
}

/** An entry of the add-node palette. */
export interface CatalogItem {
  id: string;
  kind: NodeKind;
  logicType?: LogicType | undefined;
  name: string;
  subtitle?: string | undefined;
  group: string;
  icon?: NodeIconName | undefined;
  connector?: ConnectorRef | undefined;
  keywords?: readonly string[] | undefined;
  inputs: PortSpec[];
  outputs: PortSpec[];
}

export type ConnectError = 'unknown-node' | 'unknown-port' | 'same-node' | 'duplicate' | 'trigger-input';

export const MIN_NOTE = { w: 160, h: 80 } as const;

export const snap = (value: number, grid = GRID) => Math.round(value / grid) * grid;

/** Rendered size of a tile: 100 wide (200 for an agent), taller when it has more than two outputs. */
export function nodeSize(node: Pick<GraphNode, 'kind' | 'outputs' | 'inputs'>): { w: number; h: number } {
  const ports = Math.max(node.outputs.length, node.inputs.length);
  const h = Math.max(metrics.tile, PORT_PITCH * ports + PORT_PADDING);
  return { w: node.kind === 'agent' ? metrics.wide : metrics.tile, h };
}

/** Vertical position of each of `count` ports on one side of a tile of height `height`. */
export function portOffsets(count: number, height: number): number[] {
  if (count <= 1) return Array.from({ length: count }, () => height / 2);
  if (count === 2) return [PORT_INSET, metrics.tile - PORT_INSET];
  return Array.from({ length: count }, (_, index) => PORT_INSET + PORT_PITCH * index);
}

interface Box {
  id: string;
  x: number;
  y: number;
  w: number;
  h: number;
}

/** Every focusable thing on the canvas: nodes and sticky notes, with the box each one occupies. */
function boxes(graph: Graph): Box[] {
  return [
    ...graph.nodes.map((node) => ({ id: node.id, x: node.position.x, y: node.position.y, ...nodeSize(node) })),
    ...(graph.notes ?? []).map((note) => ({ id: note.id, x: note.position.x, y: note.position.y, w: note.size.w, h: note.size.h })),
  ];
}

export function nodeCenter(node: GraphNode): { x: number; y: number } {
  const size = nodeSize(node);
  return { x: node.position.x + size.w / 2, y: node.position.y + size.h / 2 };
}

/**
 * The node or sticky note to focus when an arrow key is pressed on `fromId`: the nearest one in
 * that direction, preferring those that are straight ahead over ones off to the side.
 */
export function nextFocus(graph: Graph, fromId: string, direction: Direction): string | undefined {
  const all = boxes(graph);
  const from = all.find((box) => box.id === fromId);
  if (!from) return graph.nodes[0]?.id;
  const origin = { x: from.x + from.w / 2, y: from.y + from.h / 2 };
  let best: { id: string; score: number } | undefined;
  for (const box of all) {
    if (box.id === fromId) continue;
    const dx = box.x + box.w / 2 - origin.x;
    const dy = box.y + box.h / 2 - origin.y;
    const along = direction === 'right' ? dx : direction === 'left' ? -dx : direction === 'down' ? dy : -dy;
    const across = direction === 'left' || direction === 'right' ? Math.abs(dy) : Math.abs(dx);
    if (along <= 0 || across > along * 2 + 80) continue;
    const score = along + across * 2;
    if (!best || score < best.score) best = { id: box.id, score };
  }
  return best?.id;
}

/** Nodes in reading order (left to right, then top to bottom), for lists and for the first focus. */
export function readingOrder(graph: Graph): GraphNode[] {
  return [...graph.nodes].sort((a, b) => a.position.x - b.position.x || a.position.y - b.position.y || a.id.localeCompare(b.id));
}

export function moveNodes(graph: Graph, ids: readonly string[], dx: number, dy: number): Graph {
  const moving = new Set(ids);
  return {
    ...graph,
    nodes: graph.nodes.map((node) =>
      moving.has(node.id) ? { ...node, position: { x: snap(node.position.x + dx), y: snap(node.position.y + dy) } } : node,
    ),
    notes: graph.notes?.map((note) =>
      moving.has(note.id) ? { ...note, position: { x: snap(note.position.x + dx), y: snap(note.position.y + dy) } } : note,
    ),
  };
}

export function setPositions(graph: Graph, positions: ReadonlyMap<string, { x: number; y: number }>): Graph {
  return {
    ...graph,
    nodes: graph.nodes.map((node) => {
      const next = positions.get(node.id);
      return next ? { ...node, position: { x: snap(next.x), y: snap(next.y) } } : node;
    }),
    notes: graph.notes?.map((note) => {
      const next = positions.get(note.id);
      return next ? { ...note, position: { x: snap(next.x), y: snap(next.y) } } : note;
    }),
  };
}

export function removeNodes(graph: Graph, ids: readonly string[]): Graph {
  const gone = new Set(ids);
  return {
    nodes: graph.nodes.filter((node) => !gone.has(node.id)),
    edges: graph.edges.filter((edge) => !gone.has(edge.source) && !gone.has(edge.target)),
    notes: graph.notes?.filter((note) => !gone.has(note.id)),
  };
}

export function removeEdge(graph: Graph, id: string): Graph {
  return { ...graph, edges: graph.edges.filter((edge) => edge.id !== id) };
}

export function updateNode(graph: Graph, id: string, patch: Partial<GraphNode>): Graph {
  return { ...graph, nodes: graph.nodes.map((node) => (node.id === id ? { ...node, ...patch } : node)) };
}

export function updateNote(graph: Graph, id: string, patch: Partial<StickyNoteData>): Graph {
  return { ...graph, notes: graph.notes?.map((note) => (note.id === id ? { ...note, ...patch } : note)) };
}

export function addNote(graph: Graph, at: { x: number; y: number }, body: string): { graph: Graph; id: string } {
  const id = uniqueId(new Set([...(graph.notes ?? []).map((note) => note.id), ...graph.nodes.map((node) => node.id)]), 'note');
  const note: StickyNoteData = { id, position: { x: snap(at.x), y: snap(at.y) }, size: { w: 220, h: 120 }, color: 1, body };
  return { graph: { ...graph, notes: [...(graph.notes ?? []), note] }, id };
}

function uniqueId(taken: ReadonlySet<string>, base: string): string {
  let index = 1;
  while (taken.has(`${base}-${index}`)) index += 1;
  return `${base}-${index}`;
}

function toNode(item: CatalogItem, id: string, at: { x: number; y: number }): GraphNode {
  return {
    id,
    kind: item.kind,
    name: item.name,
    subtitle: item.subtitle,
    logicType: item.logicType,
    icon: item.icon,
    connector: item.connector,
    disabled: item.kind === 'placeholder' ? true : undefined,
    inputs: item.kind === 'trigger' ? [] : item.inputs,
    outputs: item.outputs,
    position: { x: snap(at.x), y: snap(at.y) },
  };
}

/** Adds a node from the catalog at a snapped position and returns the new graph and the new node's id. */
export function addNode(graph: Graph, item: CatalogItem, at: { x: number; y: number }): { graph: Graph; id: string } {
  const id = uniqueId(new Set(graph.nodes.map((node) => node.id)), item.id);
  return { graph: { ...graph, nodes: [...graph.nodes, toNode(item, id, at)] }, id };
}

const overlaps = (graph: Graph, at: { x: number; y: number }, size: { w: number; h: number }) =>
  graph.nodes.some((node) => {
    const other = nodeSize(node);
    return (
      at.x < node.position.x + other.w + GRID &&
      at.x + size.w + GRID > node.position.x &&
      at.y < node.position.y + other.h + GRID &&
      at.y + size.h + GRID > node.position.y
    );
  });

/** The first free spot at `at` or below it, in steps of six grid cells. */
export function freeSpot(graph: Graph, at: { x: number; y: number }, size = { w: metrics.tile, h: metrics.tile }) {
  let spot = { x: snap(at.x), y: snap(at.y) };
  while (overlaps(graph, spot, size)) spot = { x: spot.x, y: spot.y + GRID * 6 };
  return spot;
}

/**
 * Adds a node one column to the right of `sourceId`, on the same row, and connects the source
 * output to the new node's first input (the "+" of an output port and the A key).
 */
export function addNodeAfter(
  graph: Graph,
  item: CatalogItem,
  sourceId: string,
  sourceOutput: string,
): { graph: Graph; id: string } {
  const source = graph.nodes.find((node) => node.id === sourceId);
  if (!source) return addNode(graph, item, { x: 0, y: 0 });
  const size = nodeSize({ kind: item.kind, inputs: item.inputs, outputs: item.outputs });
  const spot = freeSpot(graph, { x: source.position.x + COLUMN_PITCH, y: source.position.y }, size);
  const added = addNode(graph, item, spot);
  const input = added.graph.nodes.find((node) => node.id === added.id)?.inputs[0];
  if (!input) return added;
  return { id: added.id, graph: connect(added.graph, { source: sourceId, sourceOutput, target: added.id, targetInput: input.id }).graph };
}

/** Ids of the nodes reachable from `startId` by following connections forward, `startId` included. */
export function downstream(graph: Graph, startId: string): Set<string> {
  const seen = new Set<string>([startId]);
  const queue = [startId];
  while (queue.length > 0) {
    const current = queue.shift() as string;
    for (const edge of graph.edges) {
      if (edge.source === current && !edge.loopBack && !seen.has(edge.target)) {
        seen.add(edge.target);
        queue.push(edge.target);
      }
    }
  }
  return seen;
}

/**
 * Inserts a node on a connection: the connection is split, the new node takes the place of the
 * target, and the target and everything after it move one column to the right.
 */
export function insertOnEdge(graph: Graph, edgeId: string, item: CatalogItem): { graph: Graph; id: string } {
  const edge = graph.edges.find((candidate) => candidate.id === edgeId);
  const source = edge && graph.nodes.find((node) => node.id === edge.source);
  const target = edge && graph.nodes.find((node) => node.id === edge.target);
  if (!edge || !source || !target) return addNode(graph, item, { x: 0, y: 0 });
  const shift = downstream(graph, target.id);
  const moved: Graph = {
    ...graph,
    nodes: graph.nodes.map((node) => (shift.has(node.id) ? { ...node, position: { x: node.position.x + COLUMN_PITCH, y: node.position.y } } : node)),
  };
  const created = addNode(moved, item, { x: target.position.x, y: source.position.y });
  const added = created.graph.nodes.find((node) => node.id === created.id) as GraphNode;
  const withoutEdge = removeEdge(created.graph, edgeId);
  const first = connect(withoutEdge, { source: source.id, sourceOutput: edge.sourceOutput, target: added.id, targetInput: added.inputs[0]?.id ?? 'main' });
  const second = connect(first.graph, { source: added.id, sourceOutput: added.outputs[0]?.id ?? 'main', target: target.id, targetInput: edge.targetInput });
  return { graph: second.graph, id: created.id };
}

export interface ConnectRequest {
  source: string;
  sourceOutput: string;
  target: string;
  targetInput: string;
}

/** Validates a new connection. Returns undefined when it is allowed. */
export function connectError(graph: Graph, request: ConnectRequest): ConnectError | undefined {
  const source = graph.nodes.find((node) => node.id === request.source);
  const target = graph.nodes.find((node) => node.id === request.target);
  if (!source || !target) return 'unknown-node';
  if (source.id === target.id) return 'same-node';
  if (target.kind === 'trigger') return 'trigger-input';
  if (!source.outputs.some((port) => port.id === request.sourceOutput) || !target.inputs.some((port) => port.id === request.targetInput)) {
    return 'unknown-port';
  }
  const duplicate = graph.edges.some(
    (edge) =>
      edge.source === request.source &&
      edge.sourceOutput === request.sourceOutput &&
      edge.target === request.target &&
      edge.targetInput === request.targetInput,
  );
  return duplicate ? 'duplicate' : undefined;
}

export function connect(graph: Graph, request: ConnectRequest): { graph: Graph; error?: ConnectError } {
  const error = connectError(graph, request);
  if (error) return { graph, error };
  const taken = new Set(graph.edges.map((edge) => edge.id));
  const id = uniqueId(taken, `edge-${request.source}-${request.target}`);
  const source = graph.nodes.find((node) => node.id === request.source);
  const target = graph.nodes.find((node) => node.id === request.target);
  const loopBack = Boolean(source && target && target.position.x <= source.position.x);
  return { graph: { ...graph, edges: [...graph.edges, { id, ...request, loopBack }] } };
}

/** The first input of `nodeId` that has no connection yet, falling back to its first input. */
export function freeInput(graph: Graph, nodeId: string): string | undefined {
  const node = graph.nodes.find((candidate) => candidate.id === nodeId);
  if (!node) return undefined;
  const used = new Set(graph.edges.filter((edge) => edge.target === nodeId).map((edge) => edge.targetInput));
  return (node.inputs.find((port) => !used.has(port.id)) ?? node.inputs[0])?.id;
}

/** Outputs of `nodeId` that have no connection: they show the dangling stub. */
export function danglingOutputs(graph: Graph, nodeId: string): string[] {
  const node = graph.nodes.find((candidate) => candidate.id === nodeId);
  if (!node) return [];
  const used = new Set(graph.edges.filter((edge) => edge.source === nodeId).map((edge) => edge.sourceOutput));
  return node.outputs.filter((port) => !used.has(port.id)).map((port) => port.id);
}

/** Items that arrived at a node and items it sent on, from the execution overlay. */
export function itemCounts(graph: Graph, nodeId: string): { received: number; sent: number } {
  let received = 0;
  let sent = 0;
  for (const edge of graph.edges) {
    if (edge.target === nodeId) received += edge.items ?? 0;
    if (edge.source === nodeId) sent += edge.items ?? 0;
  }
  return { received, sent };
}

export interface History<T> {
  past: readonly T[];
  present: T;
  future: readonly T[];
}

export const startHistory = <T>(present: T): History<T> => ({ past: [], present, future: [] });

export function pushHistory<T>(history: History<T>, next: T, limit = 100): History<T> {
  if (Object.is(history.present, next)) return history;
  return { past: [...history.past, history.present].slice(-limit), present: next, future: [] };
}

export function undoHistory<T>(history: History<T>): History<T> {
  const previous = history.past[history.past.length - 1];
  if (previous === undefined) return history;
  return { past: history.past.slice(0, -1), present: previous, future: [history.present, ...history.future] };
}

export function redoHistory<T>(history: History<T>): History<T> {
  const next = history.future[0];
  if (next === undefined) return history;
  return { past: [...history.past, history.present], present: next, future: history.future.slice(1) };
}
