// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { BellRing, CircleCheck, CircleX, type LucideIcon } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { useAnnouncer } from '../canvas/announcer';
import { AlertDialog } from '../components/alert-dialog/AlertDialog';
import { AttentionInboxItem } from '../components/attention-inbox-item/AttentionInboxItem';
import { Banner } from '../components/banner/Banner';
import { Button } from '../components/button/Button';
import { EmptyState } from '../components/empty-state/EmptyState';
import { RelativeTime } from '../components/relative-time/RelativeTime';
import { Tabs } from '../components/tabs/Tabs';
import { cn } from '../lib/cn';

import {
  FIXTURE_NOW,
  attentionRows as fixtureRows,
  notificationRows as fixtureNotifications,
  type AttentionRow,
  type NotificationRow,
} from './fixtures';
import { PageHeader } from './PageHeader';
import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.inbox.title', defaultMessage: 'Needs your attention', description: 'Main heading of the inbox of things that wait for a person.' },
  description: {
    id: 'screen.inbox.description',
    defaultMessage: 'Decisions that flows are waiting for.',
    description: 'One line under the heading of the attention inbox.',
  },
  filters: { id: 'screen.inbox.filters', defaultMessage: 'Who it is for', description: 'Accessible name of the tabs that filter the inbox by who the item is for.' },
  mine: { id: 'screen.inbox.filter.mine', defaultMessage: 'Mine', description: 'Filter: items assigned to the person.' },
  groups: { id: 'screen.inbox.filter.groups', defaultMessage: 'My groups', description: 'Filter: items assigned to a group the person belongs to.' },
  all: { id: 'screen.inbox.filter.all', defaultMessage: 'All', description: 'Filter: every item.' },
  showing: {
    id: 'screen.inbox.showing',
    defaultMessage: '{count, plural, =0 {Nothing to show} one {Showing # item} other {Showing # items}}',
    description: 'Read aloud after the filter changes: how many items are now listed.',
  },
  selected: {
    id: 'screen.inbox.selected',
    defaultMessage: '{count, plural, one {# item selected} other {# items selected}}',
    description: 'Shows how many items are ticked for a batch action.',
  },
  acknowledge: { id: 'screen.inbox.acknowledge', defaultMessage: 'Acknowledge selected', description: 'Button that marks the ticked items as seen by the person.' },
  mixed: {
    id: 'screen.inbox.mixed',
    defaultMessage: 'Batch actions only work on items with the same cause from the same flow. Choose items that match.',
    description: 'Explains why the batch action is disabled when the ticked items differ in cause or flow.',
  },
  confirmTitle: {
    id: 'screen.inbox.confirm.title',
    defaultMessage: '{count, plural, one {Acknowledge # item?} other {Acknowledge # items?}}',
    description: 'Question before acknowledging several items at once. States how many are affected.',
  },
  confirmImpact: {
    id: 'screen.inbox.confirm.impact',
    defaultMessage: 'Every selected item is marked as acknowledged by you. They stay open until someone decides them.',
    description: 'Explains what acknowledging items does and that it does not resolve them.',
  },
  confirm: { id: 'screen.inbox.confirm', defaultMessage: 'Acknowledge', description: 'Button that confirms acknowledging the selected items.' },
  acknowledged: {
    id: 'screen.inbox.acknowledged',
    defaultMessage: '{count, plural, one {# item acknowledged} other {# items acknowledged}}',
    description: 'Read aloud after items were acknowledged.',
  },
  listLabel: { id: 'screen.inbox.list', defaultMessage: 'Items waiting for a person', description: 'Accessible name of the list of items that wait for a person.' },
  emptyTitle: { id: 'screen.inbox.empty.title', defaultMessage: 'Nothing is waiting for you', description: 'Title shown when the inbox has no items.' },
  emptyText: {
    id: 'screen.inbox.empty.text',
    defaultMessage: 'When a flow needs a decision, it appears here.',
    description: 'Text shown when the inbox has no items, saying what will appear.',
  },
  notifications: { id: 'screen.inbox.notifications', defaultMessage: 'Notifications', description: 'Heading of the panel with recent notifications.' },
  notificationsLabel: { id: 'screen.inbox.notifications.list', defaultMessage: 'Recent notifications', description: 'Accessible name of the list of recent notifications.' },
  noteAttention: { id: 'screen.inbox.note.attention', defaultMessage: 'Needs a decision', description: 'Text before a notification that asks for a decision.' },
  noteSuccess: { id: 'screen.inbox.note.success', defaultMessage: 'Finished', description: 'Text before a notification about a finished run.' },
  noteDanger: { id: 'screen.inbox.note.danger', defaultMessage: 'Failed', description: 'Text before a notification about a failed run.' },
  cause: {
    id: 'screen.inbox.cause',
    defaultMessage: '{cause, select, approval {Approval} reviewUncertain {Review of an uncertain operation} failedStep {Failed step} other {Other}}',
    description: 'Name of the cause of an attention item, shown as a label.',
  },
  retry: { id: 'screen.inbox.retry', defaultMessage: 'Try again', description: 'Button that loads the inbox again.' },
  errorTitle: { id: 'screen.inbox.error', defaultMessage: 'The inbox could not be loaded.', description: 'Message shown when loading the attention inbox failed.' },
});

