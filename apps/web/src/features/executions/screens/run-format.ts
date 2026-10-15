// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { MessageDescriptor } from 'react-intl';

import { runMessages } from './messages';

/** The words for how a run started, by the mode the contract names. */
export const modeMessages: Record<string, MessageDescriptor> = {
  WEBHOOK: runMessages.modeWebhook,
  TRIGGER: runMessages.modeTrigger,
  TEST: runMessages.modeTest,
  MANUAL: runMessages.modeManual,
  CHILD: runMessages.modeChild,
  ERROR: runMessages.modeError,
  RETRY: runMessages.modeRetry,
  EVALUATION: runMessages.modeEvaluation,
};

/** Milliseconds from start to finish, or undefined while the run has not finished or has not started. */
export function durationOf(startedAt: string | undefined, finishedAt: string | undefined): number | undefined {
  if (startedAt === undefined || finishedAt === undefined) return undefined;
  const ms = Date.parse(finishedAt) - Date.parse(startedAt);
  return Number.isFinite(ms) && ms >= 0 ? ms : undefined;
}

/** The short form of a run identifier, enough to tell runs apart on screen. */
export function shortId(id: string): string {
  return id.slice(0, 8);
}
