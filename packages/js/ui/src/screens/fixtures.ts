// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Example data for the screen templates. Nothing here comes from a server: templates make no
// HTTP calls. Every instant is relative to one fixed "now" so stories and screenshots never change.

import type { StatusKey } from '../components/status-badge/status-map';

export const FIXTURE_NOW = '2026-10-12T15:00:00.000Z';

const minutesAgo = (minutes: number) => new Date(Date.parse(FIXTURE_NOW) - minutes * 60_000).toISOString();
const minutesFromNow = (minutes: number) => new Date(Date.parse(FIXTURE_NOW) + minutes * 60_000).toISOString();

export interface FlowRow {
  id: string;
  name: string;
  description: string;
  enabled: boolean;
  lastRunAt?: string | undefined;
  lastStatus?: StatusKey | undefined;
}

export const flows: FlowRow[] = [
  { id: 'order-fulfilment', name: 'Order fulfilment', description: 'Checks stock, charges the card and sends the receipt.', enabled: true, lastRunAt: minutesAgo(12), lastStatus: 'SUCCEEDED_WITH_ERRORS' },
  { id: 'daily-report', name: 'Daily sales report', description: 'Collects yesterday sales and emails a summary at 07:00.', enabled: true, lastRunAt: minutesAgo(480), lastStatus: 'SUCCEEDED' },
  { id: 'refund-review', name: 'Refund review', description: 'Asks a person to approve refunds above the limit.', enabled: true, lastRunAt: minutesAgo(95), lastStatus: 'SUSPENDED' },
  { id: 'stock-sync', name: 'Stock synchronisation', description: 'Copies warehouse counts to the shop every hour.', enabled: false, lastRunAt: minutesAgo(2880), lastStatus: 'FAILED' },
  { id: 'welcome-email', name: 'Welcome email', description: 'Sends a welcome email to every new customer.', enabled: false },
];

export interface ExecutionRow {
  id: string;
  flowName: string;
  status: StatusKey;
  startedAt: string;
  durationMs?: number | undefined;
  items: number;
}

export const executions: ExecutionRow[] = [
  { id: 'ex-4821', flowName: 'Order fulfilment', status: 'SUCCEEDED_WITH_ERRORS', startedAt: minutesAgo(12), durationMs: 41_000, items: 12 },
  { id: 'ex-4820', flowName: 'Refund review', status: 'SUSPENDED', startedAt: minutesAgo(95), items: 1 },
  { id: 'ex-4819', flowName: 'Order fulfilment', status: 'RUNNING', startedAt: minutesAgo(2), items: 4 },
  { id: 'ex-4818', flowName: 'Daily sales report', status: 'SUCCEEDED', startedAt: minutesAgo(480), durationMs: 128_000, items: 340 },
  { id: 'ex-4817', flowName: 'Order fulfilment', status: 'UNCERTAIN', startedAt: minutesAgo(130), durationMs: 18_000, items: 1 },
  { id: 'ex-4816', flowName: 'Stock synchronisation', status: 'FAILED', startedAt: minutesAgo(2880), durationMs: 3_000, items: 0 },
  { id: 'ex-4815', flowName: 'Order fulfilment', status: 'CANCELED', startedAt: minutesAgo(3000), durationMs: 9_000, items: 2 },
  { id: 'ex-4814', flowName: 'Refund review', status: 'QUEUED', startedAt: minutesAgo(1), items: 1 },
];

export interface AttentionRow {
  id: string;
  title: string;
  flowName: string;
  /** What kind of decision it asks for. A batch action only groups items of the same cause and flow. */
  cause: 'approval' | 'reviewUncertain' | 'failedStep';
  group: 'mine' | 'groups' | 'other';
  createdAt: string;
  escalationLevel?: number | undefined;
  acknowledgedBy?: string | undefined;
  overdue?: boolean | undefined;
}

export const attentionRows: AttentionRow[] = [
  { id: 'at-1', title: 'Approve a refund of 480 USD', flowName: 'Refund review', cause: 'approval', group: 'mine', createdAt: minutesAgo(95), overdue: true, escalationLevel: 1 },
  { id: 'at-2', title: 'Approve a refund of 120 USD', flowName: 'Refund review', cause: 'approval', group: 'mine', createdAt: minutesAgo(40) },
  { id: 'at-3', title: 'Review a card charge that may have been applied', flowName: 'Order fulfilment', cause: 'reviewUncertain', group: 'groups', createdAt: minutesAgo(130), acknowledgedBy: 'Ana Ruiz' },
  { id: 'at-4', title: 'Approve a refund of 75 USD', flowName: 'Refund review', cause: 'approval', group: 'groups', createdAt: minutesAgo(15) },
  { id: 'at-5', title: 'Choose what to do with a failed stock update', flowName: 'Stock synchronisation', cause: 'failedStep', group: 'other', createdAt: minutesAgo(2880), overdue: true, escalationLevel: 2 },
];

