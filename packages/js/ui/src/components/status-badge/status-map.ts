// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Ban,
  BellRing,
  CircleAlert,
  CircleCheck,
  CircleDashed,
  CircleOff,
  CirclePause,
  CircleX,
  Clock,
  Hourglass,
  LoaderCircle,
  RotateCw,
  ShieldQuestion,
  Undo2,
  type LucideIcon,
} from 'lucide-react';
import { defineMessages, type MessageDescriptor } from 'react-intl';

import type { BadgeProps } from '../badge/Badge';

/** Color families of the status tokens. `attention` (violet) is reserved for UNCERTAIN and states that need a person. */
export type StatusFamily = NonNullable<BadgeProps['tone']>;

/**
 * Domain states shown as a StatusBadge. Names follow the execution model
 * (SPEC-001); the map below is the normative pairing of state, label, icon and
 * color family.
 */
export type StatusKey =
  | 'QUEUED'
  | 'RUNNING'
  | 'WAITING'
  | 'SUSPENDED'
  | 'CANCELING'
  | 'CANCELED'
  | 'COMPENSATING'
  | 'SUCCEEDED'
  | 'SUCCEEDED_WITH_ERRORS'
  | 'FAILED'
  | 'ERROR_FINAL'
  | 'UNCERTAIN'
  | 'RETRY_SCHEDULED'
  | 'NOT_DISPATCHED'
  | 'NOT_RUN'
  | 'AWAITING_PERSON';

export const statusMessages = defineMessages({
  queued: { id: 'status.queued', defaultMessage: 'Queued', description: 'Status of an execution waiting to start.' },
  running: { id: 'status.running', defaultMessage: 'Running', description: 'Status of an execution or node that is running now.' },
  waiting: { id: 'status.waiting', defaultMessage: 'Waiting', description: 'Status of an execution waiting for a timer or an event.' },
  suspended: {
    id: 'status.suspended',
    defaultMessage: 'Needs action',
    description: 'Status of a suspended execution that needs a person to decide.',
  },
  canceling: { id: 'status.canceling', defaultMessage: 'Canceling', description: 'Status of an execution that is being stopped.' },
  canceled: { id: 'status.canceled', defaultMessage: 'Canceled', description: 'Status of an execution that was stopped.' },
  compensating: {
    id: 'status.compensating',
    defaultMessage: 'Undoing',
    description: 'Status of an execution that is undoing completed steps after a failure.',
  },
  succeeded: { id: 'status.succeeded', defaultMessage: 'Succeeded', description: 'Status of an execution or node that finished correctly.' },
  succeededWithErrors: {
    id: 'status.succeededWithErrors',
    defaultMessage: 'Succeeded with errors',
    description: 'Status of a node that finished, but some of its items failed.',
  },
  failed: { id: 'status.failed', defaultMessage: 'Failed', description: 'Status of an execution, node or item that failed for good.' },
  uncertain: {
    id: 'status.uncertain',
    defaultMessage: 'Uncertain',
    description: 'Status of an item whose external operation may or may not have been applied.',
  },
  retryScheduled: {
    id: 'status.retryScheduled',
    defaultMessage: 'Retry scheduled',
    description: 'Status of an item that failed and will be tried again.',
  },
  notDispatched: {
    id: 'status.notDispatched',
    defaultMessage: 'Not sent',
    description: 'Status of an item that was never sent to the external system.',
  },
  notRun: { id: 'status.notRun', defaultMessage: 'Not run', description: 'Status of a node on a path that was not taken.' },
  awaitingPerson: {
    id: 'status.awaitingPerson',
    defaultMessage: 'Awaiting a person',
    description: 'Status of work that has an open attention request for a person.',
  },
  uncertainDescription: {
    id: 'status.uncertain.description',
    defaultMessage: 'The external system may or may not have applied this operation. Review it before continuing.',
    description: 'Accessible description of the uncertain status. It never says the operation failed or succeeded.',
  },
});

export interface StatusEntry {
  family: StatusFamily;
  icon: LucideIcon;
  label: MessageDescriptor;
  /** The icon turns while the state lasts. Rotation stops under reduced motion; icon and text remain. */
  spins?: boolean | undefined;
  /** Longer explanation exposed to assistive technology. */
  description?: MessageDescriptor | undefined;
}

export const statusMap: Readonly<Record<StatusKey, StatusEntry>> = {
  QUEUED: { family: 'neutral', icon: Clock, label: statusMessages.queued },
  RUNNING: { family: 'active', icon: LoaderCircle, label: statusMessages.running, spins: true },
  WAITING: { family: 'active', icon: Hourglass, label: statusMessages.waiting },
  SUSPENDED: { family: 'attention', icon: CirclePause, label: statusMessages.suspended },
  CANCELING: { family: 'neutral', icon: Ban, label: statusMessages.canceling },
  CANCELED: { family: 'neutral', icon: Ban, label: statusMessages.canceled },
  COMPENSATING: { family: 'active', icon: Undo2, label: statusMessages.compensating },
  SUCCEEDED: { family: 'success', icon: CircleCheck, label: statusMessages.succeeded },
  SUCCEEDED_WITH_ERRORS: { family: 'warning', icon: CircleAlert, label: statusMessages.succeededWithErrors },
  FAILED: { family: 'danger', icon: CircleX, label: statusMessages.failed },
  ERROR_FINAL: { family: 'danger', icon: CircleX, label: statusMessages.failed },
  UNCERTAIN: {
    family: 'attention',
    icon: ShieldQuestion,
    label: statusMessages.uncertain,
    description: statusMessages.uncertainDescription,
  },
  RETRY_SCHEDULED: { family: 'neutral', icon: RotateCw, label: statusMessages.retryScheduled },
  NOT_DISPATCHED: { family: 'neutral', icon: CircleOff, label: statusMessages.notDispatched },
  NOT_RUN: { family: 'neutral', icon: CircleDashed, label: statusMessages.notRun },
  AWAITING_PERSON: { family: 'attention', icon: BellRing, label: statusMessages.awaitingPerson },
};

export const statusKeys = Object.keys(statusMap) as StatusKey[];
