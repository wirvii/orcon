// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Graph, layout } from '@dagrejs/dagre';

import { COLUMN_PITCH, snap } from './canvas-graph';
import type { Point } from './flow-document';

/**
 * Tidy up (SPEC-014 R-ED-CAN-03): a layered, left-to-right layout of the whole flow or of the
 * selection. It is a pure function over plain data so it can run in a web worker (`layout.worker.ts`)
 * and in tests. The result puts every laid out node on the 20 px grid and keeps each sticky note
 * at the same distance from the node it was nearest to.
 */

export interface LayoutNode {
  id: string;
  w: number;
  h: number;
  position: Point;
}

export interface LayoutNote {
  id: string;
  w: number;
  h: number;
  position: Point;
}

export interface LayoutInput {
  nodes: LayoutNode[];
  /** Forward connections only; a loop-back connection must not be sent. `port` is the position of the output the connection leaves from. */
  edges: { from: string; to: string; port?: number }[];
  notes: LayoutNote[];
}

/** The new position of every node and note that moved, as pairs so they cross a worker boundary as they are. */
export type LayoutOutput = [string, Point][];

/** Room under a tile for its name and subtitle (canvas.md#labels). */
const LABEL_ROOM = 56;
const ROW_GAP = 40;
const GRID_GAP = 20;
const RANK_GAP = COLUMN_PITCH - 100;

const centerOf = (box: { position: Point; w: number; h: number }): Point => ({ x: box.position.x + box.w / 2, y: box.position.y + box.h / 2 });

/**
 * Keeps the targets of one node in the order of its outputs, top to bottom: the target of "true" is
 * placed above the target of "false", so the connections of an If node do not cross.
 */
function outputOrder(input: LayoutInput, ids: ReadonlySet<string>): { left: string; right: string }[] {
  const bySource = new Map<string, { to: string; port: number }[]>();
  for (const edge of input.edges) {
    if (!ids.has(edge.from) || !ids.has(edge.to) || edge.from === edge.to) continue;
    const list = bySource.get(edge.from) ?? [];
    list.push({ to: edge.to, port: edge.port ?? 0 });
    bySource.set(edge.from, list);
  }
  const constraints: { left: string; right: string }[] = [];
  for (const targets of bySource.values()) {
    const ordered = targets.sort((a, b) => a.port - b.port).filter((target, index, all) => all.findIndex((other) => other.to === target.to) === index);
    for (let index = 1; index < ordered.length; index += 1) {
      const before = ordered[index - 1];
      const after = ordered[index];
      if (before && after) constraints.push({ left: before.to, right: after.to });
    }
  }
  return constraints;
}

export function layoutGraph(input: LayoutInput): LayoutOutput {
  if (input.nodes.length === 0) return [];
  const ids = new Set(input.nodes.map((node) => node.id));
  const graph = new Graph();
  graph.setGraph({ rankdir: 'LR', nodesep: ROW_GAP, ranksep: RANK_GAP, marginx: 0, marginy: 0 });
  graph.setDefaultEdgeLabel(() => ({}));
  for (const node of input.nodes) graph.setNode(node.id, { width: node.w, height: node.h + LABEL_ROOM });
  for (const edge of input.edges) if (ids.has(edge.from) && ids.has(edge.to) && edge.from !== edge.to) graph.setEdge(edge.from, edge.to);
  layout(graph, { constraints: outputOrder(input, ids) });

  // The layout keeps its place: the top left corner of the laid out nodes stays where it was.
  const laid = input.nodes.map((node) => {
    const placed = graph.node(node.id) as Point;
    return { node, x: placed.x - node.w / 2, y: placed.y - (node.h + LABEL_ROOM) / 2 };
  });
  const laidLeft = Math.min(...laid.map((entry) => entry.x));
  const laidTop = Math.min(...laid.map((entry) => entry.y));
  const originLeft = snap(Math.min(...input.nodes.map((node) => node.position.x)));
  const originTop = snap(Math.min(...input.nodes.map((node) => node.position.y)));

  const moved = new Map<string, Point>();
  for (const { node, x, y } of laid) moved.set(node.id, { x: snap(originLeft + x - laidLeft), y: snap(originTop + y - laidTop) });

  // A note keeps its distance to the laid out node it was nearest to; a note far from every one of them stays.
  for (const note of input.notes) {
    const center = centerOf(note);
    let nearest: { node: LayoutNode; distance: number } | undefined;
    for (const node of input.nodes) {
      const at = centerOf({ position: node.position, w: node.w, h: node.h });
      const distance = Math.hypot(at.x - center.x, at.y - center.y);
      if (!nearest || distance < nearest.distance) nearest = { node, distance };
    }
    const target = nearest ? moved.get(nearest.node.id) : undefined;
    if (!nearest || !target) continue;
    moved.set(note.id, {
      x: snap(target.x + (note.position.x - nearest.node.position.x)),
      y: snap(target.y + (note.position.y - nearest.node.position.y)),
    });
  }
  separateNotes(input, moved);
  return [...moved];
}

interface Box {
  x: number;
  y: number;
  w: number;
  h: number;
}

const overlap = (a: Box, b: Box) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;

/**
 * A sticky note is an obstacle, not just a thing that follows a node: a note that ended up under a
 * node (a node with no connections lands in the first column, where a note often is) moves down to
 * the first free place below what it overlaps, on the grid. Notes are placed in the order they have.
 */
function separateNotes(input: LayoutInput, moved: Map<string, Point>): void {
  const boxes: Box[] = input.nodes.map((node) => {
    const at = moved.get(node.id) ?? node.position;
    return { x: at.x, y: at.y, w: node.w, h: node.h + LABEL_ROOM };
  });
  for (const note of input.notes) {
    const at = moved.get(note.id) ?? note.position;
    let box: Box = { x: at.x, y: at.y, w: note.w, h: note.h };
    // Every pass moves the note below one more obstacle, so the loop ends after at most one pass per box.
    for (let pass = 0; pass <= boxes.length; pass += 1) {
      const blocker = boxes.find((other) => overlap(box, other));
      if (!blocker) break;
      box = { ...box, y: snap(blocker.y + blocker.h + GRID_GAP) };
    }
    moved.set(note.id, { x: box.x, y: box.y });
    boxes.push(box);
  }
}
