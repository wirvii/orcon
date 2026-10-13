// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Background,
  BackgroundVariant,
  getNodesBounds,
  getViewportForBounds,
  ReactFlow,
  ReactFlowProvider,
  SelectionMode,
  useNodesInitialized,
  useNodesState,
  useReactFlow,
  useStore,
  type Connection,
  type OnConnectStartParams,
} from '@xyflow/react';
import { useCallback, useEffect, useImperativeHandle, useLayoutEffect, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent, type Ref, type RefObject } from 'react';
import { useIntl, type IntlShape } from 'react-intl';
import { toast } from 'sonner';

import { AlertDialog } from '../components/alert-dialog/AlertDialog';
import { EmptyState } from '../components/empty-state/EmptyState';
import { statusMap, type StatusKey } from '../components/status-badge/status-map';
import { cn } from '../lib/cn';

import { AddFirstStep } from './add-first-step/AddFirstStep';
import { useAnnouncer } from './announcer';
import { CanvasControls, type CanvasControlsProps } from './canvas-controls/CanvasControls';
import { CanvasEdge, type CanvasFlowEdge } from './canvas-edge/CanvasEdge';
import { CanvasMinimap } from './canvas-minimap/CanvasMinimap';
import { CanvasOutline } from './canvas-outline/CanvasOutline';
import { tileRadius, tileRadiusStyle } from './canvas-node/CanvasNode';
import { CanvasTools } from './CanvasTools';
import { ConnectDialog } from './ConnectDialog';
import { CanvasContext, type CanvasContextValue, type CanvasNodeActions, type DetailLevel } from './context';
import { nodeTypes, type CanvasFlowNodeUnion, type NoteFlowNode, type TileFlowNode } from './FlowNodes';
import { canvasMessages } from './messages';
import {
  COARSE_STEPS,
  GRID,
  addNode,
  addNodeAfter,
  addNote,
  connect,
  connectError,
  freeInput,
  freeSpot,
  insertOnEdge,
  metrics,
  moveNodes,
  nextFocus,
  nodeSize,
  pushHistory,
  readingOrder,
  redoHistory,
  removeEdge,
  removeNodes,
  setPositions,
  snap,
  startHistory,
  undoHistory,
  updateNode,
  updateNote,
  MIN_NOTE,
  type CatalogItem,
  type ConnectRequest,
  type Direction,
  type EdgeState,
  type Graph,
  type GraphNode,
  type History,
  type StickyNoteData,
} from './model';
import { NodeDetails } from './NodeDetails';
import { NodePalettePanel } from './node-palette-panel/NodePalettePanel';
import { edgeAriaLabel, nodeAriaLabel } from './nodeLabel';
import { ShortcutsDialog } from './ShortcutsDialog';
import { useMediaQuery, wideQuery } from './useMediaQuery';

const edgeTypes = { canvas: CanvasEdge };

/** Below these zoom levels node text stops being readable, so fitting never goes lower: the view is anchored to the first node instead. */
const READABLE_ZOOM_WIDE = 0.6;
const READABLE_ZOOM_NARROW = 0.6;
const FIT_PADDING = 0.2;
/** Fitting never zooms in past 100 %: a small flow stays at its natural size. */
const FIT_MAX_ZOOM = 1;
const ANCHOR_MARGIN = 24;
/** The minimap stays visible this long after the canvas stops moving. */
const MINIMAP_LINGER_MS = 1000;
/** Above this many nodes, only the nodes on screen are rendered. */
const VIRTUALIZE_ABOVE = 150;

const arrowDirection: Record<string, Direction> = { ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'up', ArrowDown: 'down' };
const arrowStep: Record<Direction, { x: number; y: number }> = {
  left: { x: -1, y: 0 },
  right: { x: 1, y: 0 },
  up: { x: 0, y: -1 },
  down: { x: 0, y: 1 },
};

export interface NodeExecutionState {
  status: StatusKey;
  /** A short error code for the Failed chip. */
  statusCode?: string | undefined;
  uncertainCount?: number | undefined;
}

export interface EdgeExecutionState {
  state: EdgeState;
  items?: number | undefined;
}

/** The execution being shown on top of the flow: the state of each node and each connection. */
export interface ExecutionOverlay {
  nodes?: Readonly<Record<string, NodeExecutionState>> | undefined;
  edges?: Readonly<Record<string, EdgeExecutionState>> | undefined;
}

/** Where a node picked in the palette goes. */
type PaletteTarget =
  | { mode: 'first' }
  | { mode: 'free' }
  | { mode: 'after'; nodeId: string; outputId: string }
  | { mode: 'edge'; edgeId: string };

export interface WorkflowCanvasHandle {
  /** The current graph (without the execution overlay). */
  getGraph: () => Graph;
  /** Changes one node and records the change in the undo history. */
  updateNode: (id: string, patch: Partial<GraphNode>) => void;
  /** Opens the node palette to add a node. */
  openPalette: () => void;
  /** Moves keyboard focus to a node. */
  focusNode: (id: string) => void;
  fit: () => void;
  /** Sets the zoom level (1 is 100 %). */
  zoomTo: (level: number) => void;
}

export interface WorkflowCanvasProps {
  /** The flow to start with. The canvas keeps its own copy and reports every change. */
  defaultGraph: Graph;
  /** Nodes that can be added. Without it the add button, the plus buttons and the A shortcut are not offered. */
  catalog?: readonly CatalogItem[] | undefined;
  /** The execution being shown. Changes of node state are announced politely, at most every five seconds. */
  overlay?: ExecutionOverlay | undefined;
  onGraphChange?: ((graph: Graph) => void) | undefined;
  /** Enter, double-click or "Open details" on a node. Without it, a node opens a panel with its state and counts. */
  onOpenDetails?: ((nodeId: string, tab?: 'agent') => void) | undefined;
  /** Run one node (the toolbar and the R key). */
  onRunStep?: ((nodeId: string) => void) | undefined;
  /** Run the whole flow from a trigger. */
  onExecuteWorkflow?: ((nodeId: string) => void) | undefined;
  /** Link to the execution detail of a node, shown in the node details. */
  executionHref?: ((nodeId: string) => string) | undefined;
  /** Force view-only mode. Below the md breakpoint the canvas is always view-only. */
  readOnly?: boolean | undefined;
  /** Say that the canvas is view only when it is. Off where the page already makes that clear. */
  notice?: boolean | undefined;
  /** Show the strip of tools (add, connect, delete, undo, redo, outline, shortcuts) above the canvas. */
  tools?: boolean | undefined;
  defaultOutlineOpen?: boolean | undefined;
  /** Show the minimap while the canvas moves. Default true when editing. */
  minimap?: boolean | undefined;
  /** Offer the Tidy up control, which snaps the nodes to the grid. */
  tidyUp?: boolean | undefined;
  ref?: Ref<WorkflowCanvasHandle> | undefined;
  className?: string | undefined;
}

