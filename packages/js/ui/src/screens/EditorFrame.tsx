// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Play } from 'lucide-react';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useIntl } from 'react-intl';
import { toast } from 'sonner';

import { exampleCatalog, exampleDraft } from '../canvas/fixtures';
import type { CatalogItem, Graph } from '../canvas/model';
import { NodeStatusChip } from '../canvas/node-status-chip/NodeStatusChip';
import { useMediaQuery, wideQuery } from '../canvas/useMediaQuery';
import { applyOverlay, WorkflowCanvas, type ExecutionOverlay, type WorkflowCanvasHandle } from '../canvas/WorkflowCanvas';
import { Button } from '../components/button/Button';
import type { StatusKey } from '../components/status-badge/status-map';
import { cn } from '../lib/cn';
import { DataPane } from '../editor/data-pane/DataPane';
import { editorMessages } from '../editor/messages';
import type { DataView, PaneData } from '../editor/model';
import { NodeDetailsView, type DetailsTab } from '../editor/node-details-view/NodeDetailsView';
import { sampleMessages } from '../editor/sample/messages';
import { AgentSample, DocsSample, NodeStatusAlert, ParametersSample, SettingsSample } from '../editor/sample/ParametersSample';
import { useRunSimulation, type NodeOutputs } from '../editor/sample/useRunSimulation';

/** The desktop breakpoint (lg, 1024 px): from it the node details view opens as a dialog with three panes. */
const lgQuery = '(min-width: 64rem)';

const docks = ['input', 'output'] as const;

export interface EditorFrameProps {
  /** The flow to edit. */
  graph?: Graph;
  catalog?: readonly CatalogItem[];
  /** The state of the nodes and connections to start with, from an earlier run. */
  overlay?: ExecutionOverlay;
  /** Data that nodes already produced. */
  outputs?: NodeOutputs;
  readOnly?: boolean;
  /** Milliseconds a step takes in the sample runs. */
  runDelay?: number;
  /** Open the details view of this node on load. */
  defaultDetailsId?: string;
  executionHref?: (nodeId: string) => string;
  /** Called after every change of the flow, for tests and stories. */
  onGraphChange?: (graph: Graph) => void;
  className?: string;
}

/** The state of the last run, from the nodes shown: failed and uncertain win over running, which wins over succeeded. */
function lastRunStatus(graph: Graph): StatusKey | undefined {
  const states = graph.nodes.map((node) => node.status).filter((status): status is StatusKey => status !== undefined);
  if (states.length === 0) return undefined;
  for (const status of ['FAILED', 'UNCERTAIN', 'RUNNING', 'WAITING'] as const) if (states.includes(status)) return status;
  return states.includes('SUCCEEDED_WITH_ERRORS') ? 'SUCCEEDED_WITH_ERRORS' : 'SUCCEEDED';
}

/**
 * The flow editor in style A, without the app shell: the top bar (flow name, last run, Save and
 * Execute workflow), the canvas with its tools, and the node details view. From lg the details open
 * as a dialog with Input, Parameters and Output; between md and lg the Parameters pane docks at the
 * right and the data shows in a panel at the bottom. Runs are sample runs on timers: no request.
 */
