// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useContext, type ReactNode } from 'react';
import { useStore } from 'zustand';

import type { EditorState, EditorStore } from './editor-store';

const EditorStoreContext = createContext<EditorStore | undefined>(undefined);

/** Provides the store of the open editor. The context carries the store itself, which never changes (orcon-web ST-11). */
export function EditorStoreProvider({ store, children }: { store: EditorStore; children: ReactNode }) {
  return <EditorStoreContext.Provider value={store}>{children}</EditorStoreContext.Provider>;
}

/** The store of the open editor, for code that reads or acts outside a render (event handlers, effects). */
export function useEditorStoreApi(): EditorStore {
  const store = useContext(EditorStoreContext);
  if (!store) throw new Error('The editor store is only available inside an EditorStoreProvider.');
  return store;
}

/** Subscribes to a slice of the store with a narrow selector (orcon-web ST-6). */
export function useEditorStore<T>(selector: (state: EditorState) => T): T {
  return useStore(useEditorStoreApi(), selector);
}
