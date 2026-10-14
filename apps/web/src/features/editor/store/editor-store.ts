// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { applyPatches, enablePatches, produceWithPatches, type Patch } from 'immer';
import { createStore, type StoreApi } from 'zustand/vanilla';

import { applyCanvasEdit, type EditReport } from '../model/canvas-edit';
import type { CanvasGraph } from '../model/canvas-graph';
import {
  addNodeOfType,
  addStickyNote,
  applyPositions,
  connect,
  disconnect,
  moveNodes,
  removeNodes,
  renameNode,
  setDisabled,
  updateStickyNote,
  type EdgeProblem,
  type NotePatch,
  type RenameProblem,
} from '../model/document-ops';
import type { FlowDocument, Point } from '../model/flow-document';
import { buildFragment, pasteFragment, type FlowFragment, type PasteOptions, type PasteResult } from '../model/fragment';
import { typeDefBySlug } from '../model/node-types';

enablePatches();

/** History keeps this many entries (SPEC-014 R-ED-UNDO-01, orcon-web ST-5). */
export const HISTORY_LIMIT = 100;

export type ActionName = 'addNode' | 'removeNodes' | 'connect' | 'disconnect' | 'moveNodes' | 'renameNode' | 'setDisabled' | 'addNote' | 'updateNote' | 'paste' | 'tidyUp' | 'canvasEdit';

/** One undoable user action: the patches that make it and the patches that take it back. */
export interface HistoryEntry {
  readonly name: ActionName;
  readonly patches: readonly Patch[];
  readonly inverse: readonly Patch[];
  /** The selection when the action was made and after it, restored by undo and redo. */
  readonly selectionBefore: readonly string[];
  readonly selectionAfter: readonly string[];
  /** The canvas already shows the result of this action, so undoing it from the canvas needs no rebuild. */
  readonly fromCanvas: boolean;
}

export interface EditorState {
  /** The working copy of the draft (`orcon/flow@1`). */
  readonly document: FlowDocument;
  /** The revision of the draft the working copy started from; saving sends it as `If-Match`. */
  readonly baseRevision: string;
  readonly selection: readonly string[];
  readonly history: { readonly past: readonly HistoryEntry[]; readonly future: readonly HistoryEntry[] };
  /** The last text copied, kept when the system clipboard refuses the write. */
  readonly clipboardFallback: string | null;
  /** Counts every change of the document; autosave compares it with `savedVersion`. */
  readonly version: number;
  readonly savedVersion: number;
  /** Bumps when the canvas must be rebuilt from the document (it does not hold the change itself). */
  readonly canvasEpoch: number;

  select: (ids: readonly string[]) => void;
  addNode: (slug: string, at: Point, name: string) => string | undefined;
  removeNodes: (ids: readonly string[]) => string[];
  connect: (source: string, port: string, target: string) => { edgeId: string } | { problem: EdgeProblem };
  disconnect: (edgeId: string) => boolean;
  moveNodes: (ids: readonly string[], dx: number, dy: number) => void;
  renameNode: (id: string, name: string) => { references: number } | { problem: RenameProblem };
  setDisabled: (id: string, disabled: boolean) => void;
  addNote: (at: Point) => string;
  updateNote: (id: string, patch: NotePatch) => void;
  /** Adds a fragment to the flow; one history entry. */
  paste: (fragment: FlowFragment, options: PasteOptions) => PasteResult;
  /** Copies the items and pastes them offset by two grid steps; one history entry. */
  duplicate: (ids: readonly string[]) => PasteResult;
  /** Moves items to the positions a layout computed; one history entry. */
  applyLayout: (positions: ReadonlyMap<string, Point>) => void;
  /** Applies what the canvas changed. A change the document refuses asks for a rebuild of the canvas. */
  applyCanvasEdit: (previous: CanvasGraph, next: CanvasGraph) => EditReport;
  /** Undoes the last action. `fromCanvas`: the canvas already went back, so it needs no rebuild. */
  undo: (options?: { fromCanvas?: boolean }) => boolean;
  redo: (options?: { fromCanvas?: boolean }) => boolean;
  setClipboardFallback: (text: string | null) => void;
  /** Replaces the working copy with a draft loaded from the server; history starts again (R-ED-UNDO-01). */
  replaceDocument: (document: FlowDocument, revision: string) => void;
  /** Records that the draft at `version` is saved as `revision`. */
  markSaved: (version: number, revision: string) => void;
  /** Rebuilds the canvas from the document. */
  rebuildCanvas: () => void;
}

export type EditorStore = StoreApi<EditorState>;

export interface EditorInit {
  document: FlowDocument;
  revision: string;
}

interface CommitOptions<T> {
  fromCanvas?: boolean;
  /** The selection after the action, or a function of what the action returned. */
  selection?: readonly string[] | ((value: T | undefined) => readonly string[]);
  /** False when the canvas already shows the change. */
  rebuild?: boolean;
}

/**
 * One store per open editor (orcon-web ST-3): no module-level instance. The document changes only
 * through the named actions below, each of them one history entry made of Immer patches.
 */
