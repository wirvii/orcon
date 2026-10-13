// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { NodeResizeControl } from '@xyflow/react';
import { StickyNote as StickyNoteIcon } from 'lucide-react';
import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { canvasMessages } from '../messages';
import { MIN_NOTE, type StickyColor, type StickyNoteData } from '../model';

import { NoteMarkdown } from './markdown';

interface NoteColorClasses {
  surface: string;
  accent: string;
  swatch: string;
}

/** Static class names per decorative color, so Tailwind finds them. Never a status color. */
const colors: Record<StickyColor, NoteColorClasses> = {
  1: { surface: 'bg-canvas-sticky-1-bg border-canvas-sticky-1-border', accent: 'text-canvas-sticky-1-accent', swatch: 'bg-canvas-sticky-1-bg border-canvas-sticky-1-border' },
  2: { surface: 'bg-canvas-sticky-2-bg border-canvas-sticky-2-border', accent: 'text-canvas-sticky-2-accent', swatch: 'bg-canvas-sticky-2-bg border-canvas-sticky-2-border' },
  3: { surface: 'bg-canvas-sticky-3-bg border-canvas-sticky-3-border', accent: 'text-canvas-sticky-3-accent', swatch: 'bg-canvas-sticky-3-bg border-canvas-sticky-3-border' },
  4: { surface: 'bg-canvas-sticky-4-bg border-canvas-sticky-4-border', accent: 'text-canvas-sticky-4-accent', swatch: 'bg-canvas-sticky-4-bg border-canvas-sticky-4-border' },
  5: { surface: 'bg-canvas-sticky-5-bg border-canvas-sticky-5-border', accent: 'text-canvas-sticky-5-accent', swatch: 'bg-canvas-sticky-5-bg border-canvas-sticky-5-border' },
  6: { surface: 'bg-canvas-sticky-6-bg border-canvas-sticky-6-border', accent: 'text-canvas-sticky-6-accent', swatch: 'bg-canvas-sticky-6-bg border-canvas-sticky-6-border' },
};

const stickyColors = [1, 2, 3, 4, 5, 6] as const satisfies readonly StickyColor[];

export interface StickyNoteProps {
  note: Pick<StickyNoteData, 'id' | 'color' | 'title' | 'body' | 'size'>;
  selected?: boolean | undefined;
  /** The note can be edited, resized and recolored. */
  editable?: boolean | undefined;
  /** Renders the live resize handle of React Flow. Without it the grip is only drawn. */
  live?: boolean | undefined;
  tabbable?: boolean | undefined;
  /** Start in edit mode (stories). */
  defaultEditing?: boolean | undefined;
  /** Controlled edit mode: the canvas opens it from the keyboard. */
  editing?: boolean | undefined;
  onEditingChange?: ((editing: boolean) => void) | undefined;
  onChange?: ((patch: Partial<Pick<StickyNoteData, 'body' | 'color' | 'title'>>) => void) | undefined;
  /** The resize ended with this size. */
  onResize?: ((size: { w: number; h: number }) => void) | undefined;
  className?: string | undefined;
}

/**
 * A free-text note that sits behind the nodes: a colored card with a header, a Markdown subset body
 * (no raw HTML), a resize grip and, when selected, a row of six decorative colors. Double-click or
 * Enter edits the text; Ctrl or Cmd + Enter or leaving the field saves it. The colors are decorative:
 * a sticky note never uses a status color.
 */