type Filter = 'mine' | 'groups' | 'all';

const noteMeta: Record<NotificationRow['tone'], { icon: LucideIcon; text: typeof messages.noteAttention; color: string }> = {
  attention: { icon: BellRing, text: messages.noteAttention, color: 'text-status-attention' },
  success: { icon: CircleCheck, text: messages.noteSuccess, color: 'text-status-success' },
  danger: { icon: CircleX, text: messages.noteDanger, color: 'text-status-danger' },
};

export interface AttentionInboxScreenProps {
  items?: readonly AttentionRow[];
  notifications?: readonly NotificationRow[];
  now?: string;
  status?: 'ready' | 'error';
  onAcknowledge?: (ids: readonly string[]) => void;
  onRetry?: () => void;
}

/**
 * S6: the attention inbox and the notifications panel. Filters (mine, my groups,
 * all), age, cause, escalation, who acknowledged, an overdue marker with icon and
 * text, and batch actions that only accept items of the same cause and flow, behind
 * a confirmation that states how many items are affected. Announcements are polite and
 * joined, and never move focus. Decisions never live in toasts.
 */
export function AttentionInboxScreen({
  items = fixtureRows,
  notifications = fixtureNotifications,
  now = FIXTURE_NOW,
  status = 'ready',
  onAcknowledge,
  onRetry,
}: AttentionInboxScreenProps) {
  const intl = useIntl();
  const { announce, region } = useAnnouncer();
  const [filter, setFilter] = useState<Filter>('all');
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [confirming, setConfirming] = useState(false);
  const [acknowledgedIds, setAcknowledgedIds] = useState<ReadonlySet<string>>(new Set());

  const visible = useMemo(() => items.filter((item) => filter === 'all' || item.group === filter), [items, filter]);
  const chosen = items.filter((item) => selected.has(item.id));
  const sameCauseAndFlow = chosen.length > 0 && chosen.every((item) => item.cause === chosen[0]?.cause && item.flowName === chosen[0]?.flowName);

  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    announce(intl.formatMessage(messages.showing, { count: visible.length }));
  }, [visible.length, filter, intl, announce]);

  const toggle = (id: string, on: boolean) =>
    setSelected((current) => {
      const next = new Set(current);
      if (on) next.add(id);
      else next.delete(id);
      return next;
    });

  const acknowledge = () => {
    const ids = chosen.map((item) => item.id);
    setAcknowledgedIds((current) => new Set([...current, ...ids]));
    setSelected(new Set());
    onAcknowledge?.(ids);
    announce(intl.formatMessage(messages.acknowledged, { count: ids.length }));
  };

  const list = (
    <div className="flex flex-col">
      {chosen.length > 0 ? (
        <div className="bg-surface-sunken flex flex-wrap items-center gap-3 rounded-md p-2" data-batch-bar>
          <span className="font-medium">{intl.formatMessage(messages.selected, { count: chosen.length })}</span>
          <Button variant="secondary" disabled={!sameCauseAndFlow} onClick={() => setConfirming(true)}>
            {intl.formatMessage(messages.acknowledge)}
          </Button>
          {!sameCauseAndFlow ? <span className="text-fg-muted text-sm">{intl.formatMessage(messages.mixed)}</span> : null}
        </div>
      ) : null}
      {visible.length === 0 ? (
        <EmptyState title={intl.formatMessage(messages.emptyTitle)} description={intl.formatMessage(messages.emptyText)} headingLevel={3} />
      ) : (
        <div role="list" aria-label={intl.formatMessage(messages.listLabel)}>
          {visible.map((item) => (
            <div role="listitem" key={item.id}>
              <p className="text-fg-muted px-2 pt-2 text-xs font-medium">{intl.formatMessage(messages.cause, { cause: item.cause })}</p>
              <AttentionInboxItem
                title={item.title}
                href={`#attention/${item.id}`}
                flowName={item.flowName}
                createdAt={item.createdAt}
                now={now}
                escalationLevel={item.escalationLevel}
                acknowledgedBy={acknowledgedIds.has(item.id) ? undefined : item.acknowledgedBy}
                overdue={item.overdue}
                selected={selected.has(item.id)}
                onSelectedChange={(on) => toggle(item.id, on)}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );

  return (
    <ProductShell current="attention" attentionCount={items.length}>
      <PageHeader title={intl.formatMessage(messages.title)} description={intl.formatMessage(messages.description)} />
      {status === 'error' ? (
        <Banner
          tone="danger"
          title={intl.formatMessage(messages.errorTitle)}
          action={
            <Button variant="secondary" onClick={onRetry}>
              {intl.formatMessage(messages.retry)}
            </Button>
          }
        />
      ) : (
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start">
          <div className="bg-surface border-border min-w-0 flex-1 rounded-lg border p-2">
            <Tabs
              label={intl.formatMessage(messages.filters)}
              value={filter}
              onValueChange={(value) => setFilter(value as Filter)}
              tabs={[
                { value: 'all', label: intl.formatMessage(messages.all), content: list },
                { value: 'mine', label: intl.formatMessage(messages.mine), content: list },
                { value: 'groups', label: intl.formatMessage(messages.groups), content: list },
              ]}
            />
          </div>
          <aside aria-labelledby="notifications-heading" className="bg-surface border-border rounded-lg border p-3 lg:w-panel-width lg:shrink-0">
            <h2 id="notifications-heading" className="text-md font-semibold wrap-anywhere">
              {intl.formatMessage(messages.notifications)}
            </h2>
            <ul aria-label={intl.formatMessage(messages.notificationsLabel)} className="mt-2 flex flex-col gap-3">
              {notifications.map((note) => {
                const meta = noteMeta[note.tone];
                const Icon = meta.icon;
                return (
                  <li key={note.id} className="flex items-start gap-2">
                    <Icon aria-hidden className={cn('mt-1 size-icon-16 shrink-0', meta.color)} />
                    <div className="min-w-0">
                      <p className={cn('text-sm font-medium wrap-anywhere', meta.color)}>{intl.formatMessage(meta.text)}</p>
                      <p className="wrap-anywhere">{note.text}</p>
                      <p className="text-fg-muted text-sm">
                        <RelativeTime value={note.at} now={now} />
                      </p>
                    </div>
                  </li>
                );
              })}
            </ul>
          </aside>
        </div>
      )}
      {region}
      <AlertDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={intl.formatMessage(messages.confirmTitle, { count: chosen.length })}
        impact={intl.formatMessage(messages.confirmImpact)}
        confirmLabel={intl.formatMessage(messages.confirm)}
        destructive={false}
        onConfirm={acknowledge}
      />
    </ProductShell>
  );
}
