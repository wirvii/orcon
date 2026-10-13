// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { EdgeLabelRenderer, type Edge as FlowEdge, type EdgeProps } from '@xyflow/react';

import { cn } from '../../lib/cn';
import { EdgeActions } from '../edge-actions/EdgeActions';
import { EdgeLabel } from '../edge-label/EdgeLabel';
import type { EdgeState } from '../model';
import { arrowFill, edgeLooks } from '../state';

export interface CanvasEdgeData extends Record<string, unknown> {
  /** Execution state of the connection. Absent at design time. */
  state?: EdgeState;
  /** Items that travelled the connection. Shows the label. */
  items?: number;
  /** The connection comes from a pinned node. */
  pinned?: boolean;
  /** The connection goes back to an earlier node and is routed under the flow. */
  loopBack?: boolean;
  /** The pointer or the keyboard is on the connection: stroke 3 and the action bar. */
  active?: boolean;
  sourceName?: string;
  targetName?: string;
  /** Show the action bar when active. False in read-only mode. */
  editable?: boolean;
  onAdd?: () => void;
  onDelete?: () => void;
  onBarEnter?: () => void;
  onBarLeave?: () => void;
  /** Level of detail: labels and the action bar are hidden at low zoom. */
  detail?: 'full' | 'detail' | 'minimal';
}

export type CanvasFlowEdge = FlowEdge<CanvasEdgeData, 'canvas'>;

/** Distance the line stops short of its ports, so the arrowhead tip touches the target port. */
const START_GAP = 6;
const END_GAP = 8;
/** How far a loop-back connection dips below the nodes it joins, and how far it leaves them. */
const LOOP_DEPTH = 120;
const LOOP_REACH = 80;
const ARROW = 7;

export interface EdgeGeometry {
  d: string;
  /** Where the label and the actions sit: the middle of the curve. */
  mid: { x: number; y: number };
  /** The tip of the arrowhead. */
  tip: { x: number; y: number };
}

/** A cubic bezier with horizontal tangents (control offset max(30, dx / 2)); loop-backs dip under the flow. */
export function edgeGeometry(sx: number, sy: number, tx: number, ty: number, loopBack = false): EdgeGeometry {
  const x1 = sx + START_GAP;
  const x2 = tx - END_GAP;
  if (loopBack) {
    const dip = Math.max(sy, ty) + LOOP_DEPTH;
    const c1 = { x: x1 + LOOP_REACH, y: dip };
    const c2 = { x: x2 - LOOP_REACH, y: dip };
    return {
      d: `M ${x1},${sy} C ${c1.x},${c1.y} ${c2.x},${c2.y} ${x2},${ty}`,
      mid: { x: (x1 + 3 * c1.x + 3 * c2.x + x2) / 8, y: (sy + 3 * c1.y + 3 * c2.y + ty) / 8 },
      tip: { x: x2, y: ty },
    };
  }
  const offset = Math.max(30, (x2 - x1) / 2);
  return {
    d: `M ${x1},${sy} C ${x1 + offset},${sy} ${x2 - offset},${ty} ${x2},${ty}`,
    mid: { x: (x1 + x2) / 2, y: (sy + ty) / 2 },
    tip: { x: x2, y: ty },
  };
}

/**
 * A connection between two ports: a 2 px bezier with a filled arrowhead. After a run the line shows
 * the state by its dash pattern and a label with the item count; hovering or focusing it thickens the
 * line and opens the actions. Only a running connection animates, and not under reduced motion.
 */
export function CanvasEdge({ id, sourceX, sourceY, targetX, targetY, data }: EdgeProps<CanvasFlowEdge>) {
  const state = data?.state;
  const key = state ?? 'design';
  const look = edgeLooks[key];
  const loopBack = data?.loopBack ?? false;
  const { d, mid, tip } = edgeGeometry(sourceX, sourceY, targetX, targetY, loopBack);
  const active = data?.active ?? false;
  const showLabel = (state !== undefined || loopBack) && data?.detail !== 'detail' && data?.detail !== 'minimal';
  const showBar = active && data?.editable && data?.detail === 'full';
  const arrow = `M ${tip.x - ARROW},${tip.y - ARROW / 2 - 0.5} L ${tip.x},${tip.y} L ${tip.x - ARROW},${tip.y + ARROW / 2 + 0.5} Z`;
  return (
    <>
      {/* The focus ring of a focused connection; the browser outline would box the whole curve. */}
      <path d={d} fill="none" strokeWidth={8} strokeLinecap="round" className="orcon-edge-focus stroke-focus pointer-events-none opacity-0" />
      <path d={d} fill="none" strokeWidth={active ? 3 : 2} strokeLinecap="round" strokeDasharray={look.dash} data-edge-state={key} data-edge-id={id} className={cn(look.stroke, look.flows && 'motion-safe:animate-edge-flow')} />
      <path d={arrow} data-edge-arrow className={cn(arrowFill[key], 'stroke-none')} />
      {/* A wide invisible stroke makes the thin line easy to hover and click. */}
      <path d={d} fill="none" strokeWidth={20} className="stroke-transparent" />
      <EdgeLabelRenderer>
        <div
          className="nodrag nopan pointer-events-none absolute"
          // The label follows the curve, so its position is a runtime value from the canvas layout.
          // eslint-disable-next-line orcon/no-inline-style -- label positioned from computed edge coordinates
          style={{ transform: `translate(-50%, -50%) translate(${mid.x}px, ${mid.y}px)` }}
        >
          <div className={cn('flex flex-col items-center gap-1', showBar && '-translate-y-4')}>
            {showLabel ? <EdgeLabel items={data?.items} state={state} pinned={data?.pinned} loopBack={loopBack} /> : null}
            {showBar ? (
              <div className="pointer-events-auto">
                <EdgeActions
                  source={data?.sourceName ?? ''}
                  target={data?.targetName ?? ''}
                  onAdd={data?.onAdd}
                  onDelete={data?.onDelete}
                  onPointerEnter={data?.onBarEnter}
                  onPointerLeave={data?.onBarLeave}
                />
              </div>
            ) : null}
          </div>
        </div>
      </EdgeLabelRenderer>
    </>
  );
}
