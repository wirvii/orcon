// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * When the editor saves the draft (SPEC-014 R-ED-DRAFT-01, R-ED-DRAFT-02): 2 seconds after the last
 * change, at most 10 seconds after the first unsaved one while changes keep coming, at once on
 * request, and again with a growing wait after a failure. The scheduler knows nothing about the
 * server: the screen gives it a `save` function and reads the state it reports.
 */

/** What the save indicator says: Saved, Saving, Unsaved changes, Not saved (retrying), or a conflict that needs the person. */
export type SaveStatus = 'saved' | 'saving' | 'unsaved' | 'retrying' | 'conflict';

export type SaveOutcome = 'saved' | 'conflict' | 'failed';

export const IDLE_MS = 2_000;
export const MAX_WAIT_MS = 10_000;
export const RETRY_MS = [2_000, 4_000, 8_000, 16_000, 30_000] as const;

export interface AutosaveOptions {
  /** True while the working copy differs from the last saved one. */
  isDirty: () => boolean;
  /** Sends the working copy. A thrown error counts as a failure. */
  save: () => Promise<SaveOutcome>;
  onStatus: (status: SaveStatus) => void;
}

export interface Autosave {
  /** Tell the scheduler the working copy changed. */
  notifyChange: () => void;
  /** Save now (Ctrl+S). */
  flush: () => Promise<void>;
  status: () => SaveStatus;
  dispose: () => void;
}

export function createAutosave(options: AutosaveOptions): Autosave {
  let status: SaveStatus = 'saved';
  let idle: ReturnType<typeof setTimeout> | undefined;
  let longest: ReturnType<typeof setTimeout> | undefined;
  let retry: ReturnType<typeof setTimeout> | undefined;
  let failures = 0;
  let inFlight = false;
  // Read through an object: a save in flight can be stopped (disposed) while it is awaited.
  const life = { stopped: false };
  const isStopped = () => life.stopped;

  const setStatus = (next: SaveStatus) => {
    if (next === status) return;
    status = next;
    options.onStatus(next);
  };

  const clearWaits = () => {
    clearTimeout(idle);
    clearTimeout(longest);
    clearTimeout(retry);
    idle = undefined;
    longest = undefined;
    retry = undefined;
  };

  const run = async (): Promise<void> => {
    clearWaits();
    if (inFlight || life.stopped) return;
    if (!options.isDirty()) {
      setStatus('saved');
      return;
    }
    inFlight = true;
    setStatus('saving');
    let outcome: SaveOutcome;
    try {
      outcome = await options.save();
    } catch {
      outcome = 'failed';
    }
    inFlight = false;
    if (isStopped()) return;
    if (outcome === 'conflict') {
      life.stopped = true;
      setStatus('conflict');
      return;
    }
    if (outcome === 'failed') {
      failures += 1;
      setStatus('retrying');
      retry = setTimeout(() => void run(), RETRY_MS[Math.min(failures, RETRY_MS.length) - 1]);
      return;
    }
    failures = 0;
    if (options.isDirty()) notifyChange();
    else setStatus('saved');
  };

  function notifyChange(): void {
    if (life.stopped) return;
    if (failures > 0) {
      // A retry is already waiting; it will send the newest copy.
      setStatus('retrying');
      return;
    }
    setStatus(inFlight ? 'saving' : 'unsaved');
    clearTimeout(idle);
    idle = setTimeout(() => void run(), IDLE_MS);
    longest ??= setTimeout(() => void run(), MAX_WAIT_MS);
  }

  return {
    notifyChange,
    flush: run,
    status: () => status,
    dispose: () => {
      life.stopped = true;
      clearWaits();
    },
  };
}
