// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ChevronDown, ChevronRight, Download, Lock } from 'lucide-react';
import { useLayoutEffect, useRef, useState, type CSSProperties } from 'react';
import { useIntl } from 'react-intl';

import { RedactedValue } from '../../components/redacted-value/RedactedValue';
import { cn } from '../../lib/cn';
import { editorMessages } from '../messages';
import { columnsOf, isBinary, isProtected, visibleValue, type DataItem, type ProtectedValue } from '../model';

/** Above this many items only the rows in view are rendered. */
export const VIRTUALIZE_ROWS = 100;
const OVERSCAN = 8;

/** The JSON literal null, shown dimmed. It is data, not a message. */
const NULL = String(null);

export interface DataTableViewProps {
  items: readonly DataItem[];
  /** Accessible name of the scrollable region. */
  label?: string;
  className?: string;
}

/** A protected value: the permission notice, a masked secret, or the history notice. It is never blank and never the real value. */
export function ProtectedCell({ value }: { value: ProtectedValue }) {
  const intl = useIntl();
  if (value.$protected === 'permission') return <RedactedValue />;
  const text = intl.formatMessage(value.$protected === 'history' ? editorMessages.hiddenInHistory : editorMessages.secretMasked);
  return (
    <span data-redacted={value.$protected} className="bg-surface-sunken text-fg-muted inline-flex items-center gap-1 rounded-sm px-2 font-mono text-mono">
      <Lock aria-hidden className="size-icon-16 shrink-0" />
      {text}
    </span>
  );
}

function BinaryCell({ value }: { value: ReturnType<typeof asBinary> }) {
  const intl = useIntl();
  const { name, mediaType, size, href } = value;
  return (
    <span className="inline-flex items-center gap-2">
      <span>
        {name} <span className="text-fg-muted">· {mediaType} · {intl.formatMessage(editorMessages.fileSize, { size })}</span>
      </span>
      {href ? (
        <a href={href} download={name} aria-label={intl.formatMessage(editorMessages.download, { name })} className="text-fg-link inline-flex min-h-target items-center rounded-sm px-1 underline">
          <Download aria-hidden className="size-icon-16" />
        </a>
      ) : null}
    </span>
  );
}
const asBinary = (value: unknown) => (isBinary(value) ? value.$binary : { name: '', mediaType: '', size: 0, href: undefined as string | undefined });

function Cell({ name, value }: { name: string; value: unknown }) {
  const intl = useIntl();
  const [open, setOpen] = useState(false);
  if (isProtected(value)) return <ProtectedCell value={value} />;
  if (isBinary(value)) return <BinaryCell value={asBinary(value)} />;
  if (value === null || value === undefined) return <span className="text-fg-subtle">{NULL}</span>;
  if (typeof value === 'object') {
    const list = Array.isArray(value);
    const count = list ? value.length : Object.keys(value).length;
    const summary = intl.formatMessage(list ? editorMessages.nestedList : editorMessages.nestedKeys, { count });
    return (
      <span className="flex flex-col items-start gap-1">
        <button
          type="button"
          aria-expanded={open}
          aria-label={intl.formatMessage(open ? editorMessages.collapse : editorMessages.expand, { name })}
          onClick={() => setOpen((current) => !current)}
          className="text-fg-link hover:bg-surface-sunken inline-flex min-h-target items-center gap-1 rounded-sm px-1"
        >
          {open ? <ChevronDown aria-hidden className="size-icon-16" /> : <ChevronRight aria-hidden className="size-icon-16" />}
          <span aria-hidden>{summary}</span>
        </button>
        {open ? <pre className="bg-surface-sunken rounded-md p-2 text-mono whitespace-pre-wrap">{JSON.stringify(visibleValue(value, intl.formatMessage(editorMessages.secretMasked)), null, 2)}</pre> : null}
      </span>
    );
  }
  return <span>{String(value)}</span>;
}

/** The row height of the table, from the density token, so windowing matches the rendered rows. */
function useRowHeight(ref: React.RefObject<HTMLElement | null>): number {
  const [height, setHeight] = useState(40);
  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const value = Number.parseFloat(getComputedStyle(element).getPropertyValue('--orcon-size-row'));
    if (Number.isFinite(value) && value > 0) setHeight(value);
  }, [ref]);
  return height;
}

/**
 * The items of a node as a table: a "#" column, one column per field, values in mono text,
 * `null` dimmed, nested objects collapsed and expandable, protected values as notices, and a sticky
 * header. Long lists render only the rows in view. The region scrolls on its own and has a name.
 */
export function DataTableView({ items, label, className }: DataTableViewProps) {
  const intl = useIntl();
  const region = useRef<HTMLDivElement>(null);
  const rowHeight = useRowHeight(region);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewport, setViewport] = useState(320);
  const columns = columnsOf(items);
  const windowed = items.length > VIRTUALIZE_ROWS;
  const first = windowed ? Math.max(0, Math.floor(scrollTop / rowHeight) - OVERSCAN) : 0;
  const last = windowed ? Math.min(items.length, Math.ceil((scrollTop + viewport) / rowHeight) + OVERSCAN) : items.length;
  const visible = items.slice(first, last);
  const spacer = (rows: number): CSSProperties => ({ height: rows * rowHeight });

  useLayoutEffect(() => {
    if (region.current) setViewport(region.current.clientHeight);
  }, [items.length]);

  if (items.length === 0) {
    return <p className={cn('text-fg-muted p-4 text-base', className)}>{intl.formatMessage(editorMessages.tableEmpty)}</p>;
  }
  return (
    <div
      ref={region}
      role="region"
      tabIndex={0}
      aria-label={label ?? intl.formatMessage(editorMessages.tableLabel)}
      data-data-table
      onScroll={(event) => windowed && setScrollTop(event.currentTarget.scrollTop)}
      className={cn('max-w-full overflow-auto', className)}
    >
      <table className="w-full min-w-max border-collapse text-mono">
        <caption className="sr-only">{label ?? intl.formatMessage(editorMessages.tableLabel)}</caption>
        <thead>
          <tr>
            <th scope="col" className="bg-surface-sunken text-fg-subtle border-border sticky top-0 z-10 w-10 border-b px-3 py-2 text-left text-xs font-semibold">
              {intl.formatMessage(editorMessages.rowNumber)}
            </th>
            {columns.map((column) => (
              <th key={column} scope="col" className="bg-surface-sunken text-fg-muted border-border sticky top-0 z-10 border-b px-3 py-2 text-left text-xs font-semibold whitespace-nowrap">
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {first > 0 ? (
            <tr aria-hidden>
              {/* The rows above the window are one empty row of the same total height. */}
              {/* eslint-disable-next-line orcon/no-inline-style -- spacer height computed from the scroll position */}
              <td colSpan={columns.length + 1} style={spacer(first)} />
            </tr>
          ) : null}
          {visible.map((item, offset) => {
            const index = first + offset;
            return (
              <tr key={index} data-row={index} aria-rowindex={index + 2} className="h-row">
                <td className="text-fg-subtle border-border border-b px-3 align-middle">{index + 1}</td>
                {columns.map((column) => (
                  <td key={column} className="text-fg border-border border-b px-3 align-middle whitespace-nowrap">
                    <Cell name={column} value={item[column]} />
                  </td>
                ))}
              </tr>
            );
          })}
          {last < items.length ? (
            <tr aria-hidden>
              {/* eslint-disable-next-line orcon/no-inline-style -- spacer height computed from the scroll position */}
              <td colSpan={columns.length + 1} style={spacer(items.length - last)} />
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  );
}
