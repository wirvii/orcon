// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { AlertDialog, Banner, Button, CommandPalette, type DetailsTab, type PaletteCommand } from '@orcon/ui';
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState, type ClipboardEvent, type KeyboardEvent } from 'react';
import { useIntl } from 'react-intl';

import { clipboardEventText, setClipboardEventText } from '../../../app/boundary/clipboard';
import type { FlowDocument } from '../model/flow-document';
import { chordKeys, EDITOR_KEYMAP, resolveEntry, type EditorAction } from '../model/keymap';
import { useEditorStore, useEditorStoreApi } from '../store/editor-context';

import { EditorCanvas, type EditorCanvasHandle } from './EditorCanvas';
import { EditorOutline } from './EditorOutline';
import { EditorTopBar } from './EditorTopBar';
import { FindNodeDialog } from './FindNodeDialog';
import { editorMessages } from './messages';
import { createNoticeChannel, NoticeContext } from './notice-channel';
import { NoticeBar } from './NoticeBar';
import { ShortcutsDialog } from './ShortcutsDialog';
import { useAutosave, type SaveFunction } from './use-autosave';
import { useEditorCommands } from './use-editor-commands';
import { EDITING_QUERY, useMediaQuery } from './use-media-query';

// The details view and its forms load when the first node is opened: they are not part of opening the editor.
const NodeDetails = lazy(() => import('./NodeDetails').then((module) => ({ default: module.NodeDetails })));
const DockedDataPanel = lazy(() => import('./NodeDetails').then((module) => ({ default: module.DockedDataPanel })));

/** From this width the node details view is a dialog over the canvas (lg); below it, from md, it docks beside the canvas. */
const DIALOG_QUERY = '(min-width: 64rem)';

export interface EditorWorkspaceProps {
  /** Writes the draft. */
  save: SaveFunction;
  /** Reads the newest draft, for "Reload the flow" after a conflict. */
  reload: () => Promise<{ document: FlowDocument; revision: string }>;
  /** True while the page shows a sample flow that is not stored on the server. */
  preview: boolean;
}

/** Actions a key can still trigger while a text field has focus: they never change what is typed. */
const WORKS_IN_TEXT: ReadonlySet<EditorAction> = new Set(['save', 'commands', 'find']);

const TEXT_FIELD = 'input, textarea, select, [contenteditable="true"], [role="textbox"], [role="combobox"]';

const detectPlatform = () => ({ mac: typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.userAgent) });

/**
 * The open flow: the top bar, the canvas, the outline, the dialogs and the one key handler that
 * resolves every editor shortcut through the keymap. Below the md breakpoint it shows the flow
 * read-only, without any editing control.
 */
export function EditorWorkspace(props: EditorWorkspaceProps) {
  const channel = useMemo(() => createNoticeChannel(), []);
  return (
    <NoticeContext.Provider value={channel}>
      <Workspace {...props} />
    </NoticeContext.Provider>
  );
}

