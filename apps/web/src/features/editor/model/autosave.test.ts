// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { createAutosave, IDLE_MS, MAX_WAIT_MS, RETRY_MS, type SaveOutcome, type SaveStatus } from './autosave';

function setup(outcomes: SaveOutcome[] | ((call: number) => SaveOutcome) = []) {
  const state = { dirty: false, calls: 0 };
  const statuses: SaveStatus[] = [];
  const save = vi.fn(() => {
    state.calls += 1;
    const outcome = typeof outcomes === 'function' ? outcomes(state.calls) : (outcomes[state.calls - 1] ?? 'saved');
    if (outcome === 'saved') state.dirty = false;
    return Promise.resolve(outcome);
  });
  const autosave = createAutosave({ isDirty: () => state.dirty, save, onStatus: (status) => statuses.push(status) });
  const change = () => {
    state.dirty = true;
    autosave.notifyChange();
  };
  return { state, statuses, save, autosave, change };
}

describe('autosave', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('saves two seconds after the last change', async () => {
    const { change, save, statuses, autosave } = setup();
    change();
    expect(autosave.status()).toBe('unsaved');
    await vi.advanceTimersByTimeAsync(IDLE_MS - 1);
    expect(save).not.toHaveBeenCalled();
    change();
    await vi.advanceTimersByTimeAsync(IDLE_MS - 1);
    expect(save).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(save).toHaveBeenCalledTimes(1);
    expect(statuses).toEqual(['unsaved', 'saving', 'saved']);
  });

  it('saves at most ten seconds after the first unsaved change while changes keep coming', async () => {
    const { change, save } = setup();
    change();
    for (let second = 0; second < 9; second += 1) {
      await vi.advanceTimersByTimeAsync(1_000);
      change();
    }
    expect(save).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(MAX_WAIT_MS - 9_000);
    expect(save).toHaveBeenCalledTimes(1);
  });

  it('saves at once on request, and does nothing when there is nothing to save', async () => {
    const { change, save, autosave } = setup();
    await autosave.flush();
    expect(save).not.toHaveBeenCalled();
    expect(autosave.status()).toBe('saved');
    change();
    await autosave.flush();
    expect(save).toHaveBeenCalledTimes(1);
    expect(autosave.status()).toBe('saved');
  });

  it('keeps the changes made while a save is in flight and saves them next', async () => {
    let release: (outcome: SaveOutcome) => void = () => undefined;
    const state = { dirty: false };
    const statuses: SaveStatus[] = [];
    let calls = 0;
    const autosave = createAutosave({
      isDirty: () => state.dirty,
      save: () => {
        calls += 1;
        if (calls > 1) {
          state.dirty = false;
          return Promise.resolve('saved');
        }
        return new Promise<SaveOutcome>((resolve) => {
          release = (outcome) => {
            // The first save sent the older copy: the person changed the flow again meanwhile, so it stays dirty.
            resolve(outcome);
          };
        });
      },
      onStatus: (status) => statuses.push(status),
    });
    state.dirty = true;
    autosave.notifyChange();
    await vi.advanceTimersByTimeAsync(IDLE_MS);
    expect(autosave.status()).toBe('saving');
    autosave.notifyChange();
    release('saved');
    await vi.advanceTimersByTimeAsync(0);
    expect(autosave.status()).toBe('unsaved');
    await vi.advanceTimersByTimeAsync(IDLE_MS);
    expect(calls).toBe(2);
    expect(autosave.status()).toBe('saved');
  });

  it('tries again with a growing wait after a failure, and says so', async () => {
    const { change, save, statuses, autosave } = setup((call) => (call < 3 ? 'failed' : 'saved'));
    change();
    await vi.advanceTimersByTimeAsync(IDLE_MS);
    expect(autosave.status()).toBe('retrying');
    await vi.advanceTimersByTimeAsync(RETRY_MS[0] - 1);
    expect(save).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(save).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(RETRY_MS[1]);
    expect(save).toHaveBeenCalledTimes(3);
    expect(autosave.status()).toBe('saved');
    expect(statuses).toEqual(['unsaved', 'saving', 'retrying', 'saving', 'retrying', 'saving', 'saved']);
  });

  it('counts a thrown error as a failure and lets the retry send the newest copy', async () => {
    const state = { dirty: true, calls: 0 };
    const autosave = createAutosave({
      isDirty: () => state.dirty,
      save: () => {
        state.calls += 1;
        if (state.calls === 1) return Promise.reject(new Error('offline'));
        state.dirty = false;
        return Promise.resolve('saved');
      },
      onStatus: () => undefined,
    });
    autosave.notifyChange();
    await vi.advanceTimersByTimeAsync(IDLE_MS);
    expect(autosave.status()).toBe('retrying');
    autosave.notifyChange();
    expect(autosave.status()).toBe('retrying');
    await vi.advanceTimersByTimeAsync(RETRY_MS[0]);
    expect(state.calls).toBe(2);
    expect(autosave.status()).toBe('saved');
  });

  it('stops at a conflict and never overwrites', async () => {
    const { change, save, autosave } = setup(['conflict']);
    change();
    await vi.advanceTimersByTimeAsync(IDLE_MS);
    expect(autosave.status()).toBe('conflict');
    change();
    await autosave.flush();
    await vi.advanceTimersByTimeAsync(MAX_WAIT_MS);
    expect(save).toHaveBeenCalledTimes(1);
    expect(autosave.status()).toBe('conflict');
  });

  it('stops for good when it is disposed', async () => {
    const { change, save, autosave } = setup();
    change();
    autosave.dispose();
    await vi.advanceTimersByTimeAsync(MAX_WAIT_MS);
    expect(save).not.toHaveBeenCalled();
  });
});
