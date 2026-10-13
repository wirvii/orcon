// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { QueryClient } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { z } from 'zod';

import {
  COALESCE_MS,
  createLiveEventDispatcher,
  defineLiveHandler,
  emptyLiveEventSource,
  STREAM_RESET,
  type LiveConnectionState,
  type LiveEvent,
  type LiveEventSource,
  type LiveHandlers,
} from './dispatcher';

/** A source the test drives by hand. */
function manualSource() {
  let deliver: ((event: LiveEvent) => void) | undefined;
  let report: ((state: LiveConnectionState) => void) | undefined;
  const calls = { subscribed: 0, unsubscribed: 0 };
  const source: LiveEventSource = {
    subscribe: (onEvent, onState) => {
      calls.subscribed += 1;
      deliver = onEvent;
      report = onState;
      return () => {
        calls.unsubscribed += 1;
      };
    },
  };
  return {
    calls,
    source,
    emit: (type: string, data: unknown = {}, id?: string) => {
      deliver?.({ type, data, id });
    },
    state: (state: LiveConnectionState) => {
      report?.(state);
    },
  };
}

const flowUpdated = z.object({ id: z.string(), name: z.string() });

/** Handlers for a made-up `flow.*` family, the way a feature's api/live.ts writes them. */
function flowHandlers(queryClient: QueryClient): LiveHandlers {
  return {
    'flow.updated': defineLiveHandler(flowUpdated, (data) => {
      queryClient.setQueryData(['org', 'flows', 'detail', data.id], data);
      return [['org', 'flows', 'list']];
    }),
    'flow.deleted': defineLiveHandler(z.object({ id: z.string() }), (data) => [
      ['org', 'flows', 'list'],
      ['org', 'flows', 'detail', data.id],
    ]),
  };
}

let queryClient: QueryClient;
let invalidate: MockInstance<QueryClient['invalidateQueries']>;

beforeEach(() => {
  vi.useFakeTimers();
  queryClient = new QueryClient();
  invalidate = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue(undefined);
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
});

describe('connection', () => {
  it('opens the stream once and follows the states the source reports', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient });
    expect(dispatcher.getState()).toBe('idle');
    dispatcher.start();
    dispatcher.start();
    expect(live.calls.subscribed).toBe(1);
    expect(dispatcher.getState()).toBe('connecting');
    live.state('live');
    expect(dispatcher.getState()).toBe('live');
    live.state('paused');
    expect(dispatcher.getState()).toBe('paused');
  });

  it('closes the stream on stop and goes idle', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient });
    dispatcher.start();
    live.state('live');
    dispatcher.stop();
    expect(live.calls.unsubscribed).toBe(1);
    expect(dispatcher.getState()).toBe('idle');
    dispatcher.stop();
    expect(live.calls.unsubscribed).toBe(1);
  });

  it('tells listeners about each change once, until they leave', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient });
    const listener = vi.fn();
    const leave = dispatcher.subscribe(listener);
    dispatcher.start();
    live.state('live');
    live.state('live');
    expect(listener).toHaveBeenCalledTimes(2);
    leave();
    live.state('paused');
    expect(listener).toHaveBeenCalledTimes(2);
  });

  it('works with the source that never delivers anything', () => {
    const dispatcher = createLiveEventDispatcher({ source: emptyLiveEventSource, queryClient });
    dispatcher.start();
    expect(dispatcher.getState()).toBe('connecting');
    dispatcher.stop();
    expect(dispatcher.getState()).toBe('idle');
  });
});

