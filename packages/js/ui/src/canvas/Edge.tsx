// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { BaseEdge, EdgeLabelRenderer, getBezierPath, type Edge as FlowEdge, type EdgeProps } from '@xyflow/react';

import { cn } from '../lib/cn';

import { EdgeCount } from './EdgeCount';

export interface OrconEdgeData extends Record<string, unknown> {
  /** Items that travelled this connection. Shows the count chip. */
  items?: number;
  /** The connection goes back to an earlier node and is routed under the flow. */
  loopBack?: boolean;
  /** The connection is on the path that ran. */
  active?: boolean;
}

export type OrconFlowEdge = FlowEdge<OrconEdgeData, 'orcon'>;

/** How far a loop-back connection dips below the nodes it joins, in px. */
const LOOP_DEPTH = 120;
const LOOP_REACH = 80;

/** A cubic curve that leaves to the right, dips under the flow and enters from the left. */
export function loopBackPath(sx: number, sy: number, tx: number, ty: number): { path: string; labelX: number; labelY: number } {
  const dip = Math.max(sy, ty) + LOOP_DEPTH;
  const c1 = { x: sx + LOOP_REACH, y: dip };
  const c2 = { x: tx - LOOP_REACH, y: dip };
  return {
    path: `M ${sx},${sy} C ${c1.x},${c1.y} ${c2.x},${c2.y} ${tx},${ty}`,
    // The midpoint of a cubic curve at t = 0.5.
    labelX: (sx + 3 * c1.x + 3 * c2.x + tx) / 8,
    labelY: (sy + 3 * c1.y + 3 * c2.y + ty) / 8,
  };
}

/**
 * A connection: a 2 px curve. The path that ran uses the active color; a loop back
 * is routed distinctly under the flow. After an execution a chip shows how many
 * items went through. Color never carries the meaning alone: the chip text and the
 * connection's accessible name say it.
 */
export function OrconEdge({ id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, selected, data }: EdgeProps<OrconFlowEdge>) {
  const loopBack = data?.loopBack ?? false;
  const { path, labelX, labelY } = loopBack
    ? loopBackPath(sourceX, sourceY, targetX, targetY)
    : (() => {
        const [curve, x, y] = getBezierPath({ sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition });
        return { path: curve, labelX: x, labelY: y };
      })();
  const stroke = selected ? 'stroke-canvas-selection' : data?.active ? 'stroke-canvas-edge-active' : 'stroke-canvas-edge';
  const showCount = typeof data?.items === 'number';
  return (
    <>
      <BaseEdge id={id} path={path} className={cn('fill-none stroke-2', stroke)} interactionWidth={16} />
      {showCount ? (
        <EdgeLabelRenderer>
          <div
            className="nodrag nopan pointer-events-none absolute"
            // The chip follows the curve, so its position is a runtime value from the canvas layout.
            // eslint-disable-next-line orcon/no-inline-style -- canvas label positioned from computed edge coordinates
            style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)` }}
          >
            <EdgeCount count={data.items ?? 0} loopBack={loopBack} />
          </div>
        </EdgeLabelRenderer>
      ) : null}
    </>
  );
}
