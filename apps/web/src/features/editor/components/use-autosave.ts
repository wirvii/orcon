// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useCallback, useEffect, useRef, useState } from 'react';

import { createAutosave, type Autosave, type SaveOutcome, type SaveStatus } from '../model/autosave';
import type { FlowDocument } from '../model/flow-document';
import { useEditorStoreApi } from '../store/editor-context';

export type SaveFunction = (input: { document: FlowDocument; ifMatch: string }) => Promise<{ kind: 'saved'; revision: string } | { kind: 'conflict' }>;

/**
 * Keeps the draft saved: it watches the store, saves with the revision the working copy started
 * from, and reports the state for the save indicator. A change made while a save is in flight is
 * saved next, because the store counts versions and only the version that was sent is marked saved.
 * `resetKey` starts it again (after a reload that replaced the working copy).
 */
export function useAutosave(save: SaveFunction, resetKey: number): { status: SaveStatus; saveNow: () => void } {
  const api = useEditorStoreApi();
  const [status, setStatus] = useState<SaveStatus>('saved');
  const controller = useRef<Autosave | undefined>(undefined);
  const latestSave = useRef(save);
  latestSave.current = save;

  useEffect(() => {
    const autosave = createAutosave({
      isDirty: () => api.getState().version !== api.getState().savedVersion,
      onStatus: setStatus,
      save: async (): Promise<SaveOutcome> => {
        const { document, baseRevision, version } = api.getState();
        const result = await latestSave.current({ document, ifMatch: baseRevision });
        if (result.kind === 'conflict') return 'conflict';
        api.getState().markSaved(version, result.revision);
        return 'saved';
      },
    });
    controller.current = autosave;
    setStatus('saved');
    if (api.getState().version !== api.getState().savedVersion) autosave.notifyChange();
    const unsubscribe = api.subscribe((state, previous) => {
      if (state.version !== previous.version) autosave.notifyChange();
    });
    // Leaving the page with changes that are not saved asks first (the browser shows its own wording).
    const warn = (event: BeforeUnloadEvent) => {
      if (api.getState().version === api.getState().savedVersion) return;
      event.preventDefault();
    };
    window.addEventListener('beforeunload', warn);
    return () => {
      window.removeEventListener('beforeunload', warn);
      unsubscribe();
      // Closing the editor saves what is left, so a change made in the last two seconds is not lost.
      if (api.getState().version !== api.getState().savedVersion && autosave.status() !== 'conflict') void autosave.flush();
      autosave.dispose();
      controller.current = undefined;
    };
  }, [api, resetKey]);

  const saveNow = useCallback(() => {
    void controller.current?.flush();
  }, []);

  return { status, saveNow };
}
