// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { StatusKey } from '../components/status-badge/status-map';

/** Snap grid of the canvas, in px. */
export const GRID = 8;
/** Alt + Shift + arrow moves a node this many grid steps. */
export const COARSE_STEPS = 4;

export type NodeKind = 'action' | 'trigger' | 'control';
export type ControlType = 'if' | 'switch' | 'merge' | 'loop' | 'wait';
export type Direction = 'left' | 'right' | 'up' | 'down';

export interface PortSpec {
  id: string;
  /** Visible label of an output ("true", "false", a case name). Optional for single ports. */
  label?: string;
}

export interface ConnectorRef {
  /** URL of the connector's square logo. */
  src: string;
  name: string;
}

export interface GraphNode {
  id: string;
  kind: NodeKind;
  name: string;
  /** One line that says what the node does. */
  summary?: string;
  connector?: ConnectorRef;
  /** Only for kind "control". */
  controlType?: ControlType;
  /** Execution overlay: the state of the node in the execution being shown. */
  status?: StatusKey;
  /** Execution overlay: how many items of the node are UNCERTAIN. */
  uncertainCount?: number;
  inputs: PortSpec[];
  outputs: PortSpec[];
  position: { x: number; y: number };
}

export interface GraphEdge {
  id: string;
  source: string;
  sourceOutput: string;
  target: string;
  targetInput: string;
  /** Execution overlay: how many items travelled this connection. */
  items?: number;
  /** The connection goes back to an earlier node (a loop). */
  loopBack?: boolean;
  /** The connection is on the path that ran. */
  active?: boolean;
}

export interface Graph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

/** An entry of the add-node palette. */
export interface CatalogItem {
  id: string;
  kind: NodeKind;
  controlType?: ControlType;
  name: string;
  summary?: string;
  group: string;
  connector?: ConnectorRef;
  keywords?: readonly string[];
  inputs: PortSpec[];
  outputs: PortSpec[];
}

export type ConnectError = 'unknown-node' | 'unknown-port' | 'same-node' | 'duplicate' | 'trigger-input';

/** Approximate node boxes, used only to compare positions when moving focus with the arrow keys. */
const ACTION_BOX = { w: 240, h: 96 };
const CONTROL_BOX = { w: 120, h: 40 };

export const snap = (value: number, grid = GRID) => Math.round(value / grid) * grid;

export function nodeCenter(node: GraphNode): { x: number; y: number } {
  const box = node.kind === 'control' ? CONTROL_BOX : ACTION_BOX;
  return { x: node.position.x + box.w / 2, y: node.position.y + box.h / 2 };
}

/**
 * The node to focus when an arrow key is pressed on `fromId`: the nearest node in
 * that direction, preferring nodes that are straight ahead over ones off to the side.
 */
export function nextFocus(graph: Graph, fromId: string, direction: Direction): string | undefined {
  const from = graph.nodes.find((node) => node.id === fromId);
  if (!from) return graph.nodes[0]?.id;
  const origin = nodeCenter(from);
  let best: { id: string; score: number } | undefined;
  for (const node of graph.nodes) {
    if (node.id === fromId) continue;
    const center = nodeCenter(node);
    const dx = center.x - origin.x;
    const dy = center.y - origin.y;
    const along = direction === 'right' ? dx : direction === 'left' ? -dx : direction === 'down' ? dy : -dy;
    const across = direction === 'left' || direction === 'right' ? Math.abs(dy) : Math.abs(dx);
    if (along <= 0 || across > along * 2 + 80) continue;
    const score = along + across * 2;
    if (!best || score < best.score) best = { id: node.id, score };
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
  };
}

export function setPositions(graph: Graph, positions: ReadonlyMap<string, { x: number; y: number }>): Graph {
  return {
    ...graph,
    nodes: graph.nodes.map((node) => {
      const next = positions.get(node.id);
      return next ? { ...node, position: { x: snap(next.x), y: snap(next.y) } } : node;
    }),
  };
}

export function removeNodes(graph: Graph, ids: readonly string[]): Graph {
  const gone = new Set(ids);
  return {
    nodes: graph.nodes.filter((node) => !gone.has(node.id)),
    edges: graph.edges.filter((edge) => !gone.has(edge.source) && !gone.has(edge.target)),
  };
}

export function removeEdge(graph: Graph, id: string): Graph {
  return { ...graph, edges: graph.edges.filter((edge) => edge.id !== id) };
}

function uniqueId(taken: ReadonlySet<string>, base: string): string {
  let index = 1;
  while (taken.has(`${base}-${index}`)) index += 1;
  return `${base}-${index}`;
}

/** Adds a node from the catalog at a snapped position and returns the new graph and the new node's id. */
export function addNode(graph: Graph, item: CatalogItem, at: { x: number; y: number }): { graph: Graph; id: string } {
  const id = uniqueId(new Set(graph.nodes.map((node) => node.id)), item.id);
  const node: GraphNode = {
    id,
    kind: item.kind,
    name: item.name,
    summary: item.summary,
    connector: item.connector,
    controlType: item.controlType,
    inputs: item.kind === 'trigger' ? [] : item.inputs,
    outputs: item.outputs,
    position: { x: snap(at.x), y: snap(at.y) },
  };
  return { graph: { ...graph, nodes: [...graph.nodes, node] }, id };
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
