// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Checkbox, Input, Select, Textarea } from '@orcon/ui';
import { useId, type ChangeEvent } from 'react';
import { useIntl } from 'react-intl';

import { expressionSource, isExpression, toExpression } from '../../model/node-config';
import type { ParamDef } from '../../model/node-manifests';

import { configMessages } from './messages';

/** What every widget of a parameter receives: the value, how to change it, and the translated texts. */
export interface FieldProps {
  def: ParamDef;
  value: unknown;
  onChange: (value: unknown) => void;
  onFocus: () => void;
  onBlur: () => void;
  t: (key: string) => string;
}

const asText = (value: unknown): string => (typeof value === 'string' ? value : typeof value === 'number' || typeof value === 'boolean' ? String(value) : '');

interface TextControlProps {
  multiline: boolean;
  mono: boolean;
  value: string;
  onChange: (text: string) => void;
  onFocus: () => void;
  onBlur: () => void;
  type?: 'text' | 'number' | 'datetime-local';
  'aria-label'?: string;
}

/** A text box: one line or several, in monospace when it holds code or a formula. */
export function TextControl({ multiline, mono, value, onChange, onFocus, onBlur, type = 'text', 'aria-label': label }: TextControlProps) {
  const className = mono ? 'font-mono text-mono' : undefined;
  if (multiline) {
    return <Textarea rows={4} value={value} aria-label={label} className={className} onFocus={onFocus} onBlur={onBlur} onChange={(event: ChangeEvent<HTMLTextAreaElement>) => { onChange(event.target.value); }} />;
  }
  return <Input type={type} value={value} aria-label={label} className={className} onFocus={onFocus} onBlur={onBlur} onChange={(event: ChangeEvent<HTMLInputElement>) => { onChange(event.target.value); }} />;
}

/** The control of a parameter in expression mode: the formula as text. The expression editor replaces it in a later cut. */
export function ExpressionControl({ def, value, onChange, onFocus, onBlur }: Omit<FieldProps, 't'>) {
  return (
    <TextControl
      multiline={def.multiline === true || def.type === 'JSON'}
      mono
      value={expressionSource(value)}
      onChange={(source) => {
        onChange(toExpression(source));
      }}
      onFocus={onFocus}
      onBlur={onBlur}
    />
  );
}

export function StringField({ def, value, onChange, onFocus, onBlur }: FieldProps) {
  return <TextControl multiline={def.multiline === true} mono={false} value={asText(value)} onChange={onChange} onFocus={onFocus} onBlur={onBlur} />;
}

/** A number is stored as a number while the text is one, and as the raw text otherwise, so the form can point at the mistake. */
export function NumberField({ value, onChange, onFocus, onBlur }: FieldProps) {
  return (
    <TextControl
      multiline={false}
      mono={false}
      value={asText(value)}
      onChange={(text) => {
        const trimmed = text.trim();
        onChange(trimmed === '' ? undefined : Number.isFinite(Number(trimmed)) ? Number(trimmed) : text);
      }}
      onFocus={onFocus}
      onBlur={onBlur}
    />
  );
}

export function JsonField({ value, onChange, onFocus, onBlur }: FieldProps) {
  return <TextControl multiline mono value={asText(value)} onChange={(text) => { onChange(text === '' ? undefined : text); }} onFocus={onFocus} onBlur={onBlur} />;
}

export function BinaryField(props: FieldProps) {
  return <StringField {...props} />;
}

const pad = (n: number) => String(n).padStart(2, '0');

/** An instant (RFC 3339, UTC) as the value of a `datetime-local` input, in the person's time zone. */
function toLocalInput(iso: unknown): string {
  if (typeof iso !== 'string' || isExpression(iso)) return '';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return '';
  return `${String(date.getFullYear())}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function DateTimeField({ value, onChange, onFocus, onBlur }: FieldProps) {
  return (
    <TextControl
      multiline={false}
      mono={false}
      type="datetime-local"
      value={toLocalInput(value)}
      onChange={(local) => {
        const date = new Date(local);
        onChange(local === '' || Number.isNaN(date.getTime()) ? undefined : date.toISOString().replace('.000Z', 'Z'));
      }}
      onFocus={onFocus}
      onBlur={onBlur}
    />
  );
}

const NONE = '__none__';

/** The labels of the choices of an OPTIONS parameter, from the catalog. */
export function optionList(def: ParamDef, t: (key: string) => string) {
  return (def.options ?? []).map((option) => ({ value: option, label: t(`param.${def.name}.option.${option}`) }));
}

export function OptionsField({ def, value, onChange, t }: FieldProps) {
  const intl = useIntl();
  const known = typeof value === 'string' && (def.options ?? []).includes(value);
  return (
    <Select
      value={known ? value : NONE}
      onValueChange={(next) => {
        onChange(next === NONE ? undefined : next);
      }}
      options={[...(known ? [] : [{ value: NONE, label: intl.formatMessage(configMessages.choose) }]), ...optionList(def, t)]}
    />
  );
}

export function MultiOptionsField({ def, value, onChange, t }: FieldProps) {
  const chosen = Array.isArray(value) ? (value as unknown[]).filter((entry): entry is string => typeof entry === 'string') : [];
  return (
    <div className="flex flex-col gap-1">
      {(def.options ?? []).map((option) => (
        <Checkbox
          key={option}
          label={t(`param.${def.name}.option.${option}`)}
          checked={chosen.includes(option)}
          onCheckedChange={(checked) => {
            const next = checked === true ? [...chosen, option] : chosen.filter((entry) => entry !== option);
            onChange(next.length === 0 ? undefined : (def.options ?? []).filter((entry) => next.includes(entry)));
          }}
        />
      ))}
    </div>
  );
}

interface Locator {
  mode: 'id' | 'url';
  value: string;
}

const LOCATOR_MODES = ['id', 'url'] as const;

const asLocator = (value: unknown): Locator => {
  if (typeof value === 'object' && value !== null) {
    const { mode, value: text } = value as Record<string, unknown>;
    return { mode: mode === 'url' ? 'url' : 'id', value: typeof text === 'string' ? text : '' };
  }
  return { mode: 'id', value: '' };
};

/** A resource given by identifier or by address (the list mode needs the connector to look resources up, which comes with the server). */
export function ResourceLocatorField({ def, value, onChange, onFocus, onBlur }: FieldProps) {
  const intl = useIntl();
  const locator = asLocator(value);
  const name = useId();
  return (
    <div className="flex flex-col gap-2">
      <div role="group" aria-label={intl.formatMessage(configMessages.locatorMode, { field: def.name })} className="border-control inline-flex self-start overflow-hidden rounded-md border">
        {LOCATOR_MODES.map((mode) => (
          <button
            key={mode}
            type="button"
            name={name}
            aria-pressed={locator.mode === mode}
            onClick={() => { onChange({ ...locator, mode }); }}
            className="text-fg bg-surface hover:bg-surface-sunken aria-pressed:bg-surface-sunken min-h-target px-2 text-xs aria-pressed:font-semibold"
          >
            {intl.formatMessage(mode === 'id' ? configMessages.locatorId : configMessages.locatorUrl)}
          </button>
        ))}
      </div>
      <TextControl multiline={false} mono={false} value={locator.value} onChange={(text) => { onChange({ ...locator, value: text }); }} onFocus={onFocus} onBlur={onBlur} />
    </div>
  );
}
