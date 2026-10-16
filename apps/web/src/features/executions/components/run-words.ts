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
