// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, IconButton } from '@orcon/ui';
import { Plus, Trash2 } from 'lucide-react';
import { useIntl } from 'react-intl';

import { isExpression } from '../../model/node-config';

import type { FieldProps } from './fields';
import { TextControl } from './fields';
import { configMessages } from './messages';

type Entry = Record<string, unknown>;

const entriesOf = (value: unknown): Entry[] =>
  Array.isArray(value) ? (value as unknown[]).map((entry) => (typeof entry === 'object' && entry !== null && !Array.isArray(entry) ? (entry as Entry) : {})) : [];

const text = (value: unknown): string => (typeof value === 'string' ? value : typeof value === 'number' || typeof value === 'boolean' ? String(value) : '');

/**
 * A list of entries, each with the fields the parameter declares (a name and a value for headers,
 * query parameters and field assignments). Entries are added and removed with buttons; a value that
 * starts with `=` is an expression and shows in monospace.
 */
export function CollectionField({ def, value, onChange, onFocus, onBlur, t }: FieldProps) {
  const intl = useIntl();
  const entries = entriesOf(value);
  const label = t(`param.${def.name}.label`);
  const fields = def.fields ?? [];
  const change = (index: number, field: string, next: string) => {
    onChange(entries.map((entry, position) => (position === index ? { ...entry, [field]: next } : entry)));
  };
  return (
    <div className="flex flex-col gap-2">
      {entries.length === 0 ? <p className="text-fg-muted text-sm">{intl.formatMessage(configMessages.noEntries)}</p> : null}
      {entries.map((entry, index) => (
        // The entries have no identity of their own: position is the key, and every edit rewrites the list.
        <fieldset key={index} className="flex items-start gap-2" onBlur={onBlur}>
          <legend className="sr-only">{intl.formatMessage(configMessages.entry, { field: label, index: index + 1 })}</legend>
          <div className="grid min-w-0 flex-1 grid-cols-1 gap-2 sm:grid-cols-2">
            {fields.map((field) => (
              <TextControl
                key={field.name}
                multiline={false}
                mono={field.name !== 'name' && isExpression(entry[field.name])}
                aria-label={`${t(`param.${def.name}.field.${field.name}`)}, ${String(index + 1)}`}
                value={text(entry[field.name])}
                onChange={(next) => { change(index, field.name, next); }}
                onFocus={onFocus}
                onBlur={onBlur}
              />
            ))}
          </div>
          <IconButton
            variant="ghost"
            icon={Trash2}
            label={intl.formatMessage(configMessages.removeEntry, { field: label, index: index + 1 })}
            onClick={() => {
              onChange(entries.filter((_, position) => position !== index));
            }}
          />
        </fieldset>
      ))}
      <Button
        variant="secondary"
        className="self-start"
        onClick={() => {
          onChange([...entries, Object.fromEntries(fields.map((field) => [field.name, '']))]);
        }}
      >
        <Plus aria-hidden className="size-icon-16" />
        {intl.formatMessage(configMessages.addEntry, { field: label })}
      </Button>
    </div>
  );
}
