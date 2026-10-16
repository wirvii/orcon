// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ExpressionChip } from '@orcon/ui';
import { useMemo } from 'react';
import { useIntl } from 'react-intl';

import { pathLabel, removeSegment, textToChips } from '../../model/expression-chips';

import { configMessages } from './messages';

export interface ExpressionChipsFieldProps {
  /** The stored value of the parameter: `=...` for an expression. */
  value: unknown;
  /** The name of the parameter, for the accessible name. */
  label: string;
  /** Writes the stored text back (after a chip was removed). */
  onChange: (stored: string) => void;
  onFocus: () => void;
  onBlur: () => void;
}

/**
 * A formula shown as chips (SPEC-019 R-UX-NDV-03): each field it reads as `customer.phone`, each other expression as raw
 * text in a chip, the text between them as text. It is a view of the stored string, which it never rewrites except to
 * take a chip out. Fields dragged from the input are added to it by the parameter around it; to change the text
 * itself the person switches the field to raw text.
 */
export function ExpressionChipsField({ value, label, onChange, onFocus, onBlur }: ExpressionChipsFieldProps) {
  const intl = useIntl();
  const stored = typeof value === 'string' ? value : '';
  const parsed = useMemo(() => textToChips(stored), [stored]);
  return (
    <div
      role="group"
      tabIndex={0}
      aria-label={intl.formatMessage(configMessages.chipsLabel, { field: label })}
      data-expression-chips-field
      onFocus={onFocus}
      onBlur={onBlur}
      className="bg-surface border-control focus-visible:outline-focus flex min-h-control-target flex-wrap items-center gap-1 rounded-md border px-2 py-1 focus-visible:outline-2 focus-visible:outline-offset-2"
    >
      {parsed.segments.length === 0 ? <span className="text-fg-subtle text-sm">{intl.formatMessage(configMessages.chipsDrop)}</span> : null}
      {parsed.segments.map((segment, index) =>
        segment.kind === 'text' ? (
          <span key={`${String(index)}-${segment.raw}`} className="font-mono text-mono whitespace-pre-wrap">
            {segment.raw}
          </span>
        ) : (
          <ExpressionChip
            key={`${String(index)}-${segment.raw}`}
            kind={segment.chip.type === 'json' ? 'path' : segment.chip.type}
            label={segment.chip.type === 'raw' ? segment.chip.label : pathLabel(segment.chip.path)}
            node={segment.chip.type === 'node' ? segment.chip.node : undefined}
            onRemove={() => {
              onChange(removeSegment(parsed, index));
            }}
          />
        ),
      )}
      {parsed.segments.length > 0 ? <span className="text-fg-subtle ms-1 text-xs">{intl.formatMessage(configMessages.chipsDrop)}</span> : null}
    </div>
  );
}
