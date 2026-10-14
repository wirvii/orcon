// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CanvasOutline, type Graph } from '@orcon/ui';
import { StickyNote, TriangleAlert } from 'lucide-react';
import { useMemo } from 'react';
import { useIntl } from 'react-intl';

import { toCanvasGraph } from '../model/canvas-graph';
import { useEditorStore } from '../store/editor-context';

import { editorMessages } from './messages';

export interface EditorOutlineProps {
  onFocusNode: (id: string) => void;
  onDisconnect: (edgeId: string) => void;
  onDeleteNode: (id: string) => void;
  editable: boolean;
}

/** The first line of a note, which is how the outline names it. */
const firstLine = (body: string): string => body.split('\n').find((line) => line.trim() !== '')?.replace(/^#+\s*/, '').trim() ?? '';

/**
 * The flow as text for a screen reader (and for anybody): the design system's outline of steps and
 * connections, with the editor's additions on top and below: the steps that cannot run until
 * somebody acts, and the sticky notes by their first line (SPEC-014 R-ED-A11Y-02). Pinned and
 * disabled marks come from the design system's outline.
 */
export function EditorOutline({ onFocusNode, onDisconnect, onDeleteNode, editable }: EditorOutlineProps) {
  const intl = useIntl();
  const document = useEditorStore((state) => state.document);
  const graph = useMemo(() => toCanvasGraph(document), [document]);
  const unsupported = graph.nodes.filter((node) => node.kind === 'placeholder');
  const notes = graph.notes ?? [];
  // The notes are listed below with the editor's wording, so the design system's list is left out here.
  const forOutline: Graph = { ...graph, notes: [] };

  return (
    <div data-editor-outline className="flex flex-col gap-4">
      {unsupported.length > 0 ? (
        <div className="flex flex-col gap-2" data-outline-attention>
          <h3 className="text-md font-semibold">{intl.formatMessage(editorMessages.outlineAttention)}</h3>
          <ul className="flex flex-col gap-1">
            {unsupported.map((node) => (
              <li key={node.id} className="text-status-warning flex items-start gap-2 text-sm">
                <TriangleAlert aria-hidden className="mt-1 size-icon-16 shrink-0" />
                <span className="wrap-anywhere">{intl.formatMessage(editorMessages.outlineUnsupported, { name: node.name })}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      <CanvasOutline graph={forOutline} editable={editable} headingLevel={3} onFocusNode={onFocusNode} onDisconnect={onDisconnect} onDeleteNode={onDeleteNode} />
      {notes.length > 0 ? (
        <div className="flex flex-col gap-2" data-outline-notes>
          <h3 className="text-md font-semibold">{intl.formatMessage(editorMessages.outlineNotes)}</h3>
          <ul className="flex flex-col gap-2">
            {notes.map((note) => (
              <li key={note.id} className="flex items-start gap-2 text-sm">
                <StickyNote aria-hidden className="text-fg-muted mt-1 size-icon-16 shrink-0" />
                <span className="wrap-anywhere">{intl.formatMessage(editorMessages.outlineNote, { text: note.title ?? firstLine(note.body) })}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