export function EditorFrame({
  graph: initialGraph = exampleDraft,
  catalog = exampleCatalog,
  overlay: initialOverlay,
  outputs: initialOutputs,
  readOnly = false,
  runDelay = 700,
  defaultDetailsId,
  executionHref,
  onGraphChange,
  className,
}: EditorFrameProps) {
  const intl = useIntl();
  const wide = useMediaQuery(lgQuery);
  const medium = useMediaQuery(wideQuery);
  const canvas = useRef<WorkflowCanvasHandle>(null);
  const [graph, setGraph] = useState<Graph>(initialGraph);
  const graphRef = useRef(graph);
  graphRef.current = graph;
  const getGraph = useCallback(() => graphRef.current, []);
  const run = useRunSimulation(getGraph, initialOverlay, runDelay, initialOutputs);
  const [detailsId, setDetailsId] = useState<string | undefined>(defaultDetailsId);
  const [tab, setTab] = useState<DetailsTab>('parameters');
  const [inputView, setInputView] = useState<DataView>('table');
  const [outputView, setOutputView] = useState<DataView>('table');
  const [port, setPort] = useState<string | undefined>();
  const [dock, setDock] = useState<'input' | 'output'>('output');
  // Editing needs md or wider: on a phone the canvas is view only, so there is nothing to save or run.
  const editable = !readOnly && medium;

  const shown = useMemo(() => applyOverlay(graph, run.overlay), [graph, run.overlay]);
  // The details view needs md or wider; on a phone a node opens the status sheet of the canvas instead.
  const details = medium ? shown.nodes.find((node) => node.id === detailsId) : undefined;
  const trigger = shown.nodes.find((node) => node.kind === 'trigger');
  const last = lastRunStatus(shown);

  const onChange = useCallback(
    (next: Graph) => {
      setGraph(next);
      onGraphChange?.(next);
    },
    [onGraphChange],
  );

  const openDetails = (id: string, next?: 'agent') => {
    setDetailsId(id);
    setTab(next ?? 'parameters');
    setPort(undefined);
  };
  const closeDetails = () => setDetailsId(undefined);

  // What the node received: the items of the first connection into it whose source has run.
  const inputOf = (nodeId: string): PaneData | undefined => {
    const edge = shown.edges.find((candidate) => candidate.target === nodeId && run.outputs[candidate.source]);
    const items = edge ? run.outputs[edge.source]?.[edge.sourceOutput] : undefined;
    const source = edge ? shown.nodes.find((node) => node.id === edge.source) : undefined;
    return items && source ? { items, from: source.name, run: { index: 1, total: 1 } } : undefined;
  };
  const outputOf = (nodeId: string): { data?: PaneData; ports?: { id: string; label: string; count: number }[]; port?: string } => {
    const node = shown.nodes.find((candidate) => candidate.id === nodeId);
    const produced = run.outputs[nodeId];
    if (!node || !produced) return {};
    const ports = node.outputs.length > 1 ? node.outputs.map((candidate) => ({ id: candidate.id, label: candidate.label ?? candidate.id, count: produced[candidate.id]?.length ?? 0 })) : undefined;
    const active = port ?? node.outputs[0]?.id ?? 'main';
    return { data: { items: produced[active] ?? [], from: node.name, run: { index: 1, total: 1 }, pinned: node.pinned }, ports, port: active };
  };

  const detailsOutput = details ? outputOf(details.id) : {};
  const input = (
    <DataPane
      mode="input"
      data={details ? inputOf(details.id) : undefined}
      view={inputView}
      onViewChange={setInputView}
      onExecute={() => trigger && run.executeWorkflow(trigger.id)}
      className="flex-1"
    />
  );
  const output = (
    <DataPane
      mode="output"
      data={detailsOutput.data}
      ports={detailsOutput.ports}
      activePort={detailsOutput.port}
      onPortChange={setPort}
      view={outputView}
      onViewChange={setOutputView}
      onExecute={() => details && run.runStep(details.id)}
      onPinChange={(pinned) => details && canvas.current?.updateNode(details.id, { pinned })}
      onEdit={() => undefined}
      className="flex-1"
    />
  );

  const view = details ? (
    <NodeDetailsView
      node={details}
      open
      onOpenChange={(open) => !open && closeDetails()}
      layout={wide ? 'dialog' : 'docked'}
      input={details.kind === 'trigger' ? undefined : input}
      output={output}
      tabs={{
        parameters: <ParametersSample node={details} />,
        settings: <SettingsSample />,
        docs: <DocsSample />,
        agent: details.kind === 'agent' ? <AgentSample /> : undefined,
      }}
      alert={<NodeStatusAlert node={details} onAction={(action) => (action === 'run' ? run.runStep(details.id) : undefined)} />}
      tab={tab}
      onTabChange={setTab}
      onExecute={() => run.runStep(details.id)}
      onRename={(name) => canvas.current?.updateNode(details.id, { name })}
      onReturnFocus={() => canvas.current?.focusNode(details.id)}
    />
  ) : null;

  return (
    <div data-editor-frame className={cn('bg-surface border-border flex h-full min-h-0 flex-col overflow-hidden rounded-lg border', className)}>
      <header className="border-border flex flex-wrap items-center gap-x-3 gap-y-2 border-b px-4 py-2">
        <h1 className="me-auto min-w-0 truncate text-md font-semibold" data-truncate>
          {intl.formatMessage(sampleMessages.topbarFlow)}
        </h1>
        {last ? (
          <span data-last-run={last}>
            <NodeStatusChip status={last} />
          </span>
        ) : null}
        {editable ? (
          <>
            <Button variant="secondary" onClick={() => toast(intl.formatMessage(sampleMessages.topbarSaved))}>
              {intl.formatMessage(sampleMessages.topbarSave)}
            </Button>
            <Button disabled={!trigger} onClick={() => trigger && run.executeWorkflow(trigger.id)}>
              <Play aria-hidden className="size-icon-16" />
              {intl.formatMessage(sampleMessages.topbarExecute)}
            </Button>
          </>
        ) : null}
      </header>
      <div className="flex min-h-0 flex-1 flex-col">
        <div className="flex min-h-0 flex-1">
          <WorkflowCanvas
            ref={canvas}
            defaultGraph={initialGraph}
            catalog={editable ? catalog : []}
            overlay={run.overlay}
            readOnly={readOnly}
            tools
            tidyUp
            onGraphChange={onChange}
            onOpenDetails={openDetails}
            onRunStep={run.runStep}
            onExecuteWorkflow={run.executeWorkflow}
            executionHref={executionHref}
            className="min-w-0 flex-1"
          />
          {!wide ? view : null}
        </div>
        {!wide && details ? (
          <section aria-label={intl.formatMessage(editorMessages.output)} className="border-border flex h-panel-width shrink-0 flex-col border-t" data-docked-data>
            <div role="group" aria-label={intl.formatMessage(editorMessages.viewLabel)} className="border-border flex gap-1 border-b px-3 py-1">
              {docks.map((mode) => (
                <button
                  key={mode}
                  type="button"
                  aria-pressed={dock === mode}
                  onClick={() => setDock(mode)}
                  className="text-fg hover:bg-surface-sunken aria-pressed:bg-surface-sunken min-h-target rounded-md px-3 text-sm aria-pressed:font-semibold"
                >
                  {intl.formatMessage(mode === 'input' ? editorMessages.input : editorMessages.output)}
                </button>
              ))}
            </div>
            {dock === 'input' ? input : output}
          </section>
        ) : null}
      </div>
      {wide ? view : null}
    </div>
  );
}