/** The graph with the execution overlay on top: the state of each node and connection being shown. */
export function applyOverlay(graph: Graph, overlay: ExecutionOverlay | undefined): Graph {
  if (!overlay) return graph;
  return {
    ...graph,
    nodes: overlay.nodes
      ? graph.nodes.map((node) => {
          const state = overlay.nodes?.[node.id];
          return state ? { ...node, status: state.status, statusCode: state.statusCode, uncertainCount: state.uncertainCount ?? node.uncertainCount } : node;
        })
      : graph.nodes,
    edges: overlay.edges
      ? graph.edges.map((edge) => {
          const state = overlay.edges?.[edge.id];
          return state ? { ...edge, state: state.state, items: state.items ?? edge.items } : edge;
        })
      : graph.edges,
  };
}

interface SyncOptions {
  intl: IntlShape;
  editable: boolean;
  activeId: string | undefined;
  /** When set, exactly these nodes are selected after this sync. */
  select: ReadonlySet<string> | undefined;
  onNoteChange: NoteFlowNode['data']['onChange'];
  onNoteResize: NoteFlowNode['data']['onResize'];
}

/** Builds the React Flow nodes from the graph, keeping what React Flow measured and what is selected. */
/** The radius style of a tile, only when it has one (an absent property differs from an undefined one). */
function tileStyleProps(node: GraphNode): { style?: CSSProperties } {
  const style = tileRadiusStyle(node);
  return style ? { style } : {};
}

function syncNodes(previous: CanvasFlowNodeUnion[], graph: Graph, { intl, editable, activeId, select, onNoteChange, onNoteResize }: SyncOptions): CanvasFlowNodeUnion[] {
  const before = new Map(previous.map((node) => [node.id, node]));
  const tiles = graph.nodes.map((node): TileFlowNode => {
    const old = before.get(node.id) as TileFlowNode | undefined;
    return {
      ...old,
      id: node.id,
      type: 'tile',
      position: old?.dragging ? old.position : node.position,
      // The same data object while the node is unchanged, so React skips redrawing it.
      data: old?.data.node === node ? old.data : { node },
      draggable: editable,
      selectable: true,
      connectable: editable,
      deletable: false,
      ariaLabel: nodeAriaLabel(intl, node),
      ariaRole: 'group',
      className: tileRadius(node),
      ...tileStyleProps(node),
      // One node is in the tab order; the arrow keys move between nodes (a roving tab stop).
      domAttributes: { tabIndex: node.id === activeId ? 0 : -1, 'aria-roledescription': intl.formatMessage(canvasMessages.nodeRoleDescription) },
      selected: select ? select.has(node.id) : (old?.selected ?? false),
    };
  });
  const notes = (graph.notes ?? []).map((note): NoteFlowNode => {
    const old = before.get(note.id) as NoteFlowNode | undefined;
    return {
      ...old,
      id: note.id,
      type: 'sticky',
      position: old?.dragging ? old.position : note.position,
      data: old?.data.note === note ? old.data : { note, onChange: onNoteChange, onResize: onNoteResize },
      width: note.size.w,
      height: note.size.h,
      // Notes sit behind the connections and the nodes.
      zIndex: -1,
      draggable: editable,
      selectable: true,
      connectable: false,
      deletable: false,
      ariaLabel: intl.formatMessage(canvasMessages.stickyNamed, { text: note.body.slice(0, 80) }),
      ariaRole: 'group',
      className: 'rounded-md',
      domAttributes: { tabIndex: note.id === activeId ? 0 : -1, 'aria-roledescription': intl.formatMessage(canvasMessages.stickyLabel) },
      selected: select ? select.has(note.id) : (old?.selected ?? false),
    };
  });
  return [...notes, ...tiles];
}

/** The level of detail for a zoom: the lower it is, the less each node draws. */
function detailOf(zoom: number): DetailLevel {
  return zoom < metrics.lodMinimal ? 'minimal' : zoom < metrics.lodDetail ? 'detail' : 'full';
}

/** At the lowest level of detail no name is drawn, so the scale stays put and a zoom there rebuilds nothing. */
function labelScaleOf(zoom: number): number {
  return detailOf(zoom) === 'detail' ? Math.round(Math.min(3, Math.max(1, 10 / (13 * zoom))) * 100) / 100 : 1;
}

/**
 * The zoom controls and the zoom the container reports (`data-zoom`, in percent). They follow every
 * zoom step, so they live here and the canvas around them does not render again for each one.
 */
function ZoomControls({ containerRef, ...controls }: Omit<CanvasControlsProps, 'zoom'> & { containerRef: RefObject<HTMLDivElement | null> }) {
  const zoom = useStore((state) => state.transform[2]);
  const percent = Math.round(zoom * 100);
  useLayoutEffect(() => {
    containerRef.current?.setAttribute('data-zoom', String(percent));
  }, [containerRef, percent]);
  return <CanvasControls zoom={zoom} {...controls} />;
}

