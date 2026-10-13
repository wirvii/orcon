// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { statusMap, type StatusFamily, type StatusKey } from '../components/status-badge/status-map';

import type { DiffMark, EdgeState } from './model';

/** How a node looks on the canvas for an execution state (canvas.md#execution-states). */
export type NodeVisual = 'design' | 'running' | 'succeeded' | 'warning' | 'failed' | 'uncertain' | 'waiting' | 'canceled' | 'not-run';

export function nodeVisual(status: StatusKey | undefined): NodeVisual {
  switch (status) {
    case undefined:
      return 'design';
    case 'RUNNING':
      return 'running';
    case 'SUCCEEDED':
      return 'succeeded';
    case 'SUCCEEDED_WITH_ERRORS':
      return 'warning';
    case 'FAILED':
    case 'ERROR_FINAL':
      return 'failed';
    case 'UNCERTAIN':
      return 'uncertain';
    case 'CANCELED':
      return 'canceled';
    case 'NOT_RUN':
    case 'NOT_DISPATCHED':
      return 'not-run';
    default:
      return 'waiting';
  }
}

/** Static class names per color family, so Tailwind finds every utility it has to generate. */
export interface FamilyClasses {
  /** Text and icons in the family color. */
  text: string;
  /** The family background (chips, alerts). */
  bg: string;
  /** A 1 px border in the family color. */
  border: string;
  /** A 2 px border in the family color. */
  border2: string;
  /** A solid fill in the family color (badges). */
  solid: string;
  /** Text on the solid family color (the glyph of a badge). */
  onSolid: string;
  /** Run ring and outlines. */
  stroke: string;
}

export const familyClasses: Record<StatusFamily, FamilyClasses> = {
  neutral: {
    text: 'text-status-neutral',
    bg: 'bg-status-neutral-bg',
    border: 'border-status-neutral',
    border2: 'border-2 border-status-neutral',
    solid: 'bg-status-neutral',
    onSolid: 'text-status-neutral-bg',
    stroke: 'stroke-status-neutral',
  },
  active: {
    text: 'text-status-active',
    bg: 'bg-status-active-bg',
    border: 'border-status-active',
    border2: 'border-2 border-status-active',
    solid: 'bg-status-active',
    onSolid: 'text-status-active-bg',
    stroke: 'stroke-status-active',
  },
  success: {
    text: 'text-status-success',
    bg: 'bg-status-success-bg',
    border: 'border-status-success',
    border2: 'border-2 border-status-success',
    solid: 'bg-status-success',
    onSolid: 'text-status-success-bg',
    stroke: 'stroke-status-success',
  },
  warning: {
    text: 'text-status-warning',
    bg: 'bg-status-warning-bg',
    border: 'border-status-warning',
    border2: 'border-2 border-status-warning',
    solid: 'bg-status-warning',
    onSolid: 'text-status-warning-bg',
    stroke: 'stroke-status-warning',
  },
  danger: {
    text: 'text-status-danger',
    bg: 'bg-status-danger-bg',
    border: 'border-status-danger',
    border2: 'border-2 border-status-danger',
    solid: 'bg-status-danger',
    onSolid: 'text-status-danger-bg',
    stroke: 'stroke-status-danger',
  },
  attention: {
    text: 'text-status-attention',
    bg: 'bg-status-attention-bg',
    border: 'border-status-attention',
    border2: 'border-2 border-status-attention',
    solid: 'bg-status-attention',
    onSolid: 'text-status-attention-bg',
    stroke: 'stroke-status-attention',
  },
};

/** The color family of an execution state. */
export const familyOf = (status: StatusKey): StatusFamily => statusMap[status].family;

/** The family that marks a compare-mode difference. */
export const diffFamily: Record<DiffMark, StatusFamily> = { added: 'success', changed: 'warning', removed: 'danger' };

export interface EdgeLook {
  /** Stroke class (the color). */
  stroke: string;
  /** SVG stroke-dasharray, absent for a solid line. */
  dash?: string | undefined;
  /** The dash pattern flows (running only; off under reduced motion). */
  flows?: boolean | undefined;
  /** The family of the label chip. */
  family?: StatusFamily | undefined;
}

/** canvas.md#edges: color is never the only signal; the dash pattern differs per state. */
export const edgeLooks: Record<EdgeState | 'design', EdgeLook> = {
  design: { stroke: 'stroke-canvas-edge' },
  succeeded: { stroke: 'stroke-canvas-edge-active', family: 'success' },
  running: { stroke: 'stroke-status-active', dash: '7 6', flows: true, family: 'active' },
  waiting: { stroke: 'stroke-status-active', dash: '7 6', family: 'active' },
  failed: { stroke: 'stroke-status-danger', dash: '2 5', family: 'danger' },
  uncertain: { stroke: 'stroke-status-attention', dash: '8 5', family: 'attention' },
  'not-run': { stroke: 'stroke-canvas-edge', dash: '3 5' },
};

/** Fill class of the arrowhead for each edge state. */
export const arrowFill: Record<EdgeState | 'design', string> = {
  design: 'fill-canvas-edge',
  succeeded: 'fill-canvas-edge-active',
  running: 'fill-status-active',
  waiting: 'fill-status-active',
  failed: 'fill-status-danger',
  uncertain: 'fill-status-attention',
  'not-run': 'fill-canvas-edge',
};
