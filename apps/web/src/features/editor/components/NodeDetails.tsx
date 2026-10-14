// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { DataPane, NodeDetailsView, type DetailsTab, type GraphNode } from '@orcon/ui';
import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { useIntl } from 'react-intl';

import { ConnectorUpdate } from '../forms/node-config/ConnectorUpdate';
import { NodeConfigForm } from '../forms/node-config/NodeConfigForm';
import { NodeDocs } from '../forms/node-config/NodeDocs';
import { NodeSettingsForm } from '../forms/node-config/NodeSettingsForm';
import { canvasNodeView } from '../model/canvas-graph';
import { manifestOf } from '../model/node-manifests';
import { useEditorStore, useEditorStoreApi } from '../store/editor-context';

import { editorMessages } from './messages';
import { useNoticeChannel } from './notice-channel';
import { useDescribeNode } from './use-node-catalog';

export interface NodeDetailsProps {
  nodeId: string;
  /** `dialog`: three panes over the canvas (lg and wider). `docked`: the Parameters pane beside the canvas (md to lg). */
  layout: 'dialog' | 'docked';
  tab: DetailsTab;
  onTabChange: (tab: DetailsTab) => void;
  onClose: () => void;
  /** Moves keyboard focus back to the node on the canvas. */
  onReturnFocus: () => void;
}

/** The node as the design system's view draws it: the same name, line and ports as its tile on the canvas. */
function useViewNode(nodeId: string): GraphNode | undefined {
  const node = useEditorStore((state) => state.document.nodes.find((candidate) => candidate.id === nodeId));
  const edges = useEditorStore((state) => state.document.edges);
  const describe = useDescribeNode();
  return useMemo(() => (node ? canvasNodeView(node, edges, describe) : undefined), [node, edges, describe]);
}

/** The two data panes of a node. Nothing has run in this editor yet, so both show their empty state and offer the run. */
function usePanes(node: GraphNode | undefined, className: string): { input: ReactNode; output: ReactNode; run: () => void } {
  const intl = useIntl();
  const notices = useNoticeChannel();
  const run = () => {
    notices.notify('warning', intl.formatMessage(editorMessages.noticeLaterRun));
  };
  const trigger = node?.kind === 'trigger';
  return {
    run,
    input: trigger ? undefined : <DataPane mode="input" onExecute={run} className={className} />,
    output: <DataPane mode="output" onExecute={run} onPinChange={() => { notices.notify('warning', intl.formatMessage(editorMessages.noticeLaterPin)); }} className={className} />,
  };
}

/**
 * The node details view of the editor (SPEC-014 R-ED-CFG-01, SPEC-007 `canvas.md#node-details-view`):
 * the design system's `NodeDetailsView` wired to the store. From lg it is a dialog over the canvas
 * with Input, Parameters and Output; between md and lg only the Parameters pane shows, docked at the
 * right of the canvas, and `DockedDataPanel` shows the data at the bottom. The tabs are Parameters,
 * Settings and Docs; every edit is a store action, so it is one step of the undo history.
 */
export function NodeDetails({ nodeId, layout, tab, onTabChange, onClose, onReturnFocus }: NodeDetailsProps) {
  const intl = useIntl();
  const api = useEditorStoreApi();
  const notices = useNoticeChannel();
  const view = useViewNode(nodeId);
  const node = useEditorStore((state) => state.document.nodes.find((candidate) => candidate.id === nodeId));
  const panes = usePanes(view, 'flex-1');

  useEffect(() => {
    if (!node) onClose();
  }, [node, onClose]);
  if (!node || !view) return null;

  const manifest = manifestOf(node);
  const rename = (name: string) => {
    const result = api.getState().renameNode(nodeId, name);
    if ('problem' in result) {
      if (result.problem !== 'missing') notices.notify('warning', intl.formatMessage(result.problem === 'empty' ? editorMessages.noticeNameEmpty : editorMessages.noticeNameTaken));
      return;
    }
    notices.notify('info', intl.formatMessage(editorMessages.noticeRenamed, { name, count: result.references }));
  };

  return (
    <NodeDetailsView
      node={view}
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      layout={layout}
      input={panes.input}
      output={panes.output}
      tab={tab}
      onTabChange={onTabChange}
      tabs={{
        parameters: (
          <>
            <ConnectorUpdate node={node} />
            <NodeConfigForm nodeId={nodeId} />
          </>
        ),
        settings: <NodeSettingsForm node={node} manifest={manifest} />,
        docs: <NodeDocs node={node} manifest={manifest} />,
      }}
      onExecute={panes.run}
      onRename={rename}
      onReturnFocus={onReturnFocus}
    />
  );
}

/**
 * Between md and lg the data of the open node shows in a panel at the bottom of the editor, one side
 * at a time (SPEC-014 R-ED-DATA-01). A trigger has no input, so it shows only its output.
 */
export function DockedDataPanel({ nodeId }: { nodeId: string }) {
  const intl = useIntl();
  const view = useViewNode(nodeId);
  const panes = usePanes(view, 'flex-1');
  const [side, setSide] = useState<'input' | 'output'>('output');
  if (!view) return null;
  const sides = view.kind === 'trigger' ? (['output'] as const) : (['input', 'output'] as const);
  const shown = view.kind === 'trigger' ? 'output' : side;
  return (
    <section aria-label={intl.formatMessage(editorMessages.dockedData, { name: view.name })} className="border-border flex h-panel-width shrink-0 flex-col border-t" data-docked-data>
      {sides.length > 1 ? (
        <div role="group" aria-label={intl.formatMessage(editorMessages.dockedSwitch)} className="border-border flex gap-1 border-b px-3 py-1">
          {sides.map((option) => (
            <button
              key={option}
              type="button"
              aria-pressed={shown === option}
              onClick={() => {
                setSide(option);
              }}
              className="text-fg hover:bg-surface-sunken aria-pressed:bg-surface-sunken min-h-target rounded-md px-3 text-sm aria-pressed:font-semibold"
            >
              {intl.formatMessage(option === 'input' ? editorMessages.dockedInput : editorMessages.dockedOutput)}
            </button>
          ))}
        </div>
      ) : null}
      {shown === 'input' ? panes.input : panes.output}
    </section>
  );
}
