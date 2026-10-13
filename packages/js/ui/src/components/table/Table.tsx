// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { HTMLAttributes, TdHTMLAttributes, ThHTMLAttributes } from 'react';

import { cn } from '../../lib/cn';

/** A table element. Give it a caption or aria-label so it has a name. Column widths follow the content. */
export function Table({ className, ...props }: HTMLAttributes<HTMLTableElement>) {
  return <table className={cn('text-fg w-full border-separate border-spacing-0 text-base', className)} {...props} />;
}

export interface TableCaptionProps extends HTMLAttributes<HTMLTableCaptionElement> {
  /** Keep the caption for screen readers only. It still names the table. */
  visuallyHidden?: boolean | undefined;
}

export function TableCaption({ className, visuallyHidden = false, ...props }: TableCaptionProps) {
  return <caption className={cn(visuallyHidden ? 'sr-only' : 'text-fg-muted pb-2 text-left text-sm', className)} {...props} />;
}

export function TableHeader({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return <thead className={cn(className)} {...props} />;
}

export function TableBody({ className, ...props }: HTMLAttributes<HTMLTableSectionElement>) {
  return <tbody className={className} {...props} />;
}

export interface TableRowProps extends HTMLAttributes<HTMLTableRowElement> {
  /** A selected row gets a background and an inset frame, so selection is not only a color. Add a checkbox cell when people can change the selection. */
  selected?: boolean | undefined;
  /** A body row: its height follows the density (40 px comfortable, 32 px compact). Header rows are not data rows. */
  header?: boolean | undefined;
}

export function TableRow({ className, selected = false, header = false, ...props }: TableRowProps) {
  return (
    <tr
      data-row={header ? undefined : ''}
      data-state={selected ? 'selected' : undefined}
      className={cn(
        header ? 'h-row' : 'h-row hover:bg-surface-sunken',
        'data-[state=selected]:bg-surface-sunken data-[state=selected]:outline-2 data-[state=selected]:-outline-offset-2 data-[state=selected]:outline-action-primary',
        className,
      )}
      {...props}
    />
  );
}

export interface TableHeadProps extends ThHTMLAttributes<HTMLTableCellElement> {
  /** Keep the header visible while the body scrolls. Put the table in a scrolling container with a height. */
  sticky?: boolean | undefined;
}

export function TableHead({ className, sticky = false, scope = 'col', ...props }: TableHeadProps) {
  return (
    <th
      scope={scope}
      className={cn(
        'text-fg-muted border-border border-b px-control-x text-left text-sm font-medium whitespace-nowrap',
        sticky && 'sticky top-0 z-sticky bg-surface',
        className,
      )}
      {...props}
    />
  );
}

export interface TableCellProps extends TdHTMLAttributes<HTMLTableCellElement> {
  /**
   * Let the text wrap. By default a cell stays on one line so every row is exactly as tall as
   * the density says; a wrapping cell makes its row taller. Declared truncation needs a tooltip.
   */
  wrap?: boolean | undefined;
}

export function TableCell({ className, wrap = false, ...props }: TableCellProps) {
  return <td className={cn('border-border border-b px-control-x py-0 align-middle', !wrap && 'whitespace-nowrap', className)} {...props} />;
}
