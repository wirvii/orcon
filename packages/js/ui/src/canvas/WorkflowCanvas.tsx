// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Background,
  BackgroundVariant,
  getNodesBounds,
  getViewportForBounds,
  MiniMap,
  ReactFlow,
  ReactFlowProvider,
  SelectionMode,
  useNodesInitialized,
  useNodesState,
  useReactFlow,
  type Connection,
} from '@xyflow/react';
import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { useIntl, type IntlShape } from 'react-intl';
import { toast } from 'sonner';

import { AlertDialog } from '../components/alert-dialog/AlertDialog';
import { EmptyState } from '../components/empty-state/EmptyState';
import { statusMap, type StatusKey } from '../components/status-badge/status-map';
import { cn } from '../lib/cn';

import { useAnnouncer } from './announcer';
import { ConnectDialog } from './ConnectDialog';
import { CanvasOutline } from './CanvasOutline';
import { CanvasToolbar } from './CanvasToolbar';
import { OrconEdge, type OrconFlowEdge } from './Edge';
import { nodeTypes, type OrconFlowNode } from './FlowNodes';
import { canvasMessages } from './messages';
import {
  COARSE_STEPS,
  GRID,
  addNode,
  connect,
  connectError,
  moveNodes,
  nextFocus,
  pushHistory,
  readingOrder,
  redoHistory,
  removeEdge,
  removeNodes,
  setPositions,
  snap,
  startHistory,
  undoHistory,
  type CatalogItem,
  type ConnectRequest,
  type Direction,
  type Graph,
  type History,
} from './model';
import { NodeDetails } from './NodeDetails';
import { NodePalette } from './NodePalette';
import { nodeAriaLabel } from './nodeLabel';
import { ShortcutsDialog } from './ShortcutsDialog';
import { useMediaQuery, wideQuery } from './useMediaQuery';

const edgeTypes = { orcon: OrconEdge };

/** Below these zoom levels node text stops being readable, so fitting never goes lower: the view is anchored to the first node instead. */
const READABLE_ZOOM_WIDE = 0.75;
const READABLE_ZOOM_NARROW = 0.6;
const FIT_PADDING = 0.2;
const ANCHOR_MARGIN = 24;

const arrowDirection: Record<string, Direction> = { ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'up', ArrowDown: 'down' };
const arrowStep: Record<Direction, { x: number; y: number }> = {
  left: { x: -1, y: 0 },
  right: { x: 1, y: 0 },
  up: { x: 0, y: -1 },
  down: { x: 0, y: 1 },
};

export interface NodeExecutionState {
  status: StatusKey;
  uncertainCount?: number;
}

export interface WorkflowCanvasProps {
  /** The flow to start with. The canvas keeps its own copy and reports every change. */
  defaultGraph: Graph;
  /** Nodes that can be added. Without it the add button and the A shortcut are not offered. */
  catalog?: readonly CatalogItem[];
  /** Execution overlay: the live state per node id. Changes are announced politely, at most every five seconds. */
  statuses?: Readonly<Record<string, NodeExecutionState>>;
  onGraphChange?: (graph: Graph) => void;
  /** Enter on a node. Without it the node details open in a panel. */
  onInspect?: (nodeId: string) => void;
  /** Link to the execution detail of a node, shown in the node details. */
  executionHref?: (nodeId: string) => string;
  /** Force view-only mode. Below the md breakpoint the canvas is always view-only. */
  readOnly?: boolean;
  defaultOutlineOpen?: boolean;
  className?: string;
}

function applyStatuses(graph: Graph, statuses: WorkflowCanvasProps['statuses']): Graph {
  if (!statuses) return graph;
  return {
    ...graph,
    nodes: graph.nodes.map((node) => {
      const state = statuses[node.id];
      return state ? { ...node, status: state.status, uncertainCount: state.uncertainCount ?? node.uncertainCount } : node;
    }),
  };
}

