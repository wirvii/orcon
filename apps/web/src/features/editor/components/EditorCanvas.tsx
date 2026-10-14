// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { WorkflowCanvas, type Graph, type WorkflowCanvasHandle } from '@orcon/ui';
import { useCallback, useEffect, useImperativeHandle, useLayoutEffect, useMemo, useRef, type Ref } from 'react';
import { useIntl } from 'react-intl';

import { markEditorReady } from '../../../app/perf/marks';
import type { EditReport } from '../model/canvas-edit';
import { clientToFlow, graphAgrees, parseViewportTransform, toCanvasGraph, type CanvasGraph } from '../model/canvas-graph';
import type { Point } from '../model/flow-document';
import { useEditorStore, useEditorStoreApi } from '../store/editor-context';

import { editorMessages } from './messages';
import { useNoticeChannel } from './notice-channel';
import { useDescribeNode, useNodeCatalog } from './use-node-catalog';

export interface EditorCanvasHandle {
  /**
   * Takes the canvas back or forward one step. While the canvas remembers the step itself it does it
   * in place and keeps the view; otherwise the store does it and the canvas is rebuilt.
   */
  history: (direction: 'undo' | 'redo') => void;
  /** Hands a key press to the canvas, as if it were typed there: the toolbar uses the same keys as the keyboard. */
  press: (key: string, modifiers?: { shiftKey?: boolean; primary?: boolean }) => void;
  /** True while `press` hands a key to the canvas, so the key handler of the editor skips it. */
  isDispatching: () => boolean;
  /** The ids of the nodes and sticky notes that are selected. */
  selectedIds: () => string[];
  /** Moves keyboard focus to a node (after the canvas is rebuilt, as soon as it is on screen). */
  focusSoon: (id: string) => void;
  /** Opens the list of nodes to add. */
  openPalette: () => void;
  /** Shows the whole flow. */
  fit: () => void;
  /** Where the pointer is on the canvas, or the middle of what is shown: the canvas coordinate new things go to. */
  anchorPoint: () => Point;
  /** The element the canvas keyboard handler listens on. */
  region: () => HTMLElement | null;
}

const isMac = () => typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.userAgent);

/**
 * The design system canvas, connected to the editor store.
 *
 * `WorkflowCanvas` keeps its own copy of the graph and reports each change as a whole new graph. This
 * component turns every such change into a store action (one history entry), so the document in the
 * store is the source of truth. When the store changes the document by itself (paste, tidy up, undo
 * of something the canvas did not do) the canvas is built again from it (`key`). That also happens
 * when the document refuses part of what the canvas did. See the report for the primitive that would
 * make this simpler: a canvas that takes its graph as a prop.
 */