function Canvas({
  defaultGraph,
  catalog = [],
  overlay,
  onGraphChange,
  onOpenDetails,
  onRunStep,
  onExecuteWorkflow,
  executionHref,
  readOnly = false,
  notice = true,
  tools = false,
  defaultOutlineOpen = false,
  minimap = true,
  tidyUp: tidyEnabled = false,
  ref,
  className,
}: WorkflowCanvasProps) {
  const intl = useIntl();
  const wide = useMediaQuery(wideQuery);
  const reducedMotion = useMediaQuery('(prefers-reduced-motion: reduce)');
  const coarse = useMediaQuery('(pointer: coarse)');
  const editable = wide && !readOnly;
  const flow = useReactFlow();
  const { announce, region } = useAnnouncer();

  const [history, setHistory] = useState<History<Graph>>(() => startHistory({ ...defaultGraph, notes: defaultGraph.notes ?? [] }));
  const historyRef = useRef(history);
  const base = history.present;
  const graph = useMemo(() => applyOverlay(base, overlay), [base, overlay]);
  const graphRef = useRef(graph);
  graphRef.current = graph;
  const baseRef = useRef(base);
  baseRef.current = base;

  const [activeId, setActiveId] = useState<string | undefined>(() => readingOrder(defaultGraph)[0]?.id);
  const [focusedId, setFocusedId] = useState<string | undefined>();
  const [focusedEdgeId, setFocusedEdgeId] = useState<string | undefined>();
  const [hoveredEdgeId, setHoveredEdgeId] = useState<string | undefined>();
  const [renamingId, setRenamingId] = useState<string | undefined>();
  const [editingNoteId, setEditingNoteId] = useState<string | undefined>();
  const [connectingFrom, setConnectingFrom] = useState<CanvasContextValue['connectingFrom']>();
  const [palette, setPalette] = useState<PaletteTarget | undefined>();
  const [connectOpen, setConnectOpen] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [outlineOpen, setOutlineOpen] = useState(defaultOutlineOpen);
  const [pendingDelete, setPendingDelete] = useState<readonly string[] | undefined>();
  const [detailsId, setDetailsId] = useState<string | undefined>();
  const [moving, setMoving] = useState(false);
  const [minimapHover, setMinimapHover] = useState(false);

  const editingRef = useRef<string | undefined>(undefined);
  const containerRef = useRef<HTMLDivElement>(null);
  const selectNext = useRef<ReadonlySet<string> | undefined>(undefined);
  const paletteOrigin = useRef<HTMLElement | null>(null);
  const edgeLeave = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const movingTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const onNoteChange = useCallback<NoteFlowNode['data']['onChange']>((id, patch) => {
    commitRef.current(updateNote(baseRef.current, id, patch));
  }, []);
  const onNoteResize = useCallback<NoteFlowNode['data']['onResize']>((id, size) => {
    commitRef.current(updateNote(baseRef.current, id, { size: { w: Math.max(MIN_NOTE.w, snap(size.w)), h: Math.max(MIN_NOTE.h, snap(size.h)) } }));
  }, []);

  const firstNodes = useMemo(
    () => syncNodes([], graph, { intl, editable, activeId, select: undefined, onNoteChange, onNoteResize }),
    // Only the first render needs this: later changes go through the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- the first render reads the initial values once on purpose
    [],
  );
  const [nodes, setNodes, onNodesChange] = useNodesState<CanvasFlowNodeUnion>(firstNodes);

  // The canvas reads the zoom through selectors that return only the level of detail and the label scale,
  // so it renders again when these change and not on every wheel event: each render hands React Flow new
  // callbacks, which it writes into its store and so wakes thousands of subscribers (orcon-web PF-4).
  const detail = useStore((state) => detailOf(state.transform[2]));
  // Names and chips grow as the canvas shrinks, so their on-screen size stays at 10 px or more.
  const labelScale = useStore((state) => labelScaleOf(state.transform[2]));

  /**
   * Shows the whole flow when that keeps the text readable. When the flow is too
   * wide for that, it keeps the readable zoom and starts at the first node (the
   * trigger), so the start of the flow is always on screen; pan to see the rest.
   */
  const fittedSize = useRef('');
  const fitFlow = useCallback(() => {
    const rect = containerRef.current?.getBoundingClientRect();
    const all = flow.getNodes();
    if (!rect || rect.width === 0 || all.length === 0) return;
    fittedSize.current = `${Math.round(rect.width)}x${Math.round(rect.height)}`;
    // Screenshots wait until this equals the canvas size: a fit made for an older size is stale.
    containerRef.current?.setAttribute('data-fit-size', fittedSize.current);
    // With a coarse pointer controls must stay at least 44 px, so the flow is never fitted below 100 %: it starts at the trigger and is panned.
    const readable = coarse ? 1 : wide ? READABLE_ZOOM_WIDE : READABLE_ZOOM_NARROW;
    const bounds = getNodesBounds(all);
    const fitted = getViewportForBounds(bounds, rect.width, rect.height, metrics.minZoom, FIT_MAX_ZOOM, FIT_PADDING);
    const duration = reducedMotion ? 0 : 160;
    // A view-only canvas always shows the whole flow: there is nothing to edit, so the text size gives way to seeing everything.
    if (!editable || fitted.zoom >= readable || all.length > VIRTUALIZE_ABOVE) {
      void flow.setViewport(fitted, { duration });
      return;
    }
    const first = all.find((node) => node.id === readingOrder(graphRef.current)[0]?.id) ?? all[0];
    if (!first) return;
    void flow.setViewport(
      { x: ANCHOR_MARGIN - first.position.x * readable, y: rect.height / 2 - (bounds.y + bounds.height / 2) * readable, zoom: readable },
      { duration },
    );
  }, [flow, wide, reducedMotion, coarse, editable]);
  const nodesReady = useNodesInitialized();
  const fittedOnOpen = useRef(false);
  useEffect(() => {
    if (!nodesReady || fittedOnOpen.current) return;
    fittedOnOpen.current = true;
    fitFlow();
    // Tests and screenshots wait for this mark: the first fit has been applied.
    requestAnimationFrame(() => containerRef.current?.setAttribute('data-fitted', 'true'));
  }, [nodesReady, fitFlow]);

  // The first fit follows the size of the canvas until a person moves the view. Other content sizes the
  // canvas too (a notice above it wraps differently once the web fonts are drawn), so the size at the
  // moment of the first fit is not always the final one; fitting once would leave the flow off-center.
  const userMoved = useRef(false);
  const fitRef = useRef(fitFlow);
  fitRef.current = fitFlow;
  useEffect(() => {
    const element = containerRef.current;
    if (!element || typeof ResizeObserver === 'undefined') return;
    let frame = 0;
    const observer = new ResizeObserver(() => {
      const box = element.getBoundingClientRect();
      const size = `${Math.round(box.width)}x${Math.round(box.height)}`;
      // Only a size different from the one the last fit used asks for a new fit.
      if (!fittedOnOpen.current || size === fittedSize.current || userMoved.current) return;
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        if (!userMoved.current) fitRef.current();
      });
    });
    observer.observe(element);
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
    };
  }, []);

  // Where focus was inside the canvas, so a dialog opened from the keyboard hands it back when it closes.
  const dialogOpen = shortcutsOpen || connectOpen || pendingDelete !== undefined;
  const wasDialogOpen = useRef(false);
  useEffect(() => {
    if (wasDialogOpen.current && !dialogOpen) {
      requestAnimationFrame(() => {
        const active = document.activeElement;
        if (active && active !== document.body) return;
        const target = lastFocus.current?.isConnected ? lastFocus.current : containerRef.current;
        target?.focus({ preventScroll: true });
      });
    }
    wasDialogOpen.current = dialogOpen;
  }, [dialogOpen]);

  // A canvas without nodes has nothing to fit.
  useEffect(() => {
    if (graph.nodes.length === 0) containerRef.current?.setAttribute('data-fitted', 'true');
  }, [graph.nodes.length]);

  const selectedIds = useMemo(() => nodes.filter((node) => node.selected).map((node) => node.id), [nodes]);

  // The graph is the source of truth; React Flow keeps what it measures.
  useEffect(() => {
    // The selection to apply is read now: the updater below runs later, after the ref is cleared.
    const select = selectNext.current;
    setNodes((previous) => syncNodes(previous, graph, { intl, editable, activeId, select, onNoteChange, onNoteResize }));
    selectNext.current = undefined;
  }, [graph, intl, editable, activeId, setNodes, onNoteChange, onNoteResize]);

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
  const commitRef = useRef(commit);
  commitRef.current = commit;

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
    const focus = () => containerRef.current?.querySelector<HTMLElement>(`.react-flow__node[data-id="${CSS.escape(id)}"]`)?.focus({ preventScroll: false });
    // Focus now, so a held arrow key never acts on the node focus is leaving, and again once the roving tab stop has moved.
    focus();
    requestAnimationFrame(() => requestAnimationFrame(focus));
  }, []);

  const nameOf = (id: string) => graphRef.current.nodes.find((node) => node.id === id)?.name ?? id;

  const openDetails = useCallback(
    (id: string, tab?: 'agent') => {
      if (graphRef.current.notes?.some((note) => note.id === id)) {
        editingRef.current = id;
        setEditingNoteId(id);
        return;
      }
      if (wide && onOpenDetails) onOpenDetails(id, tab);
      else setDetailsId(id);
    },
    [wide, onOpenDetails],
  );

  const lastFocus = useRef<HTMLElement | null>(null);
  const closePalette = useCallback(() => {
    setPalette(undefined);
    const origin = paletteOrigin.current;
    paletteOrigin.current = null;
    // Back to where the person was; when that element has been replaced, to the last focused item or the canvas itself.
    setTimeout(() => {
      const edgeId = origin?.closest<HTMLElement>('.react-flow__edge')?.dataset['id'];
      const replaced = edgeId ? containerRef.current?.querySelector<HTMLElement>(`.react-flow__edge[data-id="${CSS.escape(edgeId)}"]`) : null;
      const target = origin?.isConnected ? origin : replaced ?? (lastFocus.current?.isConnected ? lastFocus.current : containerRef.current);
      target?.focus({ preventScroll: true });
    }, 0);
  }, []);

  const openPalette = useCallback((target: PaletteTarget) => {
    paletteOrigin.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setPalette(target);
  }, []);

  const viewportCenter = () => {
    const rect = containerRef.current?.getBoundingClientRect();
    return rect ? flow.screenToFlowPosition({ x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 }) : { x: 0, y: 0 };
  };

  const pickFromPalette = (item: CatalogItem) => {
    const target = palette;
    const current = graphRef.current;
    let result: { graph: Graph; id: string };
    if (target?.mode === 'after') {
      result = addNodeAfter(current, item, target.nodeId, target.outputId);
      announce(intl.formatMessage(canvasMessages.announceAdded, { name: item.name }));
    } else if (target?.mode === 'edge') {
      const edge = current.edges.find((candidate) => candidate.id === target.edgeId);
      result = insertOnEdge(current, target.edgeId, item);
      announce(intl.formatMessage(canvasMessages.announceInserted, { name: item.name, source: nameOf(edge?.source ?? ''), target: nameOf(edge?.target ?? '') }));
    } else {
      const center = viewportCenter();
      const size = nodeSize({ kind: item.kind, inputs: item.inputs, outputs: item.outputs });
      result = addNode(current, item, freeSpot(current, { x: center.x - size.w / 2, y: center.y - size.h / 2 }, size));
      announce(intl.formatMessage(canvasMessages.announceAdded, { name: item.name }));
    }
    selectNext.current = new Set([result.id]);
    paletteOrigin.current = null;
    setPalette(undefined);
    commit(result.graph);
    setActiveId(result.id);
    setTimeout(() => focusNode(result.id), 60);
  };

  const connectNodes = (request: ConnectRequest) => {
    const result = connect(graphRef.current, request);
    if (result.error) return;
    commit(result.graph);
    announce(intl.formatMessage(canvasMessages.announceConnected, { source: nameOf(request.source), target: nameOf(request.target) }));
  };

  const disconnect = (edgeId: string) => {
    const edge = graphRef.current.edges.find((candidate) => candidate.id === edgeId);
    if (!edge) return;
    commit(removeEdge(graphRef.current, edgeId));
    setFocusedEdgeId(undefined);
    setHoveredEdgeId(undefined);
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
    const remaining = readingOrder(removeNodes(graphRef.current, ids));
    commit(removeNodes(graphRef.current, ids));
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
    const moved = moveNodes(graphRef.current, ids, delta.x * GRID * steps, delta.y * GRID * steps);
    commit(moved);
    const first = moved.nodes.find((node) => node.id === ids[0]);
    if (first) {
      announce(intl.formatMessage(canvasMessages.announceMoved, { name: first.name, column: first.position.x / GRID, row: first.position.y / GRID }));
    }
  };

  const resizeNote = (id: string, direction: Direction) => {
    const note = baseRef.current.notes?.find((candidate) => candidate.id === id);
    if (!note) return;
    const delta = arrowStep[direction];
    const size = { w: Math.max(MIN_NOTE.w, note.size.w + delta.x * GRID), h: Math.max(MIN_NOTE.h, note.size.h + delta.y * GRID) };
    commit(updateNote(baseRef.current, id, { size }));
    announce(intl.formatMessage(canvasMessages.announceResized, { width: size.w, height: size.h }));
  };

  const tidyUp = () => {
    // Columns by reading order: every node snaps to the grid and keeps its row.
    const positions = new Map(graphRef.current.nodes.map((node) => [node.id, { x: node.position.x, y: node.position.y }]));
    commit(setPositions(graphRef.current, positions));
  };

  const addSticky = () => {
    const anchor = graphRef.current.nodes.find((node) => node.id === (selectedIds[0] ?? activeId));
    const at = anchor ? { x: anchor.position.x, y: anchor.position.y + 220 } : viewportCenter();
    const result = addNote(baseRef.current, at, '');
    selectNext.current = new Set([result.id]);
    commit(result.graph);
    announce(intl.formatMessage(canvasMessages.announceNoteAdded));
    setActiveId(result.id);
    // The note opens for editing and its text field takes focus; the node regains focus when editing ends.
    setTimeout(() => {
      editingRef.current = result.id;
      setEditingNoteId(result.id);
    }, 60);
  };

  // The actions of a node (toolbar, plus buttons, rename). They read the latest state through refs, so the object is stable.
  const callbacks = useRef({ onRunStep, onExecuteWorkflow, openDetails, openPalette, requestDelete, announce, intl, commit, focusNode });
  callbacks.current = { onRunStep, onExecuteWorkflow, openDetails, openPalette, requestDelete, announce, intl, commit, focusNode };
  const actions = useMemo<CanvasNodeActions>(
    () => ({
      addAfter: (nodeId, outputId) => callbacks.current.openPalette({ mode: 'after', nodeId, outputId }),
      runStep: (nodeId) => {
        callbacks.current.onRunStep?.(nodeId);
        callbacks.current.announce(callbacks.current.intl.formatMessage(canvasMessages.announceRunning, { name: nameOfRef(nodeId) }));
      },
      executeWorkflow: (nodeId) => callbacks.current.onExecuteWorkflow?.(nodeId),
      toggleDisabled: (nodeId) => {
        const node = baseRef.current.nodes.find((candidate) => candidate.id === nodeId);
        if (!node || node.kind === 'placeholder') return;
        callbacks.current.commit(updateNode(baseRef.current, nodeId, { disabled: !node.disabled }));
        callbacks.current.announce(callbacks.current.intl.formatMessage(node.disabled ? canvasMessages.announceEnabled : canvasMessages.announceDisabled, { name: node.name }));
      },
      remove: (nodeId) => callbacks.current.requestDelete([nodeId]),
      openDetails: (nodeId, tab) => callbacks.current.openDetails(nodeId, tab),
      startRename: (nodeId) => setRenamingId(nodeId),
      commitRename: (nodeId, name) => {
        callbacks.current.commit(updateNode(baseRef.current, nodeId, { name }));
        setRenamingId(undefined);
        callbacks.current.announce(callbacks.current.intl.formatMessage(canvasMessages.announceRenamed, { name }));
        callbacks.current.focusNode(nodeId);
      },
      cancelRename: (nodeId) => {
        setRenamingId(undefined);
        callbacks.current.focusNode(nodeId);
      },
      duplicate: (nodeId) => {
        const node = baseRef.current.nodes.find((candidate) => candidate.id === nodeId);
        if (!node) return;
        const result = addNode(baseRef.current, { id: node.id, kind: node.kind, logicType: node.logicType, name: node.name, subtitle: node.subtitle, group: '', icon: node.icon, connector: node.connector, inputs: node.inputs, outputs: node.outputs }, freeSpot(baseRef.current, { x: node.position.x, y: node.position.y + 160 }, nodeSize(node)));
        selectNext.current = new Set([result.id]);
        callbacks.current.commit(result.graph);
        callbacks.current.announce(callbacks.current.intl.formatMessage(canvasMessages.announceDuplicated, { name: node.name }));
        callbacks.current.focusNode(result.id);
      },
      togglePin: (nodeId) => {
        const node = baseRef.current.nodes.find((candidate) => candidate.id === nodeId);
        if (!node) return;
        callbacks.current.commit(updateNode(baseRef.current, nodeId, { pinned: !node.pinned }));
        callbacks.current.announce(callbacks.current.intl.formatMessage(node.pinned ? canvasMessages.announceUnpinned : canvasMessages.announcePinned, { name: node.name }));
      },
      copy: (nodeId) => {
        const node = baseRef.current.nodes.find((candidate) => candidate.id === nodeId);
        if (node) void navigator.clipboard.writeText(JSON.stringify({ ...node, status: undefined })).catch(() => undefined);
      },
      openAiBlock: (nodeId) => callbacks.current.openDetails(nodeId, 'agent'),
    }),
    [],
  );
  const nameOfRef = (id: string) => baseRef.current.nodes.find((node) => node.id === id)?.name ?? id;

  useImperativeHandle(
    ref,
    () => ({
      getGraph: () => baseRef.current,
      updateNode: (id, patch) => commitRef.current(updateNode(baseRef.current, id, patch)),
      openPalette: () => openPalette({ mode: 'free' }),
      focusNode,
      fit: fitFlow,
      zoomTo: (level) => void flow.zoomTo(level, { duration: 0 }),
    }),
    [focusNode, fitFlow, openPalette, flow],
  );

  const currentItemId = (target: HTMLElement | null): string | undefined => target?.closest<HTMLElement>('.react-flow__node')?.dataset['id'] ?? activeId;
  const currentEdgeId = (target: HTMLElement | null): string | undefined => target?.closest<HTMLElement>('.react-flow__edge')?.dataset['id'];

  const zoomBy = (kind: 'in' | 'out' | 'reset' | 'fit') => {
    pulseMinimap();
    const options = { duration: reducedMotion ? 0 : 120 };
    if (kind === 'in') void flow.zoomIn(options);
    else if (kind === 'out') void flow.zoomOut(options);
    else if (kind === 'reset') void flow.zoomTo(1, options);
    else fitFlow();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    userMoved.current = true;
    const target = event.target as HTMLElement;
    // Dialogs and the palette handle their own keys; nothing typed in a field reaches the canvas.
    if (!containerRef.current?.contains(target)) return;
    if (target.closest('input, textarea, select, [contenteditable="true"], [data-node-palette]')) return;
    // Enter and Space on a button of the node (toolbar, port, stub, block) belong to that button.
    if ((event.key === 'Enter' || event.key === ' ') && target.closest('button, [role="button"], [role="menuitem"], a[href]')) return;
    const edgeId = currentEdgeId(target);
    const id = edgeId ? undefined : currentItemId(target);
    const modifier = event.metaKey || event.ctrlKey;
    const key = event.key;
    const isNote = id !== undefined && Boolean(graphRef.current.notes?.some((note) => note.id === id));

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
    if (modifier && key.toLowerCase() === 'a' && editable) {
      event.preventDefault();
      setNodes((previous) => previous.map((item) => ({ ...item, selected: true })));
      return;
    }
    if (modifier && event.shiftKey && isNote && editable && arrowDirection[key]) {
      event.preventDefault();
      resizeNote(id, arrowDirection[key]);
      return;
    }
    if (modifier) return;

    if (key === '+' || key === '=') {
      event.preventDefault();
      zoomBy('in');
      return;
    }
    if (key === '-') {
      event.preventDefault();
      zoomBy('out');
      return;
    }
    if (key === '0') {
      event.preventDefault();
      zoomBy('reset');
      return;
    }
    if (key === '1') {
      event.preventDefault();
      zoomBy('fit');
      return;
    }

    if (edgeId) {
      if (!editable) return;
      if (key === 'Enter' && catalog.length > 0) {
        event.preventDefault();
        openPalette({ mode: 'edge', edgeId });
      } else if (key === 'Delete' || key === 'Backspace') {
        event.preventDefault();
        // The connection is gone, so focus goes to the node it left.
        const source = graphRef.current.edges.find((candidate) => candidate.id === edgeId)?.source;
        disconnect(edgeId);
        if (source) focusNode(source);
      } else if (key === 'Escape') {
        setFocusedEdgeId(undefined);
      }
      return;
    }

    const direction = arrowDirection[key];
    if (direction) {
      event.preventDefault();
      // The arrow keys act on the item that has focus; with focus on the canvas itself they pan.
      const focusedItem = target.closest<HTMLElement>('.react-flow__node')?.dataset['id'];
      if (event.shiftKey) {
        if (!editable || !focusedItem) return;
        const moving = selectedIds.includes(focusedItem) ? selectedIds : [focusedItem];
        nudge(moving, direction, event.altKey ? COARSE_STEPS : 1);
      } else if (focusedItem) {
        const next = nextFocus(graphRef.current, focusedItem, direction);
        if (next) focusNode(next);
      } else {
        const bounds = flow.getViewport();
        void flow.setViewport({ ...bounds, x: bounds.x - (direction === 'left' ? -GRID : direction === 'right' ? GRID : 0) * 2, y: bounds.y - (direction === 'up' ? -GRID : direction === 'down' ? GRID : 0) * 2 });
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
    const lower = key.toLowerCase();
    if (event.shiftKey && lower === 's') {
      event.preventDefault();
      addSticky();
    } else if (lower === 'a' && catalog.length > 0) {
      event.preventDefault();
      const anchor = id && !isNote ? graphRef.current.nodes.find((node) => node.id === id) : undefined;
      const output = anchor?.outputs[0];
      if (anchor && output && graphRef.current.edges.every((edge) => !(edge.source === anchor.id && edge.sourceOutput === output.id))) {
        openPalette({ mode: 'after', nodeId: anchor.id, outputId: output.id });
      } else if (anchor && output) {
        // The first output is taken: use the first one that is free, or the first.
        const free = anchor.outputs.find((port) => graphRef.current.edges.every((edge) => !(edge.source === anchor.id && edge.sourceOutput === port.id)));
        openPalette({ mode: 'after', nodeId: anchor.id, outputId: (free ?? output).id });
      } else {
        openPalette(graphRef.current.nodes.length === 0 ? { mode: 'first' } : { mode: 'free' });
      }
    } else if (lower === 'c') {
      event.preventDefault();
      if (!quickConnect()) setConnectOpen(true);
    } else if (lower === 'r' && id && !isNote) {
      event.preventDefault();
      actions.runStep(id);
    } else if (lower === 'd' && id && !isNote) {
      event.preventDefault();
      actions.toggleDisabled(id);
    } else if (lower === 'p' && id && !isNote) {
      event.preventDefault();
      actions.togglePin(id);
    } else if (key === 'F2' && id && !isNote) {
      event.preventDefault();
      setRenamingId(id);
    } else if ((key === 'Delete' || key === 'Backspace') && id) {
      event.preventDefault();
      requestDelete(selectedIds.length > 0 ? selectedIds : [id]);
    }
  };

  /**
   * With exactly two nodes selected, C connects the one on the left to the one on the right using
   * the first free output and the first free input. It returns false when that is not possible, and
   * the connect form opens instead.
   */
  const quickConnect = (): boolean => {
    const chosen = graphRef.current.nodes.filter((node) => selectedIds.includes(node.id));
    if (chosen.length !== 2) return false;
    const [source, target] = [...chosen].sort((a, b) => a.position.x - b.position.x) as [GraphNode, GraphNode];
    const output = source.outputs.find((port) => !graphRef.current.edges.some((edge) => edge.source === source.id && edge.sourceOutput === port.id)) ?? source.outputs[0];
    const input = freeInput(graphRef.current, target.id);
    if (!output || !input) return false;
    const request = { source: source.id, sourceOutput: output.id, target: target.id, targetInput: input };
    if (connectError(graphRef.current, request) !== undefined) return false;
    connectNodes(request);
    return true;
  };

  const onConnectByPointer = (connection: Connection) => {
    if (!connection.sourceHandle || !connection.targetHandle) return;
    connectNodes({ source: connection.source, sourceOutput: connection.sourceHandle, target: connection.target, targetInput: connection.targetHandle });
  };

  const onConnectStart = (_event: unknown, params: OnConnectStartParams) => {
    if (params.handleType === 'source' && params.nodeId && params.handleId) setConnectingFrom({ nodeId: params.nodeId, outputId: params.handleId });
  };

  const showMinimap = editable && minimap && graph.nodes.length > 0;
  const minimapVisible = moving || minimapHover;
  // The minimap shows while a person pans or zooms and for a second after; the first fit on open does not count.
  const pulseMinimap = () => {
    if (movingTimer.current) clearTimeout(movingTimer.current);
    setMoving(true);
    movingTimer.current = setTimeout(() => setMoving(false), MINIMAP_LINGER_MS);
  };
  const onMoveStart = (event: unknown) => {
    if (event) {
      userMoved.current = true;
      pulseMinimap();
    }
  };
  const onMove = (event: unknown) => {
    if (event) pulseMinimap();
  };
  useEffect(
    () => () => {
      if (movingTimer.current) clearTimeout(movingTimer.current);
      if (edgeLeave.current) clearTimeout(edgeLeave.current);
    },
    [],
  );

  const activeEdgeId = hoveredEdgeId ?? focusedEdgeId;
  const edges = useMemo<CanvasFlowEdge[]>(() => {
    const byId = new Map(graph.nodes.map((node) => [node.id, node]));
    const pinnedSources = new Set(graph.nodes.filter((node) => node.pinned).map((node) => node.id));
    return graph.edges.map((edge) => {
      const source = byId.get(edge.source);
      const target = byId.get(edge.target);
      return {
        id: edge.id,
        type: 'canvas',
        source: edge.source,
        target: edge.target,
        sourceHandle: edge.sourceOutput,
        targetHandle: edge.targetInput,
        focusable: editable,
        interactionWidth: 20,
        ariaLabel: edgeAriaLabel(intl, edge, source, target),
        data: {
          state: edge.state,
          items: edge.items,
          pinned: pinnedSources.has(edge.source) && edge.state !== undefined,
          loopBack: edge.loopBack,
          active: activeEdgeId === edge.id,
          sourceName: source?.name,
          targetName: target?.name,
          editable: editable && catalog.length > 0,
          detail,
          onAdd: () => openPalette({ mode: 'edge', edgeId: edge.id }),
          onDelete: () => disconnect(edge.id),
          onBarEnter: () => {
            if (edgeLeave.current) clearTimeout(edgeLeave.current);
            setHoveredEdgeId(edge.id);
          },
          onBarLeave: () => setHoveredEdgeId(undefined),
        },
      } satisfies CanvasFlowEdge;
    });
    // The handlers read the latest graph through refs; they are rebuilt only when the data they show changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- the handlers are not dependencies on purpose (see above)
  }, [graph, intl, editable, catalog.length, detail, activeEdgeId]);

  const dangling = useMemo(() => {
    const used = new Map<string, Set<string>>();
    for (const edge of graph.edges) {
      if (!used.has(edge.source)) used.set(edge.source, new Set());
      used.get(edge.source)?.add(edge.sourceOutput);
    }
    return new Map(graph.nodes.map((node) => [node.id, node.outputs.filter((port) => !used.get(node.id)?.has(port.id)).map((port) => port.id)]));
  }, [graph]);

  const contextValue = useMemo<CanvasContextValue>(
    () => ({
      graph,
      editable,
      activeId,
      detail,
      labelScale,
      renamingId,
      editingNoteId,
      setEditingNote: (id) => {
        if (id === undefined && editingRef.current) focusNode(editingRef.current);
        editingRef.current = id;
        setEditingNoteId(id);
      },
      focusedId,
      dangling,
      connectingFrom,
      actions,
    }),
    [graph, editable, activeId, detail, labelScale, renamingId, editingNoteId, focusedId, dangling, connectingFrom, actions, focusNode],
  );

  const empty = graph.nodes.length === 0 && (graph.notes ?? []).length === 0;
  const detailsNode = graph.nodes.find((node) => node.id === detailsId);
  const canDelete = editable && (selectedIds.length > 0 || activeId !== undefined);

  return (
    <div className={cn('flex h-full min-h-0 flex-col', className)} data-workflow-canvas data-editable={editable}>
      {tools ? (
        <CanvasTools
          editable={editable}
          canUndo={history.past.length > 0}
          canRedo={history.future.length > 0}
          canDelete={canDelete}
          outlineOpen={outlineOpen}
          onAdd={catalog.length > 0 ? () => openPalette({ mode: graph.nodes.length === 0 ? 'first' : 'free' }) : undefined}
          onConnect={() => setConnectOpen(true)}
          onAddSticky={addSticky}
          onDelete={() => requestDelete(selectedIds.length > 0 ? selectedIds : activeId ? [activeId] : [])}
          onUndo={undo}
          onRedo={redo}
          onToggleOutline={() => setOutlineOpen((open) => !open)}
          onShortcuts={() => setShortcutsOpen(true)}
        />
      ) : null}
      {!editable && notice ? <p className="text-fg-muted px-3 py-2 text-sm">{intl.formatMessage(canvasMessages.readOnly)}</p> : null}
      <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
        <div
          ref={containerRef}
          role="region"
          aria-label={intl.formatMessage(canvasMessages.canvasLabel, { nodes: graph.nodes.length })}
          data-lod={detail}
          tabIndex={0}
          className="bg-canvas relative min-h-canvas-node min-w-0 flex-1 overflow-hidden"
          onKeyDown={onKeyDown}
          onPointerDownCapture={() => {
            userMoved.current = true;
          }}
          onWheelCapture={() => {
            userMoved.current = true;
          }}
          onFocus={(event) => {
            const target = event.target as HTMLElement;
            lastFocus.current = target;
            const id = target.closest<HTMLElement>('.react-flow__node')?.dataset['id'];
            if (id) {
              setFocusedId(id);
              if (id !== activeId) setActiveId(id);
            }
            setFocusedEdgeId(target.closest<HTMLElement>('.react-flow__edge')?.dataset['id']);
          }}
          onBlur={(event) => {
            const next = event.relatedTarget as HTMLElement | null;
            // Focus moving into a menu opened from the node's toolbar keeps the toolbar, and the menu, in place.
            if (next?.closest('[role="menu"]')) return;
            const inside = next?.closest<HTMLElement>('.react-flow__node')?.dataset['id'];
            setFocusedId((current) => (inside && inside === current ? current : undefined));
            if (!next?.closest('.react-flow__edge')) setFocusedEdgeId(undefined);
          }}
        >
          <CanvasContext.Provider value={contextValue}>
            <ReactFlow<CanvasFlowNodeUnion, CanvasFlowEdge>
              nodes={nodes}
              edges={edges}
              nodeTypes={nodeTypes}
              edgeTypes={edgeTypes}
              onNodesChange={onNodesChange}
              onConnect={onConnectByPointer}
              onConnectStart={onConnectStart}
              onConnectEnd={() => setConnectingFrom(undefined)}
              connectOnClick={false}
              connectionLineStyle={{ stroke: 'var(--orcon-canvas-selection)', strokeWidth: 2 }}
              isValidConnection={(connection) =>
                !connection.sourceHandle || !connection.targetHandle
                  ? false
                  : connectError(graphRef.current, {
                      source: connection.source,
                      sourceOutput: connection.sourceHandle,
                      target: connection.target,
                      targetInput: connection.targetHandle,
                    }) === undefined
              }
              onNodeDragStop={(_event, _node, dragged) => {
                commit(setPositions(baseRef.current, new Map(dragged.map((node) => [node.id, node.position]))));
              }}
              onNodeClick={(_event, node) => {
                if (!editable && node.type === 'tile') setDetailsId(node.id);
              }}
              onNodeDoubleClick={(_event, node) => {
                if (editable && node.type === 'tile') openDetails(node.id);
              }}
              onEdgeMouseEnter={(_event, edge) => {
                if (edgeLeave.current) clearTimeout(edgeLeave.current);
                setHoveredEdgeId(edge.id);
              }}
              onEdgeMouseLeave={() => {
                if (edgeLeave.current) clearTimeout(edgeLeave.current);
                edgeLeave.current = setTimeout(() => setHoveredEdgeId(undefined), 160);
              }}
              onMoveStart={onMoveStart}
              onMove={onMove}
              nodesDraggable={editable}
              nodesConnectable={editable}
              nodesFocusable
              edgesFocusable={editable}
              elementsSelectable
              disableKeyboardA11y
              deleteKeyCode={null}
              // Drag on empty space selects (marquee); Space + drag or the middle button pans; the wheel pans, Ctrl or Cmd + wheel zooms.
              selectionKeyCode={null}
              selectionOnDrag={editable}
              panOnDrag={editable ? [1, 2] : true}
              panActivationKeyCode="Space"
              panOnScroll
              zoomOnScroll={false}
              zoomOnDoubleClick={false}
              multiSelectionKeyCode={editable ? 'Shift' : null}
              selectionMode={SelectionMode.Partial}
              snapToGrid
              snapGrid={[GRID, GRID]}
              minZoom={metrics.minZoom}
              maxZoom={metrics.maxZoom}
              onlyRenderVisibleElements={graph.nodes.length > VIRTUALIZE_ABOVE}
              proOptions={{ hideAttribution: false }}
              attributionPosition="bottom-right"
              ariaLabelConfig={{
                'node.a11yDescription.default': intl.formatMessage(canvasMessages.ariaNodeDescription),
                'node.a11yDescription.keyboardDisabled': intl.formatMessage(canvasMessages.ariaNodeDescription),
                'minimap.ariaLabel': intl.formatMessage(canvasMessages.ariaMinimap),
              }}
            >
              <Background variant={BackgroundVariant.Dots} gap={metrics.grid} size={1.4} color="var(--orcon-canvas-grid)" />
              {showMinimap ? <CanvasMinimap visible={minimapVisible} onPointerEnter={() => setMinimapHover(true)} onPointerLeave={() => setMinimapHover(false)} /> : null}
            </ReactFlow>
            <ZoomControls
              containerRef={containerRef}
              onFit={() => zoomBy('fit')}
              onZoomIn={() => zoomBy('in')}
              onZoomOut={() => zoomBy('out')}
              onReset={() => zoomBy('reset')}
              onTidyUp={editable && tidyEnabled ? tidyUp : undefined}
              className="absolute bottom-3 left-3 z-20"
            />
          </CanvasContext.Provider>
          {empty ? (
            <div className="pointer-events-none absolute inset-0 flex items-center justify-center p-4">
              {editable && catalog.length > 0 ? (
                <AddFirstStep className="pointer-events-auto" onActivate={() => openPalette({ mode: 'first' })} />
              ) : (
                <EmptyState title={intl.formatMessage(canvasMessages.emptyTitle)} description={intl.formatMessage(canvasMessages.emptyDescription)} headingLevel={3} />
              )}
            </div>
          ) : null}
          {palette && editable ? (
            <NodePalettePanel
              // Nothing can be connected into a trigger, so a node inserted after another one or on a connection is never a trigger.
              items={palette.mode === 'after' || palette.mode === 'edge' ? catalog.filter((item) => item.kind !== 'trigger') : catalog}
              filter={palette.mode === 'first' ? 'triggers' : undefined}
              onPick={pickFromPalette}
              onClose={closePalette}
            />
          ) : null}
        </div>
        {outlineOpen ? (
          <div className="border-border bg-surface min-h-0 overflow-y-auto border-t p-3 lg:w-canvas-palette-w lg:shrink-0 lg:border-s lg:border-t-0">
            <CanvasOutline graph={graph} editable={editable} headingLevel={3} onFocusNode={focusNode} onDisconnect={disconnect} onDeleteNode={(id) => requestDelete([id])} />
          </div>
        ) : null}
      </div>
      {region}
      {editable ? (
        <>
          <ConnectDialog graph={graph} open={connectOpen} onOpenChange={setConnectOpen} initialSource={selectedIds[0] ?? activeId} onConnect={connectNodes} />
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
 * The flow canvas, in style A. From md up it edits: every operation (add, move, connect, delete,
 * disable, rename, pin, sticky notes, zoom, undo and redo) works from the keyboard, and by pointer
 * as a faster way to the same result. Below md it is view only: pan, zoom, and tapping a node opens
 * its state and counts. Changes are announced politely and joined so a burst is one announcement.
 */
export function WorkflowCanvas(props: WorkflowCanvasProps) {
  return (
    <ReactFlowProvider>
      <Canvas {...props} />
    </ReactFlowProvider>
  );
}

export type { StickyNoteData };