export interface NotificationRow {
  id: string;
  text: string;
  at: string;
  tone: 'attention' | 'success' | 'danger';
}

export const notificationRows: NotificationRow[] = [
  { id: 'nt-1', text: 'Refund review needs your decision', at: minutesAgo(95), tone: 'attention' },
  { id: 'nt-2', text: 'Daily sales report finished', at: minutesAgo(480), tone: 'success' },
  { id: 'nt-3', text: 'Stock synchronisation failed', at: minutesAgo(2880), tone: 'danger' },
];

export interface AttemptRow {
  attempt: number;
  status: StatusKey;
  startedAt: string;
  durationMs: number;
}

export interface TimelineNode {
  id: string;
  name: string;
  status: StatusKey;
  items: number;
  attempts: AttemptRow[];
  output?: string | undefined;
}

export const executionTimeline: TimelineNode[] = [
  {
    id: 'order-received',
    name: 'Order received',
    status: 'SUCCEEDED',
    items: 12,
    attempts: [{ attempt: 1, status: 'SUCCEEDED', startedAt: minutesAgo(130), durationMs: 120 }],
    output: '{\n  "orderId": "A-10492",\n  "customer": "ana.ruiz@example.test",\n  "total": 84.5\n}',
  },
  {
    id: 'check-stock',
    name: 'Check stock',
    status: 'SUCCEEDED',
    items: 12,
    attempts: [{ attempt: 1, status: 'SUCCEEDED', startedAt: minutesAgo(130), durationMs: 640 }],
    output: '{\n  "inStock": true,\n  "warehouse": "north"\n}',
  },
  {
    id: 'charge-card',
    name: 'Charge card',
    status: 'UNCERTAIN',
    items: 1,
    attempts: [
      { attempt: 1, status: 'RETRY_SCHEDULED', startedAt: minutesAgo(130), durationMs: 5_000 },
      { attempt: 2, status: 'UNCERTAIN', startedAt: minutesAgo(129), durationMs: 10_000 },
    ],
    output: '{\n  "paymentId": "pay_7Q2K",\n  "amount": 84.5\n}',
  },
  { id: 'send-receipt', name: 'Send receipt', status: 'NOT_RUN', items: 0, attempts: [] },
];

export interface ResolutionAction {
  id: string;
  /** What the action does, in the words of the connector. */
  label: string;
  kind: 'repeat' | 'mark-applied' | 'mark-not-applied' | 'other';
  /** The action can have a lasting effect; a person must tick the acknowledgment first. */
  requiresRiskAck?: boolean | undefined;
  /** The risk the acknowledgment states. */
  riskText?: string | undefined;
}

export interface UncertainResolution {
  nodeName: string;
  /** The connector's own instructions for checking by hand. Shown as plain text, exactly as received. */
  manualCheck: string;
  actions: ResolutionAction[];
}

export const uncertainResolution: UncertainResolution = {
  nodeName: 'Charge card',
  manualCheck: 'Open the payments dashboard and search for payment pay_7Q2K. If it appears as "captured", the customer was charged.',
  actions: [
    { id: 'mark-applied', label: 'Mark as charged', kind: 'mark-applied' },
    { id: 'mark-not-applied', label: 'Mark as not charged and continue', kind: 'mark-not-applied', requiresRiskAck: true, riskText: 'If the customer was already charged, they may be charged twice.' },
    { id: 'repeat', label: 'Charge again', kind: 'repeat', requiresRiskAck: true, riskText: 'If the first charge went through, the customer is charged a second time.' },
  ],
};

export interface ApprovalRequest {
  id: string;
  tool: string;
  flowName: string;
  requestedBy: string;
  expiresAt: string;
  argumentsJson: string;
}

export const approvalRequest: ApprovalRequest = {
  id: 'ap-310',
  tool: 'payments.refund',
  flowName: 'Refund review',
  requestedBy: 'Order fulfilment',
  expiresAt: minutesFromNow(55),
  argumentsJson: '{\n  "paymentId": "pay_7Q2K",\n  "amount": 480,\n  "currency": "USD",\n  "reason": "Customer returned the order"\n}',
};
