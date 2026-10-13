// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { act, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { z } from 'zod';

import { createLiveEventDispatcher, defineLiveHandler, type LiveConnectionState, type LiveEvent, type LiveEventSource } from './dispatcher';
import { LiveEventsProvider, useLiveEventsState, useLiveHandlers } from './LiveEventsProvider';

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
    emit: (type: string, data: unknown) => {
      act(() => {
        deliver?.({ type, data });
      });
    },
    state: (state: LiveConnectionState) => {
      act(() => {
        report?.(state);
      });
    },
  };
}

const handlers = (queryClient: QueryClient) => ({
  'widget.renamed': defineLiveHandler(z.object({ id: z.string(), name: z.string() }), (data) => {
    queryClient.setQueryData(['org', 'widget', data.id], data.name);
    return undefined;
  }),
});

function WidgetName({ id, queryClient }: { id: string; queryClient: QueryClient }) {
  useLiveHandlers(handlers(queryClient));
  const name = useQuery({ queryKey: ['org', 'widget', id], queryFn: () => Promise.resolve('Original'), initialData: 'Original', staleTime: Infinity });
  const state = useLiveEventsState();
  return (
    <p>
      {name.data} / {state}
    </p>
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('LiveEventsProvider', () => {
  it('opens the stream while it is mounted and closes it when it leaves', () => {
    const live = manualSource();
    const queryClient = new QueryClient();
    const { unmount } = render(
      <QueryClientProvider client={queryClient}>
        <LiveEventsProvider source={live.source}>
          <p>app</p>
        </LiveEventsProvider>
      </QueryClientProvider>,
    );
    expect(live.calls.subscribed).toBe(1);
    unmount();
    expect(live.calls.unsubscribed).toBe(1);
  });

  it('lets a mounted screen add its handlers, and tells it the state of the stream', async () => {
    const live = manualSource();
    const queryClient = new QueryClient();
    render(
      <QueryClientProvider client={queryClient}>
        <LiveEventsProvider source={live.source}>
          <WidgetName id="a" queryClient={queryClient} />
        </LiveEventsProvider>
      </QueryClientProvider>,
    );
    expect(screen.getByText('Original / connecting')).toBeVisible();
    live.state('live');
    live.emit('widget.renamed', { id: 'a', name: 'Renamed live' });
    expect(await screen.findByText('Renamed live / live')).toBeVisible();
    live.state('paused');
    expect(await screen.findByText('Renamed live / paused')).toBeVisible();
  });

  it('stops handling an event type when the screen that registered it is gone', () => {
    const live = manualSource();
    const queryClient = new QueryClient();
    const tree = (show: boolean) => (
      <QueryClientProvider client={queryClient}>
        <LiveEventsProvider source={live.source}>{show ? <WidgetName id="a" queryClient={queryClient} /> : <p>gone</p>}</LiveEventsProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(tree(true));
    rerender(tree(false));
    live.emit('widget.renamed', { id: 'a', name: 'Too late' });
    expect(queryClient.getQueryData(['org', 'widget', 'a'])).toBe('Original');
  });

  it('uses the handlers it was given up front', () => {
    const live = manualSource();
    const queryClient = new QueryClient();
    render(
      <QueryClientProvider client={queryClient}>
        <LiveEventsProvider source={live.source} handlers={handlers(queryClient)}>
          <p>app</p>
        </LiveEventsProvider>
      </QueryClientProvider>,
    );
    live.emit('widget.renamed', { id: 'b', name: 'Up front' });
    expect(queryClient.getQueryData(['org', 'widget', 'b'])).toBe('Up front');
  });

  it('refuses the hooks outside the provider', () => {
    const quiet = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    expect(() => render(<WidgetName id="a" queryClient={new QueryClient()} />)).toThrow('inside a LiveEventsProvider');
    quiet.mockRestore();
  });

  it('is usable with a dispatcher built by hand (the same behavior the provider relies on)', () => {
    const live = manualSource();
    const dispatcher = createLiveEventDispatcher({ source: live.source, queryClient: new QueryClient() });
    dispatcher.start();
    live.state('live');
    expect(dispatcher.getState()).toBe('live');
  });
});
