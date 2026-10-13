// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ChevronDown, ChevronRight } from 'lucide-react';
import { useMemo, useState, type CSSProperties, type KeyboardEvent, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { cn } from '../../lib/cn';
import { ProtectedCell } from '../data-table-view/DataTableView';
import { editorMessages } from '../messages';
import { isBinary, isProtected } from '../model';

/** The JSON literal null, shown dimmed. It is data, not a message. */
const NULL = String(null);

export interface JsonTreeViewProps {
  /** Any JSON-like value: an array of items, an item, a list. Protected values and files are recognised. */
  value: unknown;
  /** Nodes deeper than this start collapsed. */
  defaultExpandedDepth?: number;
  label?: string;
  className?: string;
}

interface Row {
  path: string;
  key: string | undefined;
  value: unknown;
  depth: number;
  expandable: boolean;
  expanded: boolean;
  /** Entries under it, for the summary of a collapsed node. */
  size: number;
  /** Position among its siblings, for the tree semantics. */
  setSize: number;
  position: number;
  parent: string | undefined;
}

const entriesOf = (value: unknown): [string, unknown][] =>
  Array.isArray(value) ? value.map((entry, index) => [String(index), entry]) : Object.entries(value as Record<string, unknown>);

const branch = (value: unknown) => typeof value === 'object' && value !== null && !isProtected(value) && !isBinary(value);

function flatten(value: unknown, open: ReadonlySet<string>, depthLimit: number, closed: ReadonlySet<string>): Row[] {
  const rows: Row[] = [];
  const walk = (key: string | undefined, entry: unknown, depth: number, path: string, position: number, setSize: number, parent: string | undefined) => {
    const expandable = branch(entry);
    const size = expandable ? entriesOf(entry).length : 0;
    const expanded = expandable && (open.has(path) || (depth < depthLimit && !closed.has(path)));
    rows.push({ path, key, value: entry, depth, expandable, expanded, size, setSize, position, parent });
    if (expanded) {
      const children = entriesOf(entry);
      children.forEach(([childKey, child], index) => walk(childKey, child, depth + 1, `${path}/${childKey}`, index + 1, children.length, path));
    }
  };
  walk(undefined, value, 0, '$', 1, 1, undefined);
  return rows;
}

/** One primitive in the colors of the data panel: strings in the accent, numbers in the link color, booleans bold, null subtle. No other hues. */
function Primitive({ value }: { value: unknown }): ReactNode {
  if (isProtected(value)) return <ProtectedCell value={value} />;
  if (isBinary(value)) return <span className="text-fg-accent">{value.$binary.name}</span>;
  if (value === null || value === undefined) return <span className="text-fg-subtle">{NULL}</span>;
  if (typeof value === 'string') return <span className="text-fg-accent">&quot;{value}&quot;</span>;
  if (typeof value === 'number') return <span className="text-fg-link">{String(value)}</span>;
  if (typeof value === 'boolean') return <span className="text-fg font-semibold">{String(value)}</span>;
  return <span>{String(value)}</span>;
}

/**
 * A collapsible tree of JSON data in mono text. It is a tree widget: one Tab stop, the arrow keys move
 * between rows, Right and Left expand and collapse, Enter toggles. Keys are in the default color,
 * strings in the accent, numbers in the link color, booleans bold and null subtle.
 */
export function JsonTreeView({ value, defaultExpandedDepth = 2, label, className }: JsonTreeViewProps) {
  const intl = useIntl();
  const [open, setOpen] = useState<ReadonlySet<string>>(() => new Set());
  const [closed, setClosed] = useState<ReadonlySet<string>>(() => new Set());
  const [focused, setFocused] = useState('$');
  const rows = useMemo(() => flatten(value, open, defaultExpandedDepth, closed), [value, open, closed, defaultExpandedDepth]);

  const toggle = (row: Row, expand: boolean) => {
    if (!row.expandable) return;
    const add = (set: ReadonlySet<string>, path: string) => new Set(set).add(path);
    const drop = (set: ReadonlySet<string>, path: string) => {
      const next = new Set(set);
      next.delete(path);
      return next;
    };
    if (expand) {
      setOpen((current) => add(current, row.path));
      setClosed((current) => drop(current, row.path));
    } else {
      setClosed((current) => add(current, row.path));
      setOpen((current) => drop(current, row.path));
    }
  };

  const focusRow = (path: string) => {
    setFocused(path);
    requestAnimationFrame(() => document.querySelector<HTMLElement>(`[data-json-path="${CSS.escape(path)}"]`)?.focus());
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>, index: number) => {
    const row = rows[index] as Row;
    switch (event.key) {
      case 'ArrowDown':
        if (rows[index + 1]) focusRow((rows[index + 1] as Row).path);
        break;
      case 'ArrowUp':
        if (index > 0) focusRow((rows[index - 1] as Row).path);
        break;
      case 'ArrowRight':
        if (row.expandable && !row.expanded) toggle(row, true);
        else if (row.expanded && rows[index + 1]) focusRow((rows[index + 1] as Row).path);
        break;
      case 'ArrowLeft':
        if (row.expanded) toggle(row, false);
        else if (row.parent) focusRow(row.parent);
        break;
      case 'Home':
        focusRow((rows[0] as Row).path);
        break;
      case 'End':
        focusRow((rows[rows.length - 1] as Row).path);
        break;
      case 'Enter':
      case ' ':
        if (row.expandable) toggle(row, !row.expanded);
        break;
      default:
        return;
    }
    event.preventDefault();
  };

  const tabStop = rows.some((row) => row.path === focused) ? focused : '$';

  return (
    <div role="tree" aria-label={label ?? intl.formatMessage(editorMessages.jsonLabel)} data-json-tree className={cn('h-full text-mono overflow-auto p-3', className)}>
      {rows.map((row, index) => {
        const name = row.key ?? '';
        // The indent follows the depth, which only exists at run time.
        const indent: CSSProperties = { paddingInlineStart: `calc(var(--orcon-space-4) * ${row.depth})` };
        return (
          <div
            key={row.path}
            role="treeitem"
            aria-level={row.depth + 1}
            aria-setsize={row.setSize}
            aria-posinset={row.position}
            aria-expanded={row.expandable ? row.expanded : undefined}
            tabIndex={row.path === tabStop ? 0 : -1}
            data-json-path={row.path}
            onFocus={() => setFocused(row.path)}
            onKeyDown={(event) => onKeyDown(event, index)}
            // eslint-disable-next-line orcon/no-inline-style -- indent computed from the depth of the row
            style={indent}
            className="hover:bg-surface-sunken flex min-h-target items-center gap-1 rounded-sm"
          >
            {row.expandable ? (
              <button
                type="button"
                tabIndex={-1}
                aria-label={intl.formatMessage(row.expanded ? editorMessages.treeCollapse : editorMessages.treeExpand, { name: name || '$' })}
                onClick={() => toggle(row, !row.expanded)}
                className="text-fg-muted hover:text-fg grid size-icon-20 min-h-target min-w-target shrink-0 place-items-center"
              >
                {row.expanded ? <ChevronDown aria-hidden className="size-icon-16" /> : <ChevronRight aria-hidden className="size-icon-16" />}
              </button>
            ) : (
              <span aria-hidden className="size-icon-20 min-w-target shrink-0" />
            )}
            {row.key !== undefined ? <span className="text-fg">{row.key}</span> : null}
            {row.key !== undefined ? <span className="text-fg-muted">:</span> : null}
            {row.expandable ? (
              <span className="text-fg-muted">
                {Array.isArray(row.value) ? `[${row.expanded ? '' : `… ${row.size}`}]` : `{${row.expanded ? '' : `… ${row.size}`}}`}
              </span>
            ) : (
              <Primitive value={row.value} />
            )}
          </div>
        );
      })}
    </div>
  );
}
