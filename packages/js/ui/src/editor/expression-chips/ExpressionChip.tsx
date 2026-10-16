// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Braces, X } from 'lucide-react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { editorMessages } from '../messages';

export interface ExpressionChipProps {
  /**
   * What the chip stands for. `path` is a field of the data of this step; `node` a field of another step (its name
   * is shown first, dimmed); `raw` an expression that is more than a path, shown as its own text.
   */
  kind: 'path' | 'node' | 'raw';
  /** The readable form: `customer.phone`, or the expression itself for `raw`. */
  label: string;
  /** The name of the other step, for `node`. */
  node?: string | undefined;
  /** Takes the chip away. Without it the chip only shows. */
  onRemove?: (() => void) | undefined;
  className?: string | undefined;
}

/**
 * A piece of an expression shown the way a person reads it (SPEC-019 R-UX-NDV-03): a field as `customer.phone`, a field
 * of another step as `Webhook > id`, any other expression as raw text in a chip marked with braces. It is a view
 * of the stored text, never a different value.
 */
export function ExpressionChip({ kind, label, node, onRemove, className }: ExpressionChipProps) {
  const intl = useIntl();
  return (
    <span
      data-expression-chip={kind}
      className={cn(
        'bg-surface-sunken border-border text-fg-accent inline-flex max-w-full min-w-0 items-center gap-1 rounded-md border px-2 font-mono text-mono',
        kind === 'raw' && 'text-fg',
        className,
      )}
    >
      {kind === 'raw' ? <Braces aria-hidden className="text-fg-muted size-icon-16 shrink-0" /> : null}
      {kind === 'node' && node ? <span className="text-fg-muted shrink-0">{node} ›</span> : null}
      <span className="min-w-0 truncate" title={label}>
        {label}
      </span>
      {onRemove ? (
        <button
          type="button"
          aria-label={intl.formatMessage(editorMessages.chipRemove, { name: node ? `${node} › ${label}` : label })}
          onClick={onRemove}
          className="text-fg-muted hover:text-fg -me-1 inline-flex min-h-target min-w-target shrink-0 items-center justify-center rounded-md"
        >
          <X aria-hidden className="size-icon-16" />
        </button>
      ) : null}
    </span>
  );
}
