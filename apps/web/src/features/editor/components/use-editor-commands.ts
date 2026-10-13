// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useMemo, useRef, type RefObject } from 'react';
import { useIntl, type MessageDescriptor } from 'react-intl';

import { parsePastedJson, writeClipboardText } from '../../../app/boundary/clipboard';
import { nodeBox, toCanvasGraph } from '../model/canvas-graph';
import { detectClipboardFormat, MAX_CLIPBOARD_BYTES } from '../model/clipboard-format';
import type { EditorAction } from '../model/keymap';
import { buildFragment, flowFragmentSchema, fragmentSize, MAX_FRAGMENT_NODES } from '../model/fragment';
import { computeLayout } from '../model/layout-client';
import { useEditorStoreApi } from '../store/editor-context';

import type { EditorCanvasHandle } from './EditorCanvas';
import { editorMessages } from './messages';
import { useNoticeChannel } from './notice-channel';

export interface CommandDeps {
  canvas: RefObject<EditorCanvasHandle | null>;
  openFind: () => void;
  openShortcuts: () => void;
  openCommands: () => void;
  toggleOutline: () => void;
  /** Saves the draft now. */
  save: () => void;
}

export interface EditorCommands {
  /** Runs an action of the keymap that needs the editor (not the canvas). */
  run: (action: EditorAction) => void;
  undo: () => void;
  redo: () => void;
  addNode: () => void;
  addNote: () => void;
  selectAll: () => void;
  tidyUp: () => Promise<void>;
  duplicate: () => void;
  /** The text to put on the clipboard for the selection, or undefined (and a notice) when nothing is selected. */
  copyText: () => string | undefined;
  /** Deletes the selection after it was copied. */
  cut: () => void;
  /** Adds the nodes in a pasted text, or says why it cannot. */
  paste: (text: string) => void;
  /** Copies the selection with the clipboard API, for the command palette and the buttons. */
  copy: () => Promise<void>;
}

/**
 * What the editor does besides what the canvas does: history that keeps the view, the clipboard,
 * tidy up, duplicate, and the actions the toolbar, the command palette and the keyboard share. Each
 * one reads the store when it runs; the returned object never changes.
 */