export function StickyNote({ note, selected = false, editable = false, live = false, tabbable = false, defaultEditing = false, editing: editingProp, onEditingChange, onChange, onResize, className }: StickyNoteProps) {
  const intl = useIntl();
  const look = colors[note.color];
  const [localEditing, setLocalEditing] = useState(defaultEditing);
  const editing = editingProp ?? localEditing;
  const setEditing = (next: boolean) => {
    setLocalEditing(next);
    onEditingChange?.(next);
  };
  const [draft, setDraft] = useState(note.body);
  const field = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (editing) {
      setDraft(note.body);
      field.current?.focus();
    }
    // The draft starts from the saved text each time editing opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only opening the editor resets the draft
  }, [editing]);

  const save = () => {
    setEditing(false);
    if (draft !== note.body) onChange?.({ body: draft });
  };
  const onFieldKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    event.stopPropagation();
    if (event.key === 'Escape') {
      setDraft(note.body);
      setEditing(false);
    } else if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      save();
    }
  };
  const title = note.title ?? intl.formatMessage(canvasMessages.stickyLabel);

  return (
    <div
      data-sticky-note={note.id}
      data-selected={selected || undefined}
      className={cn(
        'relative flex size-full flex-col gap-1 rounded-md border px-3 py-2 text-xs shadow-sm',
        look.surface,
        selected && 'outline-canvas-selection outline-2 outline-offset-4',
        className,
      )}
      onDoubleClick={() => {
        if (editable) {
          setDraft(note.body);
          setEditing(true);
        }
      }}
    >
      {selected && editable ? (
        <div
          role="group"
          aria-label={intl.formatMessage(canvasMessages.stickyColor)}
          className="nodrag nopan bg-surface-raised border-canvas-node-border absolute bottom-full left-0 z-30 mb-2 flex gap-1 rounded-md border p-1 shadow-md"
        >
          {stickyColors.map((color) => (
            <button
              key={color}
              type="button"
              aria-label={intl.formatMessage(canvasMessages.stickyColorOption, { number: color })}
              aria-pressed={note.color === color}
              tabIndex={tabbable ? 0 : -1}
              onClick={(event) => {
                event.stopPropagation();
                onChange?.({ color });
              }}
              className={cn('size-icon-20 min-h-target min-w-target rounded-full border-2 p-0', colors[color].swatch, note.color === color && 'outline-fg outline-2 outline-offset-1')}
            />
          ))}
        </div>
      ) : null}
      <div className={cn('flex items-center gap-1 text-xs leading-4 font-semibold', look.accent)}>
        <StickyNoteIcon aria-hidden className="size-icon-16 shrink-0" />
        <span className="truncate" data-truncate>
          {title}
        </span>
      </div>
      {editing ? (
        <textarea
          ref={field}
          value={draft}
          aria-label={intl.formatMessage(canvasMessages.stickyText)}
          placeholder={intl.formatMessage(canvasMessages.stickyPlaceholder)}
          onChange={(event) => setDraft(event.target.value)}
          onBlur={save}
          onKeyDown={onFieldKeyDown}
          className="nodrag nopan nowheel text-fg min-h-0 w-full flex-1 resize-none bg-transparent text-xs leading-4 outline-hidden"
        />
      ) : (
        <div className="text-fg flex flex-col gap-1 overflow-hidden text-xs leading-4 wrap-anywhere">
          {note.body.trim() === '' ? <p className="text-fg-muted">{intl.formatMessage(canvasMessages.stickyPlaceholder)}</p> : <NoteMarkdown text={note.body} />}
        </div>
      )}
      {editable ? (
        live ? (
          <NodeResizeControl
            position="bottom-right"
            minWidth={MIN_NOTE.w}
            minHeight={MIN_NOTE.h}
            onResizeEnd={(_event, params) => onResize?.({ w: params.width, h: params.height })}
            aria-label={intl.formatMessage(canvasMessages.stickyResize)}
            className="nodrag border-0 bg-transparent"
          >
            <Grip className={look.accent} />
          </NodeResizeControl>
        ) : (
          <Grip className={look.accent} />
        )
      ) : null}
    </div>
  );
}

/** The 10 px accent corner at the bottom right. */
function Grip({ className }: { className?: string | undefined }) {
  return (
    <span
      aria-hidden
      className={cn('border-r-2 border-b-2 pointer-events-none absolute right-1 bottom-1 size-port rounded-br-sm border-current opacity-80', className)}
    />
  );
}
