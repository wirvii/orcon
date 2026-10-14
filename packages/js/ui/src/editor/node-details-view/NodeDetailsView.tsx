// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { ArrowLeft, Play } from 'lucide-react';
import { Dialog as DialogPrimitive } from 'radix-ui';
import { useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { Banner } from '../../components/banner/Banner';
import { Button } from '../../components/button/Button';
import { IconButton } from '../../components/icon-button/IconButton';
import { Tabs } from '../../components/tabs/Tabs';
import { NodeGlyph } from '../../canvas/NodeGlyph';
import { NodeStatusChip } from '../../canvas/node-status-chip/NodeStatusChip';
import type { GraphNode } from '../../canvas/model';
import { cn } from '../../lib/cn';
import { InsideModal, useScrim } from '../../lib/modal-nesting';
import { PaneSplitter } from '../pane-splitter/PaneSplitter';
import { editorMessages } from '../messages';

export type DetailsTab = 'parameters' | 'settings' | 'docs' | 'agent';

export interface NodeDetailsViewProps {
  node: GraphNode;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /**
   * `dialog` (lg and wider): three panes over the canvas. `docked` (md to lg): only the parameters pane, docked at the right of the canvas.
   * `fullscreen` (below md, phones): the parameters pane alone, filling the screen.
   */
  layout?: 'dialog' | 'docked' | 'fullscreen' | undefined;
  /** The Input pane. A trigger has none. */
  input?: ReactNode | undefined;
  /** The Output pane. */
  output?: ReactNode | undefined;
  /** The content of each tab of the parameters pane. The Agent tab is for AI nodes. */
  tabs: Partial<Record<DetailsTab, ReactNode>> & { parameters: ReactNode };
  /** A status alert that heads the form when the last run left the node failed or uncertain. */
  alert?: ReactNode | undefined;
  tab?: DetailsTab | undefined;
  onTabChange?: ((tab: DetailsTab) => void) | undefined;
  /** Runs the step. */
  onExecute?: (() => void) | undefined;
  onRename?: ((name: string) => void) | undefined;
  /** Moves focus back to the node tile on the canvas when the view closes. */
  onReturnFocus?: (() => void) | undefined;
  /**
   * Shows every value and no control that writes: the tab content is disabled, and the rename and
   * execute actions are not offered. A notice above the tabs says why.
   */
  readOnly?: boolean | undefined;
  className?: string | undefined;
}

const tabMessages = {
  parameters: editorMessages.parameters,
  settings: editorMessages.settings,
  docs: editorMessages.docs,
  agent: editorMessages.agent,
} as const;

/** The name of the node: text that turns into a field when pressed, so it can be renamed in place. */
function NodeName({ name, onRename }: { name: string; onRename?: ((name: string) => void) | undefined }) {
  const intl = useIntl();
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(name);
  const finish = (commit: boolean) => {
    setEditing(false);
    if (commit && draft.trim() !== '' && draft.trim() !== name) onRename?.(draft.trim());
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    event.stopPropagation();
    if (event.key === 'Enter') finish(true);
    else if (event.key === 'Escape') {
      event.preventDefault();
      finish(false);
    }
  };
  if (editing) {
    return (
      <input
        autoFocus
        value={draft}
        aria-label={intl.formatMessage(editorMessages.renameNode)}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={() => finish(true)}
        onKeyDown={onKeyDown}
        className="bg-surface border-control text-fg min-h-target w-full rounded-md border px-2 text-md font-semibold outline-hidden"
      />
    );
  }
  return (
    <button
      type="button"
      disabled={!onRename}
      onClick={() => {
        setDraft(name);
        setEditing(true);
      }}
      className="text-fg min-h-target max-w-full truncate rounded-md px-1 text-left text-md font-semibold hover:bg-surface-sunken disabled:hover:bg-transparent"
    >
      {name}
    </button>
  );
}

function Header({ node, onBack, onExecute, onRename, compact = false }: { node: GraphNode; onBack: () => void; onExecute?: (() => void) | undefined; onRename?: ((name: string) => void) | undefined; compact?: boolean }) {
  const intl = useIntl();
  const back = intl.formatMessage(editorMessages.backToCanvas);
  const execute = onExecute ? (
    <Button onClick={onExecute} className={compact ? undefined : 'ms-auto'}>
      <Play aria-hidden className="size-icon-16" />
      {intl.formatMessage(editorMessages.executeStep)}
    </Button>
  ) : null;
  const chips = (
    <>
      {node.status ? <NodeStatusChip status={node.status} code={node.statusCode} /> : null}
      {node.pinned ? <NodeStatusChip kind="pinned" /> : null}
    </>
  );
  const glyph = (
    <span className="bg-surface-sunken border-border grid size-canvas-icon shrink-0 place-items-center rounded-lg border" aria-hidden>
      <NodeGlyph node={node} iconClassName="size-icon-24" className="size-icon-24" />
    </span>
  );
  const name = (
    <div className="flex min-w-canvas-node-wide flex-1 flex-col">
      <NodeName name={node.name} onRename={onRename} />
      {node.subtitle ? (
        <span className="text-fg-muted truncate px-1 text-xs" data-truncate>
          {node.subtitle}
        </span>
      ) : null}
    </div>
  );
  if (compact) {
    // The docked panel is narrow: the back arrow, the node and its name on one row, the state and the action under them.
    return (
      <header className="border-border flex shrink-0 flex-col gap-2 border-b px-3 py-2">
        <div className="flex items-center gap-2">
          <IconButton variant="ghost" icon={ArrowLeft} label={back} onClick={onBack} />
          {glyph}
          {name}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {chips}
          <span className="ms-auto">{execute}</span>
        </div>
      </header>
    );
  }
  return (
    <header className="border-border flex min-h-target shrink-0 flex-wrap items-center gap-x-3 gap-y-2 border-b px-4 py-2">
      <Button variant="ghost" onClick={onBack}>
        <ArrowLeft aria-hidden className="size-icon-16" />
        {back}
      </Button>
      {glyph}
      {name}
      {chips}
      {execute}
    </header>
  );
}

/**
 * The node details view. From lg it opens over the canvas as a modal dialog with three panes: Input,
 * Parameters and Output; between md and lg only the Parameters pane is shown, docked at the right of
 * the canvas, and the editor shows the data in a panel at the bottom. The panes are separated by
 * handles that resize with the pointer and with the arrow keys. A trigger has no Input pane.
 */
export function NodeDetailsView({ node, open, onOpenChange, layout = 'dialog', input, output, tabs, alert, tab: tabProp, onTabChange, onExecute, onRename, onReturnFocus, readOnly = false, className }: NodeDetailsViewProps) {
  const intl = useIntl();
  const backdrop = useScrim();
  // A read-only view offers no action that writes or runs anything.
  const executeAction = readOnly ? undefined : onExecute;
  const renameAction = readOnly ? undefined : onRename;
  const [localTab, setLocalTab] = useState<DetailsTab>('parameters');
  const tab = tabProp ?? localTab;
  const frame = useRef<HTMLDivElement>(null);
  const middle = useRef<HTMLDivElement>(null);
  const [paramsPx, setParamsPx] = useState<number | undefined>();
  const [measured, setMeasured] = useState({ params: 420, total: 1200 });
  const trigger = node.kind === 'trigger';
  const minParams = 420;
  const maxParams = 640;
  const paneMin = 260;

  useLayoutEffect(() => {
    if (layout !== 'dialog' || !open) return;
    const measure = () => {
      const params = middle.current?.getBoundingClientRect().width;
      const total = frame.current?.getBoundingClientRect().width;
      if (params && total) setMeasured({ params, total });
    };
    measure();
    const observer = new ResizeObserver(measure);
    if (frame.current) observer.observe(frame.current);
    return () => observer.disconnect();
  }, [layout, open, paramsPx]);

  const panes = trigger ? 1 : 2;
  const room = Math.max(minParams, measured.total - panes * paneMin);
  const limit = Math.min(maxParams, room);
  const paramsValue = Math.round(Math.min(limit, Math.max(minParams, paramsPx ?? measured.params)));
  const switchTab = (next: string) => {
    setLocalTab(next as DetailsTab);
    onTabChange?.(next as DetailsTab);
  };

  const tabItems = (['parameters', 'settings', 'docs', ...(node.kind === 'agent' ? (['agent'] as const) : [])] as DetailsTab[])
    .filter((id) => tabs[id] !== undefined)
    .map((id) => {
      const body = (
        <>
          {id === 'parameters' ? alert : null}
          {tabs[id]}
        </>
      );
      return {
        value: id,
        label: intl.formatMessage(tabMessages[id]),
        // A disabled fieldset disables every control inside it, so a read-only view cannot be edited by any of them.
        content: readOnly ? (
          <fieldset disabled className="m-0 flex min-w-0 flex-col gap-3 border-0 p-0 pt-4">
            {body}
          </fieldset>
        ) : (
          <div className="flex flex-col gap-3 pt-4">{body}</div>
        ),
      };
    });
  const parameters = (
    <>
      {readOnly ? (
        <div className="shrink-0 px-4 pt-3">
          <Banner tone="info" title={intl.formatMessage(editorMessages.readOnlyTitle)}>
            {intl.formatMessage(editorMessages.readOnlyText)}
          </Banner>
        </div>
      ) : null}
      <Tabs tabs={tabItems} label={intl.formatMessage(editorMessages.tabsLabel)} value={tab} onValueChange={switchTab} className="min-h-0 flex-1 overflow-auto px-4 pt-2 pb-4" />
    </>
  );

  if (layout === 'docked') {
    if (!open) return null;
    return (
      <aside
        aria-label={intl.formatMessage(editorMessages.detailsTitle, { name: node.name })}
        data-node-details="docked"
        className={cn('bg-surface border-border flex min-h-0 w-editor-docked-panel max-w-full shrink-0 flex-col border-s', className)}
      >
        <Header node={node} compact onBack={() => { onOpenChange(false); onReturnFocus?.(); }} onExecute={executeAction} onRename={renameAction} />
        {parameters}
      </aside>
    );
  }

  if (layout === 'fullscreen') {
    return (
      <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
        <DialogPrimitive.Portal>
          <InsideModal.Provider value>
          <DialogPrimitive.Content
            aria-describedby={undefined}
            data-node-details="fullscreen"
            onCloseAutoFocus={(event) => {
              if (onReturnFocus) {
                event.preventDefault();
                onReturnFocus();
              }
            }}
            className={cn('bg-surface text-fg fixed inset-0 z-modal flex min-w-0 flex-col overflow-hidden', className)}
          >
            <DialogPrimitive.Title className="sr-only">{intl.formatMessage(editorMessages.detailsTitle, { name: node.name })}</DialogPrimitive.Title>
            <Header node={node} compact onBack={() => onOpenChange(false)} onExecute={executeAction} onRename={renameAction} />
            {parameters}
          </DialogPrimitive.Content>
          </InsideModal.Provider>
        </DialogPrimitive.Portal>
      </DialogPrimitive.Root>
    );
  }

  const splitter = (label: string, direction: 1 | -1) => (
    <PaneSplitter label={label} value={paramsValue} min={minParams} max={limit} onChange={setParamsPx} direction={direction} />
  );
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className={backdrop} />
        <InsideModal.Provider value>
        <DialogPrimitive.Content
          aria-describedby={undefined}
          data-node-details="dialog"
          onCloseAutoFocus={(event) => {
            if (onReturnFocus) {
              event.preventDefault();
              onReturnFocus();
            }
          }}
          className={cn('bg-surface text-fg border-border fixed inset-6 z-modal flex flex-col overflow-hidden rounded-lg border shadow-lg', className)}
        >
          <DialogPrimitive.Title className="sr-only">{intl.formatMessage(editorMessages.detailsTitle, { name: node.name })}</DialogPrimitive.Title>
          <Header node={node} onBack={() => onOpenChange(false)} onExecute={executeAction} onRename={renameAction} />
          <div ref={frame} className="flex min-h-0 flex-1">
            {!trigger ? (
              <>
                <div className="flex min-h-0 min-w-editor-pane-min flex-1 flex-col" data-pane="input">
                  {input}
                </div>
                {splitter(intl.formatMessage(editorMessages.splitterInput), -1)}
              </>
            ) : null}
            <div
              ref={middle}
              data-pane="parameters"
              // The parameters pane is 34 % of the view, between 420 and 640 px, until a handle sets it.
              // eslint-disable-next-line orcon/no-inline-style -- pane width follows the view and the handles
              style={{ width: paramsPx === undefined ? 'clamp(var(--orcon-editor-params-min), 34%, var(--orcon-editor-params-max))' : paramsValue }}
              className="flex min-h-0 shrink-0 flex-col"
            >
              {parameters}
            </div>
            {splitter(intl.formatMessage(editorMessages.splitterOutput), 1)}
            <div className="flex min-h-0 min-w-editor-pane-min flex-1 flex-col" data-pane="output">
              {output}
            </div>
          </div>
        </DialogPrimitive.Content>
        </InsideModal.Provider>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
