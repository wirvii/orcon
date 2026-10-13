// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ANNOUNCE_INTERVAL_MS, useAnnouncer, type Announcer } from '../../src/canvas/announcer';

let announcer: Announcer;

function Probe() {
  announcer = useAnnouncer();
  return <>{announcer.region}</>;
}

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe('polite coalesced announcements', () => {
  it('renders a polite status region that does not take focus', () => {
    render(<Probe />);
    const region = screen.getByRole('status');
    expect(region).toHaveAttribute('aria-live', 'polite');
    expect(region).not.toHaveAttribute('tabindex');
  });

  it('speaks the first message at once', () => {
    render(<Probe />);
    act(() => announcer.announce('Added Check stock'));
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(screen.getByRole('status').textContent.trim()).toBe('Added Check stock');
  });

  it('joins what arrives inside the interval into one announcement', () => {
    render(<Probe />);
    act(() => announcer.announce('One'));
    act(() => {
      vi.advanceTimersByTime(1);
    });
    act(() => {
      announcer.announce('Two');
      announcer.announce('Three');
    });
    // Still the first message: the interval has not ended.
    act(() => {
      vi.advanceTimersByTime(ANNOUNCE_INTERVAL_MS - 100);
    });
    expect(screen.getByRole('status').textContent.trim()).toBe('One');
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(screen.getByRole('status').textContent.trim()).toBe('Two. Three');
  });

  it('announces at most once every five seconds', () => {
    expect(ANNOUNCE_INTERVAL_MS).toBe(5000);
  });
});
