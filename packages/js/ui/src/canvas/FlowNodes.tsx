// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Node, NodeProps } from '@xyflow/react';

import { CanvasNode } from './canvas-node/CanvasNode';
import { connectError, type GraphNode, type StickyNoteData } from './model';
import { StickyNote } from './sticky-note/StickyNote';
import { useCanvasContext } from './context';
import { useIntl } from 'react-intl';
import { canvasMessages } from './messages';

export interface TileNodeData extends Record<string, unknown> {
  node: GraphNode;
}
export interface NoteNodeData extends Record<string, unknown> {
  note: StickyNoteData;
  onChange: (id: string, patch: Partial<Pick<StickyNoteData, 'body' | 'color' | 'title'>>) => void;
  onResize: (id: string, size: { w: number; h: number }) => void;
}

export type TileFlowNode = Node<TileNodeData, 'tile'>;
export type NoteFlowNode = Node<NoteNodeData, 'sticky'>;
export type CanvasFlowNodeUnion = TileFlowNode | NoteFlowNode;

/** React Flow renders these inside the focusable node element, which carries the accessible name. */
export function TileNode({ data, selected }: NodeProps<TileFlowNode>) {
  const intl = useIntl();
  const context = useCanvasContext();
  const { node } = data;
  const from = context.connectingFrom;
  // While a connection is dragged, an input that cannot receive it says why.
  const invalidInputs: Record<string, string> = {};
  if (from) {
    for (const input of node.inputs) {
      if (connectError(context.graph, { source: from.nodeId, sourceOutput: from.outputId, target: node.id, targetInput: input.id }) !== undefined) {
        invalidInputs[input.id] = intl.formatMessage(canvasMessages.portConnectInvalid);
      }
    }
  }
  return (
    <CanvasNode
      node={node}
      selected={selected}
      live
      editable={context.editable}
      tabbable={context.activeId === node.id}
      focused={context.focusedId === node.id}
      detail={context.detail}
      labelScale={context.labelScale}
      dangling={context.dangling.get(node.id)}
      invalidInputs={invalidInputs}
      renaming={context.renamingId === node.id}
      actions={context.actions}
    />
  );
}

export function NoteNode({ data, selected }: NodeProps<NoteFlowNode>) {
  const context = useCanvasContext();
  const { note } = data;
  return (
    <StickyNote
      note={note}
      selected={selected}
      editable={context.editable}
      live
      editing={context.editingNoteId === note.id}
      onEditingChange={(editing) => context.setEditingNote(editing ? note.id : undefined)}
      tabbable={context.activeId === note.id}
      onChange={(patch) => data.onChange(note.id, patch)}
      onResize={(size) => data.onResize(note.id, size)}
    />
  );
}

export const nodeTypes = { tile: TileNode, sticky: NoteNode };