describe('events', () => {
  it('patches the cache at once and refetches the returned keys after the window', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient) });
    dispatcher.start();
    live.emit('flow.updated', { id: 'a', name: 'Nightly sync' });
    expect(queryClient.getQueryData(['org', 'flows', 'detail', 'a'])).toEqual({ id: 'a', name: 'Nightly sync' });
    expect(invalidate).not.toHaveBeenCalled();
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ['org', 'flows', 'list'] });
  });

  it('coalesces refetches of the same key and keeps different keys apart', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient) });
    dispatcher.start();
    live.emit('flow.updated', { id: 'a', name: 'One' });
    vi.advanceTimersByTime(100);
    live.emit('flow.updated', { id: 'b', name: 'Two' });
    live.emit('flow.deleted', { id: 'c' });
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate.mock.calls.map(([filters]) => (filters as { queryKey: unknown[] }).queryKey)).toEqual([
      ['org', 'flows', 'list'],
      ['org', 'flows', 'detail', 'c'],
    ]);
    // The window closed: the next event opens a new one.
    live.emit('flow.updated', { id: 'd', name: 'Three' });
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate).toHaveBeenCalledTimes(3);
  });

  it('honors a different window', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient), coalesceMs: 10 });
    dispatcher.start();
    live.emit('flow.deleted', { id: 'a' });
    vi.advanceTimersByTime(10);
    expect(invalidate).toHaveBeenCalledTimes(2);
  });

  it('ignores and counts an event type nobody handles', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient) });
    dispatcher.start();
    live.emit('execution.finished', { id: 'x' });
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate).not.toHaveBeenCalled();
    expect(dispatcher.getStats()).toEqual({ received: 1, ignored: 1, invalid: 0 });
  });

  it('counts a payload that breaks its schema, logs it without the value, and keeps going', () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient) });
    dispatcher.start();
    live.emit('flow.updated', { id: 'a', name: 42, secret: 'api-key-123' });
    live.emit('flow.deleted', { id: 'b' });
    vi.advanceTimersByTime(COALESCE_MS);
    expect(dispatcher.getStats()).toEqual({ received: 2, ignored: 0, invalid: 1 });
    expect(log).toHaveBeenCalledTimes(1);
    expect(JSON.stringify(log.mock.calls)).not.toContain('api-key-123');
    expect(queryClient.getQueryData(['org', 'flows', 'detail', 'a'])).toBeUndefined();
    expect(invalidate).toHaveBeenCalledTimes(2);
  });

  it('lets a bug in a handler surface instead of hiding it', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({
      source: live.source,
      queryClient,
      handlers: {
        'flow.updated': () => {
          throw new Error('handler bug');
        },
      },
    });
    dispatcher.start();
    expect(() => {
      live.emit('flow.updated');
    }).toThrow('handler bug');
    expect(dispatcher.getStats().invalid).toBe(1);
  });

  it('treats a handler that returns nothing as having nothing to refetch', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: { 'flow.touched': () => undefined } });
    dispatcher.start();
    live.emit('flow.touched');
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate).not.toHaveBeenCalled();
    expect(dispatcher.getStats().received).toBe(1);
  });
});

describe('stream.reset and reconnecting', () => {
  it('refetches everything at once on stream.reset and drops what was waiting', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient) });
    dispatcher.start();
    live.emit('flow.deleted', { id: 'a' });
    live.emit(STREAM_RESET);
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(invalidate).toHaveBeenCalledWith();
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate).toHaveBeenCalledTimes(1);
  });

  it('refetches what live handlers touched after a reconnect, not at the first connection', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient) });
    dispatcher.start();
    live.state('live');
    live.emit('flow.deleted', { id: 'a' });
    vi.advanceTimersByTime(COALESCE_MS);
    invalidate.mockClear();

    live.state('paused');
    expect(invalidate).not.toHaveBeenCalled();
    live.state('live');
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate.mock.calls.map(([filters]) => (filters as { queryKey: unknown[] }).queryKey)).toEqual([
      ['org', 'flows', 'list'],
      ['org', 'flows', 'detail', 'a'],
    ]);
  });

  it('refetches nothing after a reconnect when no handler touched anything', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient });
    dispatcher.start();
    live.state('paused');
    live.state('live');
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate).not.toHaveBeenCalled();
  });

  it('drops pending refetches when it stops', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: flowHandlers(queryClient) });
    dispatcher.start();
    live.emit('flow.deleted', { id: 'a' });
    dispatcher.stop();
    vi.advanceTimersByTime(COALESCE_MS);
    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe('registering handlers', () => {
  it('adds handlers while a screen is mounted and removes them when it leaves', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient });
    dispatcher.start();
    const seen = vi.fn(() => [['org', 'attention']]);
    const remove = dispatcher.register({ 'attention.created': seen });
    live.emit('attention.created');
    expect(seen).toHaveBeenCalledTimes(1);
    remove();
    live.emit('attention.created');
    expect(seen).toHaveBeenCalledTimes(1);
    expect(dispatcher.getStats().ignored).toBe(1);
  });

  it('runs every handler registered for a type', () => {
    const live = manualSource();
    const first = vi.fn(() => undefined);
    const second = vi.fn(() => undefined);
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient, handlers: { 'flow.touched': first } });
    dispatcher.register({ 'flow.touched': second });
    dispatcher.start();
    live.emit('flow.touched');
    expect(first).toHaveBeenCalledTimes(1);
    expect(second).toHaveBeenCalledTimes(1);
  });
});

describe('defineLiveHandler', () => {
  it('hands the parsed payload to the handler, with defaults applied', () => {
    const apply = vi.fn(() => undefined);
    const handler = defineLiveHandler(z.object({ id: z.string(), tab: z.string().default('overview') }), apply);
    handler({ type: 'flow.updated', data: { id: 'a', extra: true } }, queryClient);
    expect(apply).toHaveBeenCalledWith({ id: 'a', tab: 'overview' }, queryClient);
  });

  it('throws a contract error that names the event type', () => {
    const handler = defineLiveHandler(z.object({ id: z.string() }), () => undefined);
    expect(() => handler({ type: 'flow.updated', data: { id: 1 } }, queryClient)).toThrow('live event flow.updated does not match its schema');
  });
});