export function createEditorStore(initial: EditorInit): EditorStore {
  return createStore<EditorState>()((set, get) => {
    /** Runs `recipe` on a draft of the document and records the result as one history entry. */
    function commit<T>(name: ActionName, recipe: (draft: FlowDocument) => T, options: CommitOptions<T> = {}): { done: boolean; value: T | undefined } {
      const state = get();
      const held: { value?: T } = {};
      const [document, patches, inverse] = produceWithPatches(state.document, (draft) => {
        held.value = recipe(draft);
      });
      if (patches.length === 0) return { done: false, value: held.value };
      const chosen = typeof options.selection === 'function' ? options.selection(held.value) : options.selection;
      const selectionAfter = chosen ?? state.selection;
      const entry: HistoryEntry = { name, patches, inverse, selectionBefore: state.selection, selectionAfter, fromCanvas: options.fromCanvas === true };
      set({
        document,
        selection: selectionAfter,
        history: { past: [...state.history.past, entry].slice(-HISTORY_LIMIT), future: [] },
        version: state.version + 1,
        canvasEpoch: state.canvasEpoch + (options.rebuild === false ? 0 : 1),
      });
      return { done: true, value: held.value };
    }

    function travel(direction: 'undo' | 'redo', options: { fromCanvas?: boolean } = {}): boolean {
      const state = get();
      const { past, future } = state.history;
      const entry = direction === 'undo' ? past[past.length - 1] : future[0];
      if (!entry) return false;
      set({
        document: applyPatches(state.document, direction === 'undo' ? entry.inverse : entry.patches),
        selection: direction === 'undo' ? entry.selectionBefore : entry.selectionAfter,
        history: direction === 'undo' ? { past: past.slice(0, -1), future: [entry, ...future] } : { past: [...past, entry], future: future.slice(1) },
        version: state.version + 1,
        canvasEpoch: state.canvasEpoch + (options.fromCanvas === true && entry.fromCanvas ? 0 : 1),
      });
      return true;
    }

    return {
      document: initial.document,
      baseRevision: initial.revision,
      selection: [],
      history: { past: [], future: [] },
      clipboardFallback: null,
      version: 0,
      savedVersion: 0,
      canvasEpoch: 0,

      select: (ids) => {
        set({ selection: [...ids] });
      },

      addNode: (slug, at, name) => {
        const def = typeDefBySlug(slug);
        if (!def) return undefined;
        const { value } = commit('addNode', (draft) => addNodeOfType(draft, def, at, name), { selection: (id) => (id ? [id] : []) });
        return value;
      },

      removeNodes: (ids) => {
        const { value } = commit('removeNodes', (draft) => removeNodes(draft, ids), { selection: [] });
        return value ?? [];
      },

      connect: (source, port, target) => {
        const { value } = commit('connect', (draft) => connect(draft, source, port, target));
        if (value && 'edge' in value) return { edgeId: value.edge.id };
        return { problem: value && 'problem' in value ? value.problem : 'unknown-node' };
      },

      disconnect: (edgeId) => commit('disconnect', (draft) => disconnect(draft, edgeId)).done,

      moveNodes: (ids, dx, dy) => {
        commit('moveNodes', (draft) => {
          moveNodes(draft, ids, dx, dy);
        });
      },

      renameNode: (id, name) => {
        const { value } = commit('renameNode', (draft) => renameNode(draft, id, name));
        return value ?? { problem: 'missing' };
      },

      setDisabled: (id, disabled) => {
        commit('setDisabled', (draft) => {
          setDisabled(draft, id, disabled);
        });
      },

      addNote: (at) => {
        const { value } = commit('addNote', (draft) => addStickyNote(draft, at), { selection: (id) => (id ? [id] : []) });
        return value ?? '';
      },

      updateNote: (id, patch) => {
        commit('updateNote', (draft) => updateStickyNote(draft, id, patch));
      },

      paste: (fragment, options) => {
        const { value } = commit('paste', (draft) => pasteFragment(draft, fragment, options), {
          selection: (result) => (result ? [...result.nodeIds, ...result.noteIds] : []),
        });
        return value ?? { nodeIds: [], noteIds: [], droppedEdges: 0, placeholders: 0 };
      },

      duplicate: (ids) => get().paste(buildFragment(get().document, new Set(ids)), { offset: { x: 40, y: 40 } }),

      applyLayout: (positions) => {
        commit('tidyUp', (draft) => {
          applyPositions(draft, positions);
        });
      },

      applyCanvasEdit: (previous, next) => {
        const held: { report?: EditReport } = {};
        commit(
          'canvasEdit',
          (draft) => {
            held.report = applyCanvasEdit(draft, previous, next);
          },
          { fromCanvas: true, rebuild: false },
        );
        const report = held.report ?? { renamed: [], renameProblems: [], edgeProblems: [], adjustedNames: [], pinIgnored: [], divergent: false };
        if (report.divergent) set({ canvasEpoch: get().canvasEpoch + 1 });
        return report;
      },

      undo: (options) => travel('undo', options),
      redo: (options) => travel('redo', options),

      setClipboardFallback: (text) => {
        set({ clipboardFallback: text });
      },

      replaceDocument: (document, revision) => {
        set({ document, baseRevision: revision, selection: [], history: { past: [], future: [] }, version: 0, savedVersion: 0, canvasEpoch: get().canvasEpoch + 1 });
      },

      markSaved: (version, revision) => {
        set({ savedVersion: version, baseRevision: revision });
      },

      rebuildCanvas: () => {
        set({ canvasEpoch: get().canvasEpoch + 1 });
      },
    };
  });
}
