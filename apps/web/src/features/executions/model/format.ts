// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Execution, RunStatus } from './types';

/** Milliseconds from start to finish, or undefined while the run has not finished or has not started. */
export function durationOf(startedAt: string | undefined, finishedAt: string | undefined): number | undefined {
  if (startedAt === undefined || finishedAt === undefined) return undefined;
  const ms = Date.parse(finishedAt) - Date.parse(startedAt);
  return Number.isFinite(ms) && ms >= 0 ? ms : undefined;
}

/**
 * The end of a run identifier. Identifiers are time-ordered, so their first characters are the same for runs that
 * started close together; the end is the random part and tells those runs apart.
 */
export function tailId(id: string): string {
  return id.slice(-8);
}

/**
 * The state a run is shown with. An operation of unknown outcome makes the run "Needs review" whatever its engine
 * status is: it is never drawn as a failure or as a success (orcon-ui rule 5).
 */
export function runStatusKey(run: Pick<Execution, 'status' | 'hasUncertainOperation'>): RunStatus {
  return run.hasUncertainOperation === true ? 'UNCERTAIN' : run.status;
}