export function useEditorCommands(deps: CommandDeps): EditorCommands {
  const intl = useIntl();
  const api = useEditorStoreApi();
  const channel = useNoticeChannel();
  const latest = useRef({ deps, intl, channel });
  latest.current = { deps, intl, channel };

  return useMemo<EditorCommands>(() => {
    const tell = (tone: 'info' | 'warning', message: string) => {
      latest.current.channel.notify(tone, message);
    };
    const say = (descriptor: MessageDescriptor, values?: Record<string, string | number>) => latest.current.intl.formatMessage(descriptor, values);
    const canvas = () => latest.current.deps.canvas.current;

    const selection = (): string[] => {
      const ids = canvas()?.selectedIds() ?? [];
      api.getState().select(ids);
      return ids;
    };

    const copyText = (): string | undefined => {
      const ids = selection();
      if (ids.length === 0) {
        tell('warning', say(editorMessages.noticeNothingSelected));
        return undefined;
      }
      const fragment = buildFragment(api.getState().document, new Set(ids));
      const text = JSON.stringify(fragment);
      api.getState().setClipboardFallback(text);
      tell('info', say(editorMessages.noticeCopied, { count: fragmentSize(fragment) }));
      return text;
    };

    const paste = (text: string) => {
      const read = parsePastedJson(text, MAX_CLIPBOARD_BYTES);
      if (!read.ok) {
        tell('warning', say(read.reason === 'too-large' ? editorMessages.noticeTooLarge : editorMessages.noticeNothingToPaste));
        return;
      }
      const format = detectClipboardFormat(read.value);
      if (format === 'n8n-workflow') {
        tell('warning', say(editorMessages.noticeN8n));
        return;
      }
      if (format === 'unknown') {
        tell('warning', say(editorMessages.noticeNothingToPaste));
        return;
      }
      const parsed = flowFragmentSchema.safeParse(read.value);
      if (!parsed.success) {
        tell('warning', say(editorMessages.noticeInvalidFragment));
        return;
      }
      if (parsed.data.nodes.length > MAX_FRAGMENT_NODES) {
        tell('warning', say(editorMessages.noticeFragmentTooBig, { max: MAX_FRAGMENT_NODES }));
        return;
      }
      const result = api.getState().paste(parsed.data, { at: canvas()?.anchorPoint() ?? { x: 0, y: 0 } });
      const first = result.nodeIds[0] ?? result.noteIds[0];
      if (first) canvas()?.focusSoon(first);
      tell('info', say(editorMessages.noticePasted, { count: result.nodeIds.length + result.noteIds.length }));
    };

    const duplicate = () => {
      const ids = selection();
      if (ids.length === 0) {
        tell('warning', say(editorMessages.noticeNothingSelected));
        return;
      }
      const result = api.getState().duplicate(ids);
      const first = result.nodeIds[0] ?? result.noteIds[0];
      if (first) canvas()?.focusSoon(first);
    };

    const tidyUp = async () => {
      const state = api.getState();
      const graph = toCanvasGraph(state.document);
      const chosen = new Set(selection());
      // With two or more nodes selected only those are tidied; otherwise the whole flow is.
      const only = graph.nodes.filter((node) => chosen.has(node.id)).length > 1 ? chosen : undefined;
      const inScope = (id: string) => only === undefined || only.has(id);
      const nodes = graph.nodes.filter((node) => inScope(node.id));
      if (nodes.length === 0) return;
      const positions = await computeLayout({
        nodes: nodes.map((node) => ({ id: node.id, ...nodeBox(node), position: node.position })),
        edges: graph.edges
          .filter((edge) => edge.loopBack !== true && inScope(edge.source) && inScope(edge.target))
          .map((edge) => ({ from: edge.source, to: edge.target, port: Math.max(0, nodes.find((node) => node.id === edge.source)?.outputs.findIndex((port) => port.id === edge.sourceOutput) ?? 0) })),
        notes: (graph.notes ?? []).filter((note) => inScope(note.id)).map((note) => ({ id: note.id, ...note.size, position: note.position })),
      });
      api.getState().applyLayout(positions);
      tell('info', say(editorMessages.noticeTidied, { count: positions.size }));
    };

    return {
      undo: () => {
        canvas()?.history('undo');
      },
      redo: () => {
        canvas()?.history('redo');
      },
      addNode: () => {
        canvas()?.openPalette();
      },
      addNote: () => {
        // The canvas adds the note where the focus is and opens it for typing.
        canvas()?.press('S', { shiftKey: true });
      },
      selectAll: () => {
        canvas()?.press('a', { primary: true });
      },
      tidyUp,
      duplicate,
      copyText,
      cut: () => {
        const ids = selection();
        if (ids.length > 0) api.getState().removeNodes(ids);
      },
      paste,
      copy: async () => {
        const text = copyText();
        if (text !== undefined && !(await writeClipboardText(text))) tell('warning', say(editorMessages.noticeCopyFailed));
      },
      run: (action) => {
        const deps = latest.current.deps;
        switch (action) {
          case 'tidy-up':
            void tidyUp();
            break;
          case 'duplicate':
            duplicate();
            break;
          case 'save':
            deps.save();
            break;
          case 'find':
            deps.openFind();
            break;
          case 'commands':
            deps.openCommands();
            break;
          case 'outline':
            deps.toggleOutline();
            break;
          case 'shortcuts':
            deps.openShortcuts();
            break;
          case 'pin':
            tell('warning', say(editorMessages.noticeLaterPin));
            break;
          case 'test-flow':
          case 'test-to-node':
          case 'test-node':
            tell('warning', say(editorMessages.noticeLaterRun));
            break;
          default:
            break;
        }
      },
    };
  }, [api]);
}
