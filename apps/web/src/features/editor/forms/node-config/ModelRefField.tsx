// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { FieldProps } from './fields';
import { TextControl } from './fields';

/** A model reference as the AI nodes store it: the provider and the model name (orcon/flow `model`). */
interface ModelRef {
  provider?: string;
  name?: string;
}

const asRef = (value: unknown): ModelRef => (typeof value === 'object' && value !== null && !Array.isArray(value) ? value : {});

/**
 * The model of an AI node: two text boxes, the provider and the model name, written as one object. The
 * provider is free text on purpose: the list of providers comes with the connectors, not with the editor.
 */
export function ModelRefField({ value, t, onChange, onFocus, onBlur }: FieldProps) {
  const ref = asRef(value);
  const write = (part: keyof ModelRef) => (text: string) => {
    onChange({ ...ref, [part]: text });
  };
  // Each box has a visible caption as well as an accessible name: the two boxes are not told apart by position alone.
  const part = (key: keyof ModelRef, caption: string) => (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="text-fg-muted text-xs">{caption}</span>
      <TextControl multiline={false} mono={false} value={ref[key] ?? ''} onChange={write(key)} onFocus={onFocus} onBlur={onBlur} aria-label={caption} />
    </div>
  );
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      {part('provider', t('param.model.field.provider'))}
      {part('name', t('param.model.field.name'))}
    </div>
  );
}
