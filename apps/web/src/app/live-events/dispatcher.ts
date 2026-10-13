// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { QueryClient, QueryKey } from '@tanstack/react-query';
import type { ZodType } from 'zod';

import { parseWith } from '../boundary/parse';
import { ContractError } from '../errors/contract-error';
import { reportContractError } from '../errors/report';

/** A server-sent event after the transport decoded it: its type and the still unchecked payload. */
export interface LiveEvent {
  readonly id?: string | undefined;
  readonly type: string;
  readonly data: unknown;
}

/**
 * Where the stream stands. `idle` means no stream is open; `paused` means the connection
 * dropped and the screens show the "live updates paused" banner (orcon-web DT-15).
 */
export type LiveConnectionState = 'idle' | 'connecting' | 'live' | 'paused';

/**
 * The stream of events. `@orcon/sdk` provides the real source (SPEC-005); until then the app
 * uses {@link emptyLiveEventSource} and tests use a Mock Service Worker `sse` handler.
 */
export interface LiveEventSource {
  /** Starts delivering events and connection states. Returns the function that stops it. */
  subscribe(onEvent: (event: LiveEvent) => void, onState: (state: LiveConnectionState) => void): () => void;
}

/** A source that never delivers anything: the app before the SDK has a stream. */
export const emptyLiveEventSource: LiveEventSource = {
  subscribe: () => () => undefined,
};

/**
 * What a feature does with one event type. It patches the cache itself when the event
 * carries enough, and returns the narrowest keys to refetch (orcon-web DT-12). The
 * dispatcher coalesces the returned keys.
 */
export type LiveHandler = (event: LiveEvent, queryClient: QueryClient) => readonly QueryKey[] | undefined;

/** Handlers by event type. Each feature exports its own map from `api/live.ts`. */
export type LiveHandlers = Readonly<Record<string, LiveHandler>>;

/** Builds a handler that parses the payload with its schema before use (orcon-web DT-13). */
export function defineLiveHandler<T>(
  schema: ZodType<T>,
  apply: (data: T, queryClient: QueryClient) => readonly QueryKey[] | undefined,
): LiveHandler {
  return (event, queryClient) => apply(parseWith(schema, event.data, `live event ${event.type}`), queryClient);
}

/** The event that tells a client it missed events and must refetch what it shows. */
export const STREAM_RESET = 'stream.reset';
/** Refetch requests for the same key within this window become one (orcon-web DT-14). */
export const COALESCE_MS = 250;

/** What the dispatcher has seen, for tests and diagnostics. */
export interface LiveEventStats {
  /** Events received. */
  readonly received: number;
  /** Events of a type nobody handles; ignored (orcon-web DT-13). */
  readonly ignored: number;
  /** Events whose payload did not match its schema, or whose handler failed. */
  readonly invalid: number;
}

export interface LiveEventDispatcher {
  /** Opens the stream. Does nothing when it is open. */
  readonly start: () => void;
  /** Closes the stream and drops pending refetches. */
  readonly stop: () => void;
  /** Adds handlers while the caller is mounted. Returns the function that removes them. */
  readonly register: (handlers: LiveHandlers) => () => void;
  readonly getState: () => LiveConnectionState;
  /** Calls the listener whenever the connection state changes. Returns the unsubscribe function. */
  readonly subscribe: (listener: () => void) => () => void;
  readonly getStats: () => LiveEventStats;
}

export interface LiveEventDispatcherOptions {
  source: LiveEventSource;
  queryClient: QueryClient;
  handlers?: LiveHandlers | undefined;
  coalesceMs?: number | undefined;
}

/**
 * Receives the events of one organization stream, hands each to the feature handlers and
 * keeps the cache fresh:
 *
 * - a payload is parsed by its handler; an unknown type is ignored and counted;
 * - refetches are coalesced per key; patches written by a handler apply at once;
 * - `stream.reset`, or a reconnect after a pause, refetches what live handlers touched.
 */
export function createLiveEventDispatcher({ source, queryClient, handlers = {}, coalesceMs = COALESCE_MS }: LiveEventDispatcherOptions): LiveEventDispatcher {
  const registered = new Set<LiveHandlers>([handlers]);
  const listeners = new Set<() => void>();
  /** Keys that handlers asked to refetch, waiting for the coalescing window to end. */
  const pending = new Map<string, QueryKey>();
  /** Every key a handler ever touched: what a reconnect refetches. */
  const touched = new Map<string, QueryKey>();
  let state: LiveConnectionState = 'idle';
  let stats: LiveEventStats = { received: 0, ignored: 0, invalid: 0 };
  let timer: ReturnType<typeof setTimeout> | undefined;
  let stopSource: (() => void) | undefined;

  const setState = (next: LiveConnectionState) => {
    if (next === state) return;
    state = next;
    for (const listener of listeners) listener();
  };

  const flush = () => {
    timer = undefined;
    for (const queryKey of pending.values()) void queryClient.invalidateQueries({ queryKey });
    pending.clear();
  };

  const schedule = (keys: readonly QueryKey[]) => {
    for (const key of keys) {
      const hash = JSON.stringify(key);
      pending.set(hash, key);
      touched.set(hash, key);
    }
    if (pending.size > 0) timer ??= setTimeout(flush, coalesceMs);
  };

  const cancelPending = () => {
    if (timer !== undefined) clearTimeout(timer);
    timer = undefined;
    pending.clear();
  };

  const handle = (event: LiveEvent) => {
    stats = { ...stats, received: stats.received + 1 };
    if (event.type === STREAM_RESET) {
      cancelPending();
      void queryClient.invalidateQueries();
      return;
    }
    const matching = [...registered].map((set) => set[event.type]).filter((handler): handler is LiveHandler => handler !== undefined);
    if (matching.length === 0) {
      stats = { ...stats, ignored: stats.ignored + 1 };
      return;
    }
    for (const handler of matching) {
      try {
        schedule(handler(event, queryClient) ?? []);
      } catch (error) {
        stats = { ...stats, invalid: stats.invalid + 1 };
        if (error instanceof ContractError) reportContractError(error);
        else throw error;
      }
    }
  };

  const onState = (next: LiveConnectionState) => {
    const reconnected = state === 'paused' && next === 'live';
    setState(next);
    if (reconnected) schedule([...touched.values()]);
  };

  return {
    start: () => {
      if (stopSource) return;
      setState('connecting');
      stopSource = source.subscribe(handle, onState);
    },
    stop: () => {
      stopSource?.();
      stopSource = undefined;
      cancelPending();
      setState('idle');
    },
    register: (more) => {
      registered.add(more);
      return () => {
        registered.delete(more);
      };
    },
    getState: () => state,
    subscribe: (listener) => {
      listeners.add(listener);
      return () => {
        listeners.delete(listener);
      };
    },
    getStats: () => stats,
  };
}