export function EditorCanvas({ ref }: { ref?: Ref<EditorCanvasHandle> | undefined }) {
  const intl = useIntl();
  const api = useEditorStoreApi();
  const channel = useNoticeChannel();
  const epoch = useEditorStore((state) => state.canvasEpoch);
  const catalog = useNodeCatalog();
  const describe = useDescribeNode();

  const frame = useRef<HTMLDivElement>(null);
  const canvas = useRef<WorkflowCanvasHandle>(null);
  const pointer = useRef<Point | undefined>(undefined);
  const pendingHistory = useRef<'undo' | 'redo' | undefined>(undefined);
  const dispatching = useRef(false);
  const pendingFocus = useRef<string | undefined>(undefined);
  // How many reports of the canvas are the echo of a name this component just told it (see `adjustedNames`).
  const echoes = useRef(0);
  // True while keyboard focus is inside the canvas area; a rebuild removes the focused element, so focus is put back.
  const focusInside = useRef(false);

  // The graph the canvas starts from; it is built again each time the store asks for a rebuild.
  const initial = useMemo<CanvasGraph>(
    () => toCanvasGraph(api.getState().document, describe),
    // `epoch` is not read inside: it is the signal that the document changed behind the canvas.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- see above
    [api, describe, epoch],
  );
  // The graph the canvas last reported: the next change is compared with it.
  const last = useRef<CanvasGraph>(initial);
  useLayoutEffect(() => {
    last.current = initial;
  }, [initial]);

  const tell = useCallback(
    (report: EditReport) => {
      for (const renamed of report.renamed) {
        channel.notify('info', intl.formatMessage(editorMessages.noticeRenamed, { name: renamed.to, count: renamed.references }));
      }
      for (const { problem } of report.renameProblems) {
        channel.notify('warning', intl.formatMessage(problem === 'empty' ? editorMessages.noticeNameEmpty : editorMessages.noticeNameTaken));
      }
      for (const { problem } of report.edgeProblems) {
        const message = problem === 'cycle' ? editorMessages.noticeCycle : problem === 'into-start' ? editorMessages.noticeIntoTrigger : editorMessages.noticeConnectionRefused;
        channel.notify('warning', intl.formatMessage(message));
      }
      if (report.pinIgnored.length > 0) channel.notify('warning', intl.formatMessage(editorMessages.noticeLaterPin));
    },
    [channel, intl],
  );

  const onGraphChange = useCallback(
    (reported: Graph) => {
      const next: CanvasGraph = reported;
      const previous = last.current;
      last.current = next;
      if (echoes.current > 0) {
        // The canvas took a name the document already holds: nothing changed in the document.
        echoes.current -= 1;
        return;
      }
      const step = pendingHistory.current;
      pendingHistory.current = undefined;
      if (step) {
        // The canvas went back or forward by itself, as asked: the store follows and must agree.
        const done = step === 'undo' ? api.getState().undo({ fromCanvas: true }) : api.getState().redo({ fromCanvas: true });
        if (!done || !graphAgrees(api.getState().document, next)) api.getState().rebuildCanvas();
        return;
      }
      const report = api.getState().applyCanvasEdit(previous, next);
      tell(report);
      if (report.adjustedNames.length > 0) {
        // The canvas finishes the change it reported before it takes another one, so the names follow right after.
        setTimeout(() => {
          for (const { id, name } of report.adjustedNames) {
            echoes.current += 1;
            canvas.current?.updateNode(id, { name });
          }
        }, 0);
      }
    },
    [api, tell],
  );

  useEffect(() => {
    const element = frame.current;
    if (!element) return;
    const ready = () => element.querySelector('[data-fitted="true"]') !== null;
    if (ready()) {
      markEditorReady();
      return;
    }
    const observer = new MutationObserver(() => {
      if (!ready()) return;
      observer.disconnect();
      markEditorReady();
    });
    observer.observe(element, { attributes: true, attributeFilter: ['data-fitted'], subtree: true });
    return () => {
      observer.disconnect();
    };
  }, []);

  // A rebuild removes the element that had focus: focus goes back to the canvas, so the shortcuts keep working.
  useEffect(() => {
    if (!focusInside.current || epoch === 0) return;
    const active = document.activeElement;
    if (active && active !== document.body) return;
    frame.current?.querySelector<HTMLElement>('[data-lod]')?.focus({ preventScroll: true });
  }, [epoch]);

  // After a rebuild, focus the node the action that caused it asked for, once it is on screen.
  useEffect(() => {
    const id = pendingFocus.current;
    if (!id) return;
    pendingFocus.current = undefined;
    let tries = 0;
    let frameId = 0;
    const attempt = () => {
      const present = frame.current?.querySelector(`.react-flow__node[data-id="${CSS.escape(id)}"]`);
      if (present) canvas.current?.focusNode(id);
      else if (tries < 30) {
        tries += 1;
        frameId = requestAnimationFrame(attempt);
      }
    };
    frameId = requestAnimationFrame(attempt);
    return () => {
      cancelAnimationFrame(frameId);
    };
  }, [epoch]);

  const press = useCallback((key: string, modifiers: { shiftKey?: boolean; primary?: boolean } = {}) => {
    const region = frame.current?.querySelector<HTMLElement>('[data-lod]');
    if (!region) return;
    const mac = isMac();
    const primary = modifiers.primary === true;
    dispatching.current = true;
    try {
      region.dispatchEvent(
        new KeyboardEvent('keydown', { key, ctrlKey: primary && !mac, metaKey: primary && mac, shiftKey: modifiers.shiftKey === true, bubbles: true, cancelable: true }),
      );
    } finally {
      dispatching.current = false;
    }
  }, []);

  useImperativeHandle(
    ref,
    (): EditorCanvasHandle => ({
      press,
      history: (direction) => {
        pendingHistory.current = direction;
        press('z', { primary: true, shiftKey: direction === 'redo' });
        // The canvas reports a step it took by itself right away; when it did not, the store takes the step and the canvas is rebuilt.
        queueMicrotask(() => {
          if (pendingHistory.current !== direction) return;
          pendingHistory.current = undefined;
          const state = api.getState();
          const moved = direction === 'undo' ? state.undo() : state.redo();
          channel.notify('info', intl.formatMessage(moved ? (direction === 'undo' ? editorMessages.noticeUndone : editorMessages.noticeRedone) : direction === 'undo' ? editorMessages.noticeNothingToUndo : editorMessages.noticeNothingToRedo));
        });
      },
      isDispatching: () => dispatching.current,
      selectedIds: () =>
        Array.from(frame.current?.querySelectorAll<HTMLElement>('.react-flow__node.selected') ?? [])
          .map((element) => element.dataset['id'])
          .filter((id): id is string => id !== undefined),
      focusSoon: (id) => {
        pendingFocus.current = id;
        // When nothing rebuilds the canvas (the node is already there) focus it now.
        if (frame.current?.querySelector(`.react-flow__node[data-id="${CSS.escape(id)}"]`)) canvas.current?.focusNode(id);
      },
      openPalette: () => {
        canvas.current?.openPalette();
      },
      fit: () => {
        canvas.current?.fit();
      },
      anchorPoint: () => {
        const region = frame.current?.querySelector<HTMLElement>('[data-lod]');
        const content = frame.current?.querySelector<HTMLElement>('.react-flow__viewport');
        const viewport = content ? parseViewportTransform(content.style.transform) : undefined;
        if (!region || !viewport) return { x: 0, y: 0 };
        const box = region.getBoundingClientRect();
        const at = pointer.current;
        const inside = at !== undefined && at.x >= box.left && at.x <= box.right && at.y >= box.top && at.y <= box.bottom;
        return clientToFlow(inside ? at : { x: box.left + box.width / 2, y: box.top + box.height / 2 }, box, viewport);
      },
      region: () => frame.current?.querySelector<HTMLElement>('[data-lod]') ?? null,
    }),
    [api, channel, intl, press],
  );

  const later = useCallback(() => {
    channel.notify('warning', intl.formatMessage(editorMessages.noticeLaterRun));
  }, [channel, intl]);

  return (
    <div
      ref={frame}
      className="flex min-h-0 min-w-0 flex-1"
      data-editor-canvas
      onFocus={() => {
        focusInside.current = true;
      }}
      onBlur={(event) => {
        // An element that is removed (a rebuild) does not take focus anywhere: that is not leaving the canvas.
        if (!event.target.isConnected) return;
        if (!(event.relatedTarget instanceof Node) || !event.currentTarget.contains(event.relatedTarget)) focusInside.current = false;
      }}
      onPointerMove={(event) => {
        pointer.current = { x: event.clientX, y: event.clientY };
      }}
    >
      <WorkflowCanvas key={epoch} ref={canvas} defaultGraph={initial} catalog={catalog} onGraphChange={onGraphChange} onRunStep={later} onExecuteWorkflow={later} className="min-w-0 flex-1" />
    </div>
  );
}