function toFlowEdges(intl: IntlShape, graph: Graph): OrconFlowEdge[] {
  const byId = new Map(graph.nodes.map((node) => [node.id, node]));
  const portName = (ports: readonly { id: string; label?: string }[] | undefined, id: string) => ports?.find((port) => port.id === id)?.label ?? id;
  return graph.edges.map((edge) => {
    const source = byId.get(edge.source);
    const target = byId.get(edge.target);
    return {
      id: edge.id,
      type: 'orcon',
      source: edge.source,
      target: edge.target,
      sourceHandle: edge.sourceOutput,
      targetHandle: edge.targetInput,
      focusable: false,
      ariaLabel: intl.formatMessage(canvasMessages.edgeLabel, {
        source: source?.name ?? edge.source,
        output: portName(source?.outputs, edge.sourceOutput),
        target: target?.name ?? edge.target,
        input: portName(target?.inputs, edge.targetInput),
        hasItems: typeof edge.items === 'number' ? 'yes' : 'no',
        items: edge.items ?? 0,
        loop: edge.loopBack ? 'yes' : 'no',
      }),
      data: { items: edge.items, loopBack: edge.loopBack, active: edge.active },
    };
  });
}

interface SyncOptions {
  intl: IntlShape;
  editable: boolean;
  activeId: string | undefined;
  /** When set, exactly these nodes are selected after this sync. */
  select: ReadonlySet<string> | undefined;
}

/** Builds the React Flow nodes from the graph, keeping what React Flow measured and what is selected. */
function syncNodes(previous: OrconFlowNode[], graph: Graph, { intl, editable, activeId, select }: SyncOptions): OrconFlowNode[] {
  const before = new Map(previous.map((node) => [node.id, node]));
  return graph.nodes.map((node) => {
    const old = before.get(node.id);
    return {
      ...old,
      id: node.id,
      type: node.kind === 'control' ? 'control' : 'action',
      position: old?.dragging ? old.position : node.position,
      data: { node, connectable: editable },
      draggable: editable,
      selectable: true,
      connectable: editable,
      deletable: false,
      ariaLabel: nodeAriaLabel(intl, node),
      ariaRole: 'group',
      className: node.kind === 'control' ? 'rounded-full' : 'rounded-lg',
      // One node is in the tab order; the arrow keys move between nodes (a roving tab stop).
      domAttributes: { tabIndex: node.id === activeId ? 0 : -1, 'aria-roledescription': intl.formatMessage(canvasMessages.nodeRoleDescription) },
      selected: select ? select.has(node.id) : (old?.selected ?? false),
    } satisfies OrconFlowNode;
  });
}

