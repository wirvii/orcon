// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useIntl } from 'react-intl';

import { Badge } from '../../components/badge/Badge';
import { cn } from '../../lib/cn';
import { editorMessages } from '../messages';
import { inferSchema, type DataItem, type SchemaField, type SchemaType } from '../model';

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
  label?: string;
  className?: string;
}

function Field({ field, depth }: { field: SchemaField; depth: number }) {
  const intl = useIntl();
  // The indent follows the depth, which only exists at run time.
  return (
    <li className="flex flex-col">
      <div
        data-schema-field={field.key}
        // eslint-disable-next-line orcon/no-inline-style -- indent computed from the depth of the field
        style={{ paddingInlineStart: `calc(var(--orcon-space-4) * ${depth})` }}
        className="flex min-h-row items-center gap-2 px-3"
      >
        <span className="text-fg text-mono">{field.key}</span>
        <Badge>{intl.formatMessage(typeLabels[field.type])}</Badge>
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
      </div>
      {field.children && field.children.length > 0 ? (
        <ul>
          {field.children.map((child) => (
            <Field key={child.key} field={child} depth={depth + 1} />
          ))}
        </ul>
      ) : null}
    </li>
  );
}

/**
 * The fields of the data, with a neutral type badge and a short example each (60 characters at most,
 * masked when the value is protected). It describes the first item: that is the shape the next node can rely on.
 */
export function SchemaView({ items, label, className }: SchemaViewProps) {
  const intl = useIntl();
  const fields = inferSchema(items);
  return (
    // The list scrolls on its own, so it takes keyboard focus to be scrollable by keyboard.
    <ul tabIndex={0} aria-label={label ?? intl.formatMessage(editorMessages.schemaLabel)} data-schema-view className={cn('h-full overflow-auto py-2', className)}>
      {fields.map((field) => (
        <Field key={field.key} field={field} depth={0} />
      ))}
    </ul>
  );
}
