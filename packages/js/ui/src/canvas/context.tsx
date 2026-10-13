// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useContext } from 'react';

import type { AiBlockKind, Graph } from './model';

/** How much of a node is drawn at the current zoom (canvas.md#level-of-detail). */
export type DetailLevel = 'full' | 'detail' | 'minimal';

export interface CanvasNodeActions {
  addAfter: (nodeId: string, outputId: string) => void;
  runStep: (nodeId: string) => void;
  executeWorkflow: (nodeId: string) => void;
  toggleDisabled: (nodeId: string) => void;
  remove: (nodeId: string) => void;
  openDetails: (nodeId: string, tab?: 'agent') => void;
  startRename: (nodeId: string) => void;
  commitRename: (nodeId: string, name: string) => void;
  cancelRename: (nodeId: string) => void;
  duplicate: (nodeId: string) => void;
  togglePin: (nodeId: string) => void;
  copy: (nodeId: string) => void;
  openAiBlock: (nodeId: string, kind: AiBlockKind) => void;
}

export interface CanvasContextValue {
  graph: Graph;
  editable: boolean;
  /** The node that holds the roving tab stop: only its toolbar and ports are reachable with Tab. */
  activeId: string | undefined;
  detail: DetailLevel;
  /** Counter-scale of names and chips at low zoom, so their on-screen size stays readable. */
  labelScale: number;
  renamingId: string | undefined;
  /** The sticky note being edited. */
  editingNoteId: string | undefined;
  setEditingNote: (id: string | undefined) => void;
  /** The node or sticky note that has keyboard focus. Its toolbar shows like on hover. */
  focusedId: string | undefined;
  /** Outputs of each node that have no connection: they show the dashed stub. */
  dangling: ReadonlyMap<string, readonly string[]>;
  /** The output a connection is being dragged from, while one is. Inputs then show whether they can receive it. */
  connectingFrom: { nodeId: string; outputId: string } | undefined;
  actions: CanvasNodeActions;
}

const noop = () => undefined;

const noActions: CanvasNodeActions = {
  addAfter: noop,
  runStep: noop,
  executeWorkflow: noop,
  toggleDisabled: noop,
  remove: noop,
  openDetails: noop,
  startRename: noop,
  commitRename: noop,
  cancelRename: noop,
  duplicate: noop,
  togglePin: noop,
  copy: noop,
  openAiBlock: noop,
};

export const CanvasContext = createContext<CanvasContextValue>({
  graph: { nodes: [], edges: [] },
  editable: false,
  activeId: undefined,
  detail: 'full',
  labelScale: 1,
  renamingId: undefined,
  editingNoteId: undefined,
  setEditingNote: noop,
  focusedId: undefined,
  dangling: new Map(),
  connectingFrom: undefined,
  actions: noActions,
});

export const useCanvasContext = () => useContext(CanvasContext);
