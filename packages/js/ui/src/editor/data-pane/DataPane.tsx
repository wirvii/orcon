// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { CircleAlert, Copy, Pencil, Pin, PinOff } from 'lucide-react';
import { useEffect, useId, useState, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { Banner } from '../../components/banner/Banner';
import { Button } from '../../components/button/Button';
import { IconButton } from '../../components/icon-button/IconButton';
import { Skeleton } from '../../components/skeleton/Skeleton';
import { cn } from '../../lib/cn';
import { DataTableView } from '../data-table-view/DataTableView';
import { JsonTreeView } from '../json-tree-view/JsonTreeView';
import { editorMessages } from '../messages';
import { visibleValue, type DataView, type PaneData } from '../model';
import { SchemaView } from '../schema-view/SchemaView';

export interface OutputPort {
  id: string;
  label: string;
  count: number;
}

export interface DataPaneProps {
  /** Input shows what the node received; output what it produced (and can be pinned). */
  mode: 'input' | 'output';
  /** The data to show. Absent means nothing has run: the pane shows its empty state. */
  data?: PaneData;
  /** `loading` shows placeholders; `error` shows the message. */
  status?: 'data' | 'loading' | 'error';
  error?: string;
  view?: DataView;
  defaultView?: DataView;
  onViewChange?: (view: DataView) => void;
  /** The outputs of a node with several, as tabs with counts. */
  ports?: readonly OutputPort[];
  activePort?: string;
  onPortChange?: (id: string) => void;
  /** The action of the empty state: run the previous nodes (input) or this step (output). */
  onExecute?: () => void;
  onPinChange?: (pinned: boolean) => void;
  onEdit?: () => void;
  headingLevel?: 2 | 3 | 4;
  className?: string;
}

const views: { id: DataView; message: typeof editorMessages.viewTable }[] = [
  { id: 'table', message: editorMessages.viewTable },
  { id: 'json', message: editorMessages.viewJson },
  { id: 'schema', message: editorMessages.viewSchema },
];

/**
 * A pane of the node details view or the docked bottom panel: a header (title, where the data comes
 * from, which run, the outputs, a Table / JSON / Schema switch, the item count and, for the output,
 * Pin and Edit) and a body that shows the data in the chosen view, or why there is none. Pinned output
 * says so. Protected values stay protected in every view, and copying never copies them.
 */
export function DataPane({
  mode,
  data,
  status = 'data',
  error,
  view: viewProp,
  defaultView = 'table',
  onViewChange,
  ports,
  activePort,
  onPortChange,
  onExecute,
  onPinChange,
  onEdit,
  headingLevel = 3,
  className,
}: DataPaneProps) {
  const intl = useIntl();
  const headingId = useId();
  const [localView, setLocalView] = useState<DataView>(defaultView);
  const [copied, setCopied] = useState(false);
  const view = viewProp ?? localView;
  const Heading = `h${headingLevel}` as const;
  const input = mode === 'input';
  const title = intl.formatMessage(input ? editorMessages.input : editorMessages.output);
  const items = data?.items ?? [];

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 2000);
    return () => window.clearTimeout(timer);
  }, [copied]);

  const setView = (next: DataView) => {
    setLocalView(next);
    onViewChange?.(next);
  };
  const copy = () => {
    // Only what is visible is copied: protected values become a fixed notice.
    void navigator.clipboard?.writeText(JSON.stringify(items.map((item) => visibleValue(item)), null, 2)).then(() => setCopied(true), () => undefined);
  };

  let body: ReactNode;
  if (status === 'loading') {
    body = (
      <div aria-busy="true" className="flex flex-col gap-2 p-3">
        <Skeleton shape="text" className="w-full" />
        <Skeleton shape="text" className="w-full" />
        <Skeleton shape="text" className="w-1/2" />
      </div>
    );
  } else if (status === 'error') {
    body = (
      <div className="p-3">
        <Banner tone="danger" icon={CircleAlert} title={error ?? ''} />
      </div>
    );
  } else if (!data) {
    body = (
      <div className="flex flex-col items-start gap-3 p-4" data-data-empty={mode}>
        <p className="text-fg-muted text-base">{intl.formatMessage(input ? editorMessages.inputEmpty : editorMessages.outputEmpty)}</p>
        {onExecute ? (
          <Button variant="secondary" onClick={onExecute}>
            {intl.formatMessage(input ? editorMessages.inputEmptyAction : editorMessages.outputEmptyAction)}
          </Button>
        ) : null}
      </div>
    );
  } else {
    body = (
      <div className="flex min-h-0 flex-1 flex-col">
        {data.pinned ? (
          <div className="px-3 pt-3">
            <Banner
              tone="info"
              icon={Pin}
              title={intl.formatMessage(editorMessages.pinnedBanner)}
              action={
                onPinChange ? (
                  <Button variant="secondary" onClick={() => onPinChange(false)}>
                    {intl.formatMessage(editorMessages.unpin)}
                  </Button>
                ) : undefined
              }
            />
          </div>
        ) : null}
        <div className="min-h-0 flex-1 overflow-hidden">
          {view === 'table' ? <DataTableView items={items} /> : view === 'json' ? <JsonTreeView value={items} /> : <SchemaView items={items} />}
        </div>
      </div>
    );
  }

  return (
    <section aria-labelledby={headingId} data-data-pane={mode} className={cn('bg-surface text-fg flex min-h-0 min-w-0 flex-col', className)}>
      <header className="border-border flex flex-wrap items-center gap-x-3 gap-y-2 border-b px-3 py-2">
        <Heading id={headingId} className="text-base font-semibold">
          {title}
        </Heading>
        {data?.from ? <span className="text-fg-muted text-sm">{intl.formatMessage(editorMessages.fromNode, { name: data.from })}</span> : null}
        {data?.run ? <span className="text-fg-muted text-sm">{intl.formatMessage(editorMessages.runOf, { index: data.run.index, total: data.run.total })}</span> : null}
        {ports && ports.length > 1 ? (
          <div role="group" aria-label={intl.formatMessage(editorMessages.portTabs)} className="flex flex-wrap gap-1">
            {ports.map((port) => (
              <button
                key={port.id}
                type="button"
                aria-pressed={(activePort ?? ports[0]?.id) === port.id}
                onClick={() => onPortChange?.(port.id)}
                className="border-control text-fg hover:bg-surface-sunken aria-pressed:bg-surface-sunken aria-pressed:font-semibold min-h-target rounded-md border px-2 text-sm"
              >
                {intl.formatMessage(editorMessages.portTab, { name: port.label, count: port.count })}
              </button>
            ))}
          </div>
        ) : null}
        <div className="ms-auto flex flex-wrap items-center gap-2">
          {data ? <span className="text-fg-muted text-sm">{intl.formatMessage(editorMessages.items, { count: items.length })}</span> : null}
          <div role="group" aria-label={intl.formatMessage(editorMessages.viewLabel)} className="border-control inline-flex overflow-hidden rounded-md border">
            {views.map(({ id, message }) => (
              <button
                key={id}
                type="button"
                aria-pressed={view === id}
                onClick={() => setView(id)}
                className="text-fg bg-surface hover:bg-surface-sunken aria-pressed:bg-fg aria-pressed:text-app min-h-target px-3 text-sm aria-pressed:font-semibold"
              >
                {intl.formatMessage(message)}
              </button>
            ))}
          </div>
          {data ? (
            <IconButton variant="ghost" icon={Copy} label={intl.formatMessage(editorMessages.copy)} onClick={copy} />
          ) : null}
          {!input && data && onPinChange ? (
            <Button variant="secondary" onClick={() => onPinChange(!data.pinned)}>
              {data.pinned ? <PinOff aria-hidden className="size-icon-16" /> : <Pin aria-hidden className="size-icon-16" />}
              {intl.formatMessage(data.pinned ? editorMessages.unpin : editorMessages.pin)}
            </Button>
          ) : null}
          {!input && data && onEdit ? (
            <IconButton variant="secondary" icon={Pencil} label={intl.formatMessage(editorMessages.edit)} onClick={onEdit} />
          ) : null}
        </div>
      </header>
      {copied ? (
        <span role="status" className="sr-only">
          {intl.formatMessage(editorMessages.copied)}
        </span>
      ) : null}
      {body}
    </section>
  );
}
