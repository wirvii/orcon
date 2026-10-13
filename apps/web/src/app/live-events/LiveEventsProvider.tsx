// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useQueryClient } from '@tanstack/react-query';
import { createContext, useContext, useEffect, useMemo, useSyncExternalStore, type ReactNode } from 'react';

import {
  createLiveEventDispatcher,
  type LiveConnectionState,
  type LiveEventDispatcher,
  type LiveEventSource,
  type LiveHandlers,
} from './dispatcher';

const DispatcherContext = createContext<LiveEventDispatcher | null>(null);

export interface LiveEventsProviderProps {
  /** The organization stream. One per tab (orcon-web DT-12). */
  source: LiveEventSource;
  /** Handlers known up front; features add theirs with {@link useLiveHandlers}. */
  handlers?: LiveHandlers | undefined;
  children: ReactNode;
}

/** Opens the organization stream while the app is mounted and keeps the query cache fresh from it. */
export function LiveEventsProvider({ source, handlers, children }: LiveEventsProviderProps) {
  const queryClient = useQueryClient();
  const dispatcher = useMemo(() => createLiveEventDispatcher({ source, queryClient, handlers }), [source, queryClient, handlers]);

  useEffect(() => {
    dispatcher.start();
    return () => dispatcher.stop();
  }, [dispatcher]);

  return <DispatcherContext.Provider value={dispatcher}>{children}</DispatcherContext.Provider>;
}

function useDispatcher(): LiveEventDispatcher {
  const dispatcher = useContext(DispatcherContext);
  if (!dispatcher) throw new Error('Live events hooks must be used inside a LiveEventsProvider.');
  return dispatcher;
}

/** The connection state of the stream, to show the "live updates paused" banner. */
export function useLiveEventsState(): LiveConnectionState {
  const dispatcher = useDispatcher();
  return useSyncExternalStore(dispatcher.subscribe, dispatcher.getState);
}

/**
 * Registers a feature's event handlers while the calling component is mounted. Pass a
 * stable map (a module constant such as the one exported from `api/live.ts`).
 */
export function useLiveHandlers(handlers: LiveHandlers): void {
  const dispatcher = useDispatcher();
  useEffect(() => dispatcher.register(handlers), [dispatcher, handlers]);
}