function Workspace({ save, reload, preview }: EditorWorkspaceProps) {
  const intl = useIntl();
  const api = useEditorStoreApi();
  const editable = useMediaQuery(EDITING_QUERY);
  const dialogLayout = useMediaQuery(DIALOG_QUERY);
  const platform = useMemo(detectPlatform, []);
  const canvas = useRef<EditorCanvasHandle>(null);
  const root = useRef<HTMLDivElement>(null);
  const [session, setSession] = useState(0);
  const [outlineOpen, setOutlineOpen] = useState(false);
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const [findOpen, setFindOpen] = useState(false);
  const [commandsOpen, setCommandsOpen] = useState(false);
  const [confirmReload, setConfirmReload] = useState(false);
  const [details, setDetails] = useState<{ id: string; tab: DetailsTab } | undefined>();
  const [previewShown, setPreviewShown] = useState(true);
  const { status, saveNow } = useAutosave(save, session);

  // A dialog opened from the keyboard has no button to give focus back to: when it closes, focus returns
  // to what had it, or to the canvas, so the shortcuts keep working.
  const lastFocus = useRef<HTMLElement | null>(null);
  const dialogOpen = shortcutsOpen || findOpen || commandsOpen || confirmReload;
  const wasDialogOpen = useRef(false);
  // The node a dialog chose (Find): focus goes there once the dialog is gone and has given its focus back.
  const picked = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (wasDialogOpen.current && !dialogOpen) {
      const frame = requestAnimationFrame(() => {
        const choice = picked.current;
        picked.current = undefined;
        if (choice) {
          canvas.current?.focusSoon(choice);
          return;
        }
        const active = document.activeElement;
        if (active && active !== document.body) return;
        const back = lastFocus.current?.isConnected ? lastFocus.current : canvas.current?.region();
        back?.focus({ preventScroll: true });
      });
      wasDialogOpen.current = dialogOpen;
      return () => {
        cancelAnimationFrame(frame);
      };
    }
    wasDialogOpen.current = dialogOpen;
    return undefined;
  }, [dialogOpen]);

  // Opening the outline from the keyboard (O, the command palette) moves focus into it, so a screen reader reads it; Escape closes it and gives focus back.
  const outlineRef = useRef<HTMLElement>(null);
  const focusOutline = useRef(false);
  const beforeOutline = useRef<HTMLElement | null>(null);
  useEffect(() => {
    if (outlineOpen && focusOutline.current) {
      focusOutline.current = false;
      outlineRef.current?.focus();
    }
  }, [outlineOpen]);
  const closeOutline = () => {
    setOutlineOpen(false);
    const back = beforeOutline.current;
    requestAnimationFrame(() => {
      (back?.isConnected ? back : canvas.current?.region())?.focus({ preventScroll: true });
    });
  };
  const toggleOutlineFromKeyboard = () => {
    if (outlineOpen) {
      closeOutline();
      return;
    }
    beforeOutline.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    focusOutline.current = true;
    setOutlineOpen(true);
  };

  // The details view needs the md breakpoint: on a phone the canvas is view-only and shows its own status panel.
  const openDetails = useCallback((id: string) => {
    setDetails((current) => ({ id, tab: current?.id === id ? current.tab : 'parameters' }));
  }, []);
  const closeDetails = useCallback(() => {
    setDetails(undefined);
  }, []);
  useEffect(() => {
    if (!editable) setDetails(undefined);
  }, [editable]);
  const detailsId = details?.id;
  const returnFocus = useCallback(() => {
    if (detailsId) canvas.current?.focusSoon(detailsId);
  }, [detailsId]);

  const commands = useEditorCommands({
    canvas,
    openFind: () => {
      setFindOpen(true);
    },
    openShortcuts: () => {
      setShortcutsOpen(true);
    },
    openCommands: () => {
      setCommandsOpen(true);
    },
    toggleOutline: toggleOutlineFromKeyboard,
    save: saveNow,
  });

  const nodeNames = useEditorStore((state) => state.document.nodes);
  const found = useMemo(() => nodeNames.map((node) => ({ id: node.id, name: node.name })), [nodeNames]);

  const paletteCommands = useMemo<PaletteCommand[]>(() => {
    const keys = (action: EditorAction) => {
      const chord = EDITOR_KEYMAP.find((entry) => entry.action === action)?.chords[0];
      return chord ? chordKeys(chord, platform) : undefined;
    };
    const edit = intl.formatMessage(editorMessages.cmdGroupEdit);
    const view = intl.formatMessage(editorMessages.cmdGroupView);
    return [
      { id: 'add-node', label: intl.formatMessage(editorMessages.addNode), group: edit, shortcut: keys('add-node'), onSelect: commands.addNode },
      { id: 'add-note', label: intl.formatMessage(editorMessages.addNote), group: edit, shortcut: keys('add-note'), onSelect: commands.addNote },
      { id: 'undo', label: intl.formatMessage(editorMessages.undo), group: edit, shortcut: keys('undo'), onSelect: commands.undo },
      { id: 'redo', label: intl.formatMessage(editorMessages.redo), group: edit, shortcut: keys('redo'), onSelect: commands.redo },
      { id: 'select-all', label: intl.formatMessage(editorMessages.cmdSelectAll), group: edit, shortcut: keys('select-all'), onSelect: commands.selectAll },
      { id: 'duplicate', label: intl.formatMessage(editorMessages.cmdDuplicate), group: edit, shortcut: keys('duplicate'), onSelect: commands.duplicate },
      { id: 'copy', label: intl.formatMessage(editorMessages.cmdCopy), group: edit, onSelect: () => void commands.copy() },
      { id: 'tidy-up', label: intl.formatMessage(editorMessages.tidyUp), group: edit, shortcut: keys('tidy-up'), onSelect: () => void commands.tidyUp() },
      { id: 'save', label: intl.formatMessage(editorMessages.saveNow), group: edit, shortcut: keys('save'), onSelect: saveNow },
      { id: 'find', label: intl.formatMessage(editorMessages.find), group: view, shortcut: keys('find'), onSelect: () => { setFindOpen(true); } },
      { id: 'fit', label: intl.formatMessage(editorMessages.cmdFit), group: view, shortcut: ['1'], onSelect: () => canvas.current?.fit() },
      { id: 'outline', label: intl.formatMessage(editorMessages.showOutline), group: view, shortcut: keys('outline'), onSelect: () => { commands.run('outline'); } },
      { id: 'shortcuts', label: intl.formatMessage(editorMessages.shortcuts), group: view, shortcut: keys('shortcuts'), onSelect: () => { setShortcutsOpen(true); } },
    ];
  }, [commands, intl, platform, saveNow]);

  /** One handler for every shortcut of the editor: the keymap says which action a key press is. */
  const onKeyDownCapture = (event: KeyboardEvent<HTMLDivElement>) => {
    const handle = canvas.current;
    const target = event.target;
    if (!editable || !handle || handle.isDispatching() || !(target instanceof HTMLElement) || !root.current?.contains(target)) return;
    const inCanvas = handle.region()?.contains(target) ?? false;
    const entry = resolveEntry(event, inCanvas ? ['editor', 'canvas'] : ['editor'], platform, { handler: 'editor', includeLater: true });
    if (!entry) return;
    if (target.closest(TEXT_FIELD) && !WORKS_IN_TEXT.has(entry.action)) return;
    // Copy, cut and paste arrive as clipboard events, which the browser also raises for the menu.
    if (entry.action === 'clipboard') return;
    event.preventDefault();
    event.stopPropagation();
    if (entry.action === 'undo') commands.undo();
    else if (entry.action === 'redo') commands.redo();
    else commands.run(entry.action);
  };

  /** Whether a clipboard event is about the canvas: copying text out of a field is the browser's business. */
  const aboutCanvas = (event: ClipboardEvent<HTMLDivElement>): boolean => {
    const target = event.target;
    return editable && target instanceof HTMLElement && (canvas.current?.region()?.contains(target) ?? false) && target.closest(TEXT_FIELD) === null;
  };

  const reloadFlow = async () => {
    const draft = await reload();
    api.getState().replaceDocument(draft.document, draft.revision);
    setSession((value) => value + 1);
  };

  /** Escape closes the docked details view, like "Back to canvas", unless a menu or dialog inside it took the key. */
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'Escape' || event.defaultPrevented || !details || dialogLayout) return;
    if (event.target instanceof HTMLElement && event.target.closest('[data-node-details="docked"]')) {
      event.preventDefault();
      closeDetails();
      returnFocus();
    }
  };

  const showDocked = details !== undefined && editable && !dialogLayout;

  return (
    <div
      ref={root}
      data-editor-workspace
      className="bg-surface border-border flex h-full min-h-0 flex-1 flex-col overflow-hidden rounded-lg border"
      onKeyDownCapture={onKeyDownCapture}
      onKeyDown={onKeyDown}
      onFocus={(event) => {
        // Portals of the dialogs bubble their focus events here too; only the editor's own elements count.
        if (event.target instanceof HTMLElement && root.current?.contains(event.target)) lastFocus.current = event.target;
      }}
      onCopy={(event) => {
        if (!aboutCanvas(event)) return;
        const text = commands.copyText();
        if (text !== undefined && setClipboardEventText(event, text)) event.preventDefault();
      }}
      onCut={(event) => {
        if (!aboutCanvas(event)) return;
        const text = commands.copyText();
        if (text === undefined || !setClipboardEventText(event, text)) return;
        event.preventDefault();
        commands.cut();
      }}
      onPaste={(event) => {
        if (!aboutCanvas(event)) return;
        event.preventDefault();
        commands.paste(clipboardEventText(event));
      }}
    >
      <EditorTopBar
        editable={editable}
        status={status}
        outlineOpen={outlineOpen}
        commands={commands}
        onToggleOutline={() => {
          setOutlineOpen((open) => !open);
        }}
        onFind={() => {
          setFindOpen(true);
        }}
        onCommands={() => {
          setCommandsOpen(true);
        }}
        onShortcuts={() => {
          setShortcutsOpen(true);
        }}
        onSave={saveNow}
      />
      <div className="flex flex-col gap-2 empty:hidden px-4 pt-2">
        {preview && previewShown && editable ? (
          <Banner
            tone="info"
            title={intl.formatMessage(editorMessages.previewTitle)}
            action={
              <Button
                variant="ghost"
                onClick={() => {
                  setPreviewShown(false);
                }}
              >
                {intl.formatMessage(editorMessages.dismiss)}
              </Button>
            }
          >
            {intl.formatMessage(editorMessages.previewText)}
          </Banner>
        ) : null}
        {status === 'conflict' ? (
          <Banner
            tone="danger"
            title={intl.formatMessage(editorMessages.conflictTitle)}
            action={
              <Button
                variant="secondary"
                onClick={() => {
                  setConfirmReload(true);
                }}
              >
                {intl.formatMessage(editorMessages.conflictReload)}
              </Button>
            }
          >
            {intl.formatMessage(editorMessages.conflictText)}
          </Banner>
        ) : null}
      </div>
      <div className="relative flex min-h-0 flex-1 flex-col">
        <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
          <div className="flex min-h-0 min-w-0 flex-1">
            <EditorCanvas ref={canvas} onOpenDetails={openDetails} syncNodes={details !== undefined} />
            {showDocked ? (
              <Suspense fallback={null}>
                <NodeDetails
                  nodeId={details.id}
                  layout="docked"
                  tab={details.tab}
                  onTabChange={(tab) => {
                    setDetails({ id: details.id, tab });
                  }}
                  onClose={closeDetails}
                  onReturnFocus={returnFocus}
                />
              </Suspense>
            ) : null}
          </div>
          <NoticeBar />
          {outlineOpen ? (
            <aside
              ref={outlineRef}
              tabIndex={-1}
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  event.stopPropagation();
                  closeOutline();
                }
              }}
              aria-label={intl.formatMessage(editorMessages.outlineTitle)}
              className="border-border bg-surface min-h-0 overflow-y-auto border-t p-3 lg:w-canvas-palette-w lg:shrink-0 lg:border-s lg:border-t-0">
              <EditorOutline
                editable={editable}
                onFocusNode={(id) => canvas.current?.focusSoon(id)}
                onDisconnect={(edgeId) => api.getState().disconnect(edgeId)}
                onDeleteNode={(id) => api.getState().removeNodes([id])}
              />
            </aside>
          ) : null}
        </div>
        {showDocked ? (
          <Suspense fallback={null}>
            <DockedDataPanel nodeId={details.id} />
          </Suspense>
        ) : null}
      </div>
      {details !== undefined && editable && dialogLayout ? (
        <Suspense fallback={null}>
          <NodeDetails
            nodeId={details.id}
            layout="dialog"
            tab={details.tab}
            onTabChange={(tab) => {
              setDetails({ id: details.id, tab });
            }}
            onClose={closeDetails}
            onReturnFocus={returnFocus}
          />
        </Suspense>
      ) : null}
      <ShortcutsDialog open={shortcutsOpen} onOpenChange={setShortcutsOpen} platform={platform} />
      <FindNodeDialog open={findOpen} onOpenChange={setFindOpen} nodes={found} onPick={(id) => {
          picked.current = id;
        }}
      />
      <CommandPalette commands={paletteCommands} open={commandsOpen} onOpenChange={setCommandsOpen} enableShortcut={false} />
      <AlertDialog
        open={confirmReload}
        onOpenChange={setConfirmReload}
        title={intl.formatMessage(editorMessages.conflictConfirmTitle)}
        impact={intl.formatMessage(editorMessages.conflictConfirmImpact)}
        confirmLabel={intl.formatMessage(editorMessages.conflictConfirmAction)}
        onConfirm={() => {
          setConfirmReload(false);
          void reloadFlow();
        }}
      />
    </div>
  );
}
