// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { GripVertical } from 'lucide-react';
import type { DragEvent } from 'react';
import { useIntl } from 'react-intl';

import { Badge } from '../../components/badge/Badge';
import { cn } from '../../lib/cn';
import { editorMessages } from '../messages';
import { FIELD_DRAG_TYPE, inferSchema, type DataItem, type FieldPath, type SchemaField, type SchemaType } from '../model';

const typeLabels = {
  string: editorMessages.typeString,
  number: editorMessages.typeNumber,
  boolean: editorMessages.typeBoolean,
  null: editorMessages.typeNull,
  object: editorMessages.typeObject,
  array: editorMessages.typeArray,
  file: editorMessages.typeFile,
  hidden: editorMessages.typeHidden,
} satisfies Record<SchemaType, unknown>;

export interface SchemaViewProps {
  items: readonly DataItem[];
  label?: string | undefined;
  /**
   * Makes the fields usable: each one can be dragged to a parameter (the drag carries its path under
   * `FIELD_DRAG_TYPE`) and activated with Enter, which calls this with the path of the field. A field whose
   * value the person may not see is never offered.
   */
  onInsert?: ((path: FieldPath) => void) | undefined;
  className?: string | undefined;
}

/** The readable name of a field path, for the accessible name: `lines[0].sku`. */
function pathName(path: FieldPath): string {
  return path.reduce<string>((name, step, index) => (typeof step === 'number' ? `${name}[${String(step)}]` : index === 0 ? step : `${name}.${step}`), '');
}

function Field({ field, depth, path, onInsert }: { field: SchemaField; depth: number; path: FieldPath; onInsert?: ((path: FieldPath) => void) | undefined }) {
  const intl = useIntl();
  const usable = onInsert !== undefined && field.type !== 'hidden';
  // The indent follows the depth, which only exists at run time.
  const content = (
    <>
      {usable ? <GripVertical aria-hidden className="text-fg-subtle size-icon-16 shrink-0" /> : null}
      <span className="text-fg text-mono">{field.key}</span>
      <Badge className="shrink-0">{intl.formatMessage(typeLabels[field.type])}</Badge>
      {field.sensitive ? (
        <span className="text-fg-muted text-mono" data-schema-example="masked">
          <span aria-hidden>••••••</span>
          <span className="sr-only">{intl.formatMessage(editorMessages.maskedExample)}</span>
        </span>
      ) : field.example !== undefined ? (
        <span className="text-fg-muted text-mono min-w-0 truncate" data-truncate data-schema-example={field.example} title={field.example}>
          {field.example}
        </span>
      ) : null}
      {/* What the button does, read after the field it names: the visible text stays part of the accessible name. */}
      {usable ? <span className="sr-only">{intl.formatMessage(editorMessages.schemaInsert, { name: pathName(path) })}</span> : null}
    </>
  );
  const rowClass = 'flex min-h-row w-full flex-wrap items-center gap-x-2 px-3 text-start';
  const rowStyle = { paddingInlineStart: `calc(var(--orcon-space-4) * ${String(depth)})` };
  return (
    <li className="flex flex-col">
      {usable ? (
        <button
          type="button"
          draggable
          data-schema-field={field.key}
          onClick={() => onInsert(path)}
          onDragStart={(event: DragEvent<HTMLButtonElement>) => {
            event.dataTransfer.setData(FIELD_DRAG_TYPE, JSON.stringify(path));
            event.dataTransfer.effectAllowed = 'copy';
          }}
          // eslint-disable-next-line orcon/no-inline-style -- indent computed from the depth of the field
          style={rowStyle}
          className={cn(rowClass, 'min-h-target hover:bg-surface-sunken cursor-grab active:cursor-grabbing')}
        >
          {content}
        </button>
      ) : (
        <div
          data-schema-field={field.key}
          // eslint-disable-next-line orcon/no-inline-style -- indent computed from the depth of the field
          style={rowStyle}
          className={rowClass}
        >
          {content}
        </div>
      )}
      {field.children && field.children.length > 0 ? (
        <ul>
          {field.children.map((child) => (
            <Field key={child.key} field={child} depth={depth + 1} path={[...path, field.type === 'array' ? Number(child.key) : child.key]} onInsert={onInsert} />
          ))}
        </ul>
      ) : null}
    </li>
  );
}

/**
 * The fields of the data, with a neutral type badge and a short example each (60 characters at most,
 * masked when the value is protected). It describes the first item: that is the shape the next node can rely on.
 * With `onInsert`, a field can be dragged to a parameter or chosen with the keyboard.
 */
export function SchemaView({ items, label, onInsert, className }: SchemaViewProps) {
  const intl = useIntl();
  const fields = inferSchema(items);
  return (
    // The list scrolls on its own, so it takes keyboard focus to be scrollable by keyboard.
    <div className="flex h-full min-h-0 flex-col">
      {onInsert && fields.length > 0 ? <p className="text-fg-muted border-border border-b px-3 py-2 text-sm">{intl.formatMessage(editorMessages.schemaHint)}</p> : null}
      <ul tabIndex={onInsert ? undefined : 0} aria-label={label ?? intl.formatMessage(editorMessages.schemaLabel)} data-schema-view className={cn('min-h-0 flex-1 overflow-auto py-2', className)}>
        {fields.map((field) => (
          <Field key={field.key} field={field} depth={0} path={[field.key]} onInsert={onInsert} />
        ))}
      </ul>
    </div>
  );
}