function Canvas({
  defaultGraph,
  catalog = [],
  statuses,
  onGraphChange,
  onInspect,
  executionHref,
  readOnly = false,
  defaultOutlineOpen = false,
  className,
}: WorkflowCanvasProps) {
  const intl = useIntl();
  const wide = useMediaQuery(wideQuery);
  const editable = wide && !readOnly;
  const flow = useReactFlow();
  const { announce, region } = useAnnouncer();

  const [history, setHistory] = useState<History<Graph>>(() => startHistory(defaultGraph));
  const historyRef = useRef(history);
  const base = history.present;
  const graph = useMemo(() => applyStatuses(base, statuses), [base, statuses]);

  const [activeId, setActiveId] = useState<string | undefined>(() => readingOrder(defaultGraph)[0]?.id);
  const firstNodes = useMemo(
    () => syncNodes([], graph, { intl, editable, activeId, select: undefined }),
    // Only the first render needs this: later changes go through the effect below.
    [],
  );
  const [nodes, setNodes, onNodesChange] = useNodesState<OrconFlowNode>(firstNodes);
  const selectNext = useRef<ReadonlySet<string> | undefined>(undefined);
  const containerRef = useRef<HTMLDivElement>(null);

  /**
   * Shows the whole flow when that keeps the text readable. When the flow is too
   * wide for that, it keeps the readable zoom and starts at the first node (the
   * trigger), so the start of the flow is always on screen; pan to see the rest.
   */
  const fitFlow = useCallback(() => {
    const rect = containerRef.current?.getBoundingClientRect();
    const all = flow.getNodes();
    if (!rect || rect.width === 0 || all.length === 0) return;
    const readable = wide ? READABLE_ZOOM_WIDE : READABLE_ZOOM_NARROW;
    const bounds = getNodesBounds(all);
    const fitted = getViewportForBounds(bounds, rect.width, rect.height, 0.05, 2, FIT_PADDING);
    if (fitted.zoom >= readable) {
      void flow.setViewport(fitted);
      return;
    }
    const first = all.find((node) => node.id === readingOrder(graphRef.current)[0]?.id) ?? all[0]!;
    void flow.setViewport({
      x: ANCHOR_MARGIN - first.position.x * readable,
      y: rect.height / 2 - (bounds.y + bounds.height / 2) * readable,
      zoom: readable,
    });
  }, [flow, wide]);
  const graphRef = useRef(graph);
  graphRef.current = graph;
  const nodesReady = useNodesInitialized();
  const fittedOnOpen = useRef(false);
  useEffect(() => {
    if (!nodesReady || fittedOnOpen.current) return;
    fittedOnOpen.current = true;
    fitFlow();
  }, [nodesReady, fitFlow]);

  const [paletteOpen, setPaletteOpen] = useState(false);
  const [connectOpen, setConnectOpen] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [outlineOpen, setOutlineOpen] = useState(defaultOutlineOpen);
  const [pendingDelete, setPendingDelete] = useState<readonly string[] | undefined>();
  const [detailsId, setDetailsId] = useState<string | undefined>();

  const selectedIds = useMemo(() => nodes.filter((node) => node.selected).map((node) => node.id), [nodes]);
  const edges = useMemo(() => toFlowEdges(intl, graph), [intl, graph]);

  // The graph is the source of truth; React Flow keeps what it measures.
  useEffect(() => {
    setNodes((previous) => syncNodes(previous, graph, { intl, editable, activeId, select: selectNext.current }));
    selectNext.current = undefined;
  }, [graph, intl, editable, activeId, setNodes]);

  const commit = useCallback(
    (next: Graph) => {
      const updated = pushHistory(historyRef.current, next);
      if (updated === historyRef.current) return;
      historyRef.current = updated;
      setHistory(updated);
      onGraphChange?.(updated.present);
    },
    [onGraphChange],
  );

  const replaceHistory = useCallback(
    (updated: History<Graph>) => {
      historyRef.current = updated;
      setHistory(updated);
      onGraphChange?.(updated.present);
    },
    [onGraphChange],
  );

  // Announce execution state changes, politely and joined.
  const lastStatuses = useRef(new Map<string, StatusKey | undefined>());
  useEffect(() => {
    for (const node of graph.nodes) {
      const previous = lastStatuses.current.get(node.id);
      if (lastStatuses.current.has(node.id) && previous !== node.status && node.status) {
        announce(intl.formatMessage(canvasMessages.announceStatus, { name: node.name, status: intl.formatMessage(statusMap[node.status].label) }));
      }
      lastStatuses.current.set(node.id, node.status);
    }
  }, [graph, intl, announce]);

  const focusNode = useCallback((id: string) => {
    setActiveId(id);
    // Wait for the roving tab stop to move, then focus the element React Flow renders.
    requestAnimationFrame(() =>
      requestAnimationFrame(() => {
        containerRef.current?.querySelector<HTMLElement>(`.react-flow__node[data-id="${CSS.escape(id)}"]`)?.focus({ preventScroll: false });
      }),
    );
  }, []);

  const currentNodeId = (target: HTMLElement | null): string | undefined =>
    target?.closest<HTMLElement>('.react-flow__node')?.dataset.id ?? activeId;

  const openDetails = (id: string) => {
    if (wide && onInspect) onInspect(id);
    else setDetailsId(id);
  };

  const addFromCatalog = (item: CatalogItem) => {
    const anchor = graph.nodes.find((node) => node.id === (selectedIds[0] ?? activeId));
    let at = anchor
      ? { x: anchor.position.x + 320, y: anchor.position.y }
      : (() => {
          const rect = containerRef.current?.getBoundingClientRect();
          return rect ? flow.screenToFlowPosition({ x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }) : { x: 0, y: 0 };
        })();
    const occupied = (point: { x: number; y: number }) =>
      graph.nodes.some((node) => Math.abs(node.position.x - snap(point.x)) < 160 && Math.abs(node.position.y - snap(point.y)) < 64);
    while (occupied(at)) at = { x: at.x, y: at.y + GRID * 12 };
    const result = addNode(graph, item, at);
    selectNext.current = new Set([result.id]);
    commit(result.graph);
    announce(intl.formatMessage(canvasMessages.announceAdded, { name: item.name }));
    setActiveId(result.id);
    // The dialog gives focus back to where it came from; move it to the new node afterwards.
    setTimeout(() => focusNode(result.id), 80);
  };

  const connectNodes = (request: ConnectRequest) => {
    const result = connect(graph, request);
    if (result.error) return;
    commit(result.graph);
    const nameOf = (id: string) => graph.nodes.find((node) => node.id === id)?.name ?? id;
    announce(intl.formatMessage(canvasMessages.announceConnected, { source: nameOf(request.source), target: nameOf(request.target) }));
  };

  const disconnect = (edgeId: string) => {
    const edge = graph.edges.find((candidate) => candidate.id === edgeId);
    if (!edge) return;
    commit(removeEdge(graph, edgeId));
    const nameOf = (id: string) => graph.nodes.find((node) => node.id === id)?.name ?? id;
    announce(intl.formatMessage(canvasMessages.announceDisconnected, { source: nameOf(edge.source), target: nameOf(edge.target) }));
  };

  const undo = () => {
    const updated = undoHistory(historyRef.current);
    if (updated === historyRef.current) return;
    replaceHistory(updated);
    announce(intl.formatMessage(canvasMessages.announceUndone));
  };
  const redo = () => {
    const updated = redoHistory(historyRef.current);
    if (updated === historyRef.current) return;
    replaceHistory(updated);
    announce(intl.formatMessage(canvasMessages.announceRedone));
  };

  const deleteNodes = (ids: readonly string[]) => {
    if (ids.length === 0) return;
    const remaining = readingOrder(removeNodes(graph, ids));
    commit(removeNodes(graph, ids));
    announce(intl.formatMessage(canvasMessages.announceDeleted, { count: ids.length }));
    toast(intl.formatMessage(canvasMessages.undoToast, { count: ids.length }), {
      action: { label: intl.formatMessage(canvasMessages.undoAction), onClick: undo },
    });
    const next = remaining[0]?.id;
    setActiveId(next);
    if (next) setTimeout(() => focusNode(next), 0);
  };

  const requestDelete = (ids: readonly string[]) => {
    if (ids.length > 1) setPendingDelete(ids);
    else deleteNodes(ids);
  };

  const nudge = (ids: readonly string[], direction: Direction, steps: number) => {
    const delta = arrowStep[direction];
    const moved = moveNodes(graph, ids, delta.x * GRID * steps, delta.y * GRID * steps);
    commit(moved);
    const first = moved.nodes.find((node) => node.id === ids[0]);
    if (first) {
      announce(
        intl.formatMessage(canvasMessages.announceMoved, { name: first.name, column: first.position.x / GRID, row: first.position.y / GRID }),
      );
    }
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement;
    // Dialogs render in a portal; their keys must never reach the canvas.
    if (!containerRef.current?.contains(target)) return;
    if (target.closest('input, textarea, select, [contenteditable="true"]')) return;
    const id = currentNodeId(target);
    const modifier = event.metaKey || event.ctrlKey;
    const key = event.key;

    if (key === '?') {
      event.preventDefault();
      setShortcutsOpen(true);
      return;
    }
    if (modifier && key.toLowerCase() === 'z' && editable) {
      event.preventDefault();
      if (event.shiftKey) redo();
      else undo();
      return;
    }
    if (modifier && key.toLowerCase() === 'y' && editable) {
      event.preventDefault();
      redo();
      return;
    }
    if (modifier) return;

    const direction = arrowDirection[key];
    if (direction) {
      event.preventDefault();
      if (event.shiftKey) {
        if (!editable || !id) return;
        const moving = selectedIds.includes(id) ? selectedIds : [id];
        nudge(moving, direction, event.altKey ? COARSE_STEPS : 1);
      } else if (id) {
        const next = nextFocus(graph, id, direction);
        if (next) focusNode(next);
      }
      return;
    }
    if (key === 'Enter' && id) {
      event.preventDefault();
      setNodes((previous) => previous.map((node) => ({ ...node, selected: node.id === id })));
      openDetails(id);
      return;
    }
    if (key === ' ' && id) {
      event.preventDefault();
      setNodes((previous) => previous.map((node) => (node.id === id ? { ...node, selected: !node.selected } : node)));
      return;
    }
    if (key === 'Escape') {
      setNodes((previous) => previous.map((node) => ({ ...node, selected: false })));
      return;
    }
    if (!editable) return;
    if ((key === 'a' || key === 'A') && catalog.length > 0) {
      event.preventDefault();
      setPaletteOpen(true);
    } else if (key === 'c' || key === 'C') {
      event.preventDefault();
      setConnectOpen(true);
    } else if ((key === 'Delete' || key === 'Backspace') && id) {
      event.preventDefault();
      requestDelete(selectedIds.length > 0 ? selectedIds : [id]);
    }
  };

  const onConnectByPointer = (connection: Connection) => {
    if (!connection.sourceHandle || !connection.targetHandle) return;
    connectNodes({ source: connection.source, sourceOutput: connection.sourceHandle, target: connection.target, targetInput: connection.targetHandle });
  };

  const empty = graph.nodes.length === 0;
  const detailsNode = graph.nodes.find((node) => node.id === detailsId);
  const canDelete = editable && (selectedIds.length > 0 || activeId !== undefined);

  return (
    <div className={cn('flex h-full min-h-0 flex-col gap-2', className)} data-workflow-canvas data-editable={editable}>
      <CanvasToolbar
        editable={editable}
        canUndo={history.past.length > 0}
        canRedo={history.future.length > 0}
        canDelete={canDelete}
        outlineOpen={outlineOpen}
        onAdd={catalog.length > 0 ? () => setPaletteOpen(true) : undefined}
        onConnect={() => setConnectOpen(true)}
        onDelete={() => requestDelete(selectedIds.length > 0 ? selectedIds : activeId ? [activeId] : [])}
        onUndo={undo}
        onRedo={redo}
        onZoomIn={() => void flow.zoomIn()}
        onZoomOut={() => void flow.zoomOut()}
        onFit={fitFlow}
        onToggleOutline={() => setOutlineOpen((open) => !open)}
        onShortcuts={() => setShortcutsOpen(true)}
      />
      {!editable ? <p className="text-fg-muted text-sm">{intl.formatMessage(canvasMessages.readOnly)}</p> : null}
      <div className="flex min-h-0 flex-1 flex-col gap-2 lg:flex-row">
        <div
          ref={containerRef}
          role="region"
          aria-label={intl.formatMessage(canvasMessages.canvasLabel, { nodes: graph.nodes.length })}
          className="bg-canvas border-border relative min-h-node-width min-w-0 flex-1 overflow-hidden rounded-lg border"
          onKeyDown={onKeyDown}
          onFocus={(event) => {
            const id = (event.target as HTMLElement).closest<HTMLElement>('.react-flow__node')?.dataset.id;
            if (id && id !== activeId) setActiveId(id);
          }}
        >
          <ReactFlow<OrconFlowNode, OrconFlowEdge>
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            edgeTypes={edgeTypes}
            onNodesChange={onNodesChange}
            onConnect={onConnectByPointer}
            isValidConnection={(connection) =>
              !connection.sourceHandle || !connection.targetHandle
                ? false
                : connectError(graph, {
                    source: connection.source,
                    sourceOutput: connection.sourceHandle,
                    target: connection.target,
                    targetInput: connection.targetHandle,
                  }) === undefined
            }
            onNodeDragStop={(_event, _node, dragged) => {
              commit(setPositions(graph, new Map(dragged.map((node) => [node.id, node.position]))));
            }}
            onNodeClick={(_event, node) => {
              if (!editable) setDetailsId(node.id);
            }}
            nodesDraggable={editable}
            nodesConnectable={editable}
            nodesFocusable
            edgesFocusable={false}
            elementsSelectable
            disableKeyboardA11y
            deleteKeyCode={null}
            selectionKeyCode={null}
            multiSelectionKeyCode={editable ? 'Shift' : null}
            selectionMode={SelectionMode.Partial}
            snapToGrid
            snapGrid={[GRID, GRID]}
            minZoom={0.25}
            maxZoom={2}
            attributionPosition="bottom-left"
            ariaLabelConfig={{
              'node.a11yDescription.default': intl.formatMessage(canvasMessages.ariaNodeDescription),
              'node.a11yDescription.keyboardDisabled': intl.formatMessage(canvasMessages.ariaNodeDescription),
              'controls.ariaLabel': intl.formatMessage(canvasMessages.ariaControls),
              'minimap.ariaLabel': intl.formatMessage(canvasMessages.ariaMinimap),
            }}
          >
            <Background variant={BackgroundVariant.Dots} gap={GRID * 2} size={1.5} color="var(--orcon-canvas-grid)" />
            {editable ? <MiniMap pannable zoomable position="bottom-right" nodeColor="var(--orcon-canvas-edge)" maskColor="var(--orcon-surface-sunken)" bgColor="var(--orcon-surface-default)" /> : null}
          </ReactFlow>
          {empty ? (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center p-4">
              <EmptyState
                title={intl.formatMessage(canvasMessages.emptyTitle)}
                description={intl.formatMessage(canvasMessages.emptyDescription)}
                headingLevel={3}
              />
            </div>
          ) : null}
        </div>
        {outlineOpen ? (
          <div className="border-border bg-surface min-h-0 overflow-y-auto rounded-lg border p-3 lg:w-node-width lg:shrink-0">
            <CanvasOutline
              graph={graph}
              editable={editable}
              headingLevel={3}
              onFocusNode={focusNode}
              onDisconnect={disconnect}
              onDeleteNode={(id) => requestDelete([id])}
            />
          </div>
        ) : null}
      </div>
      {region}
      {editable ? (
        <>
          <NodePalette catalog={catalog} open={paletteOpen} onOpenChange={setPaletteOpen} onPick={addFromCatalog} />
          <ConnectDialog
            graph={graph}
            open={connectOpen}
            onOpenChange={setConnectOpen}
            initialSource={selectedIds[0] ?? activeId}
            onConnect={connectNodes}
          />
          <AlertDialog
            open={pendingDelete !== undefined}
            onOpenChange={(open) => (open ? undefined : setPendingDelete(undefined))}
            title={intl.formatMessage(canvasMessages.deleteMany, { count: pendingDelete?.length ?? 0 })}
            impact={intl.formatMessage(canvasMessages.deleteManyImpact)}
            confirmLabel={intl.formatMessage(canvasMessages.deleteConfirm)}
            onConfirm={() => {
              if (pendingDelete) deleteNodes(pendingDelete);
              setPendingDelete(undefined);
            }}
          />
        </>
      ) : null}
      <ShortcutsDialog open={shortcutsOpen} onOpenChange={setShortcutsOpen} />
      <NodeDetails
        graph={graph}
        node={detailsNode}
        onClose={() => setDetailsId(undefined)}
        executionHref={detailsNode ? executionHref?.(detailsNode.id) : undefined}
        side={wide ? 'responsive' : 'bottom'}
      />
    </div>
  );
}

/**
 * The flow canvas. From md up it edits: every operation (add, move, connect,
 * delete, undo and redo) works from the keyboard, from the toolbar and from the
 * flow outline, and by pointer as a shortcut. Below md it is view only: pan,
 * zoom, and tapping a node opens its state and counts. Changes are announced
 * politely and joined so a burst is one announcement.
 */
export function WorkflowCanvas(props: WorkflowCanvasProps) {
  return (
    <ReactFlowProvider>
      <Canvas {...props} />
    </ReactFlowProvider>
  );
}
