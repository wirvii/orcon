// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, Banner, Button } from '@orcon/ui';
import { ChevronDown, ChevronRight, ListChecks } from 'lucide-react';
import { useId, useState } from 'react';
import { useIntl } from 'react-intl';

import { useNoticeChannel } from '../../components/notice-channel';
import { availableUpdate } from '../../model/connector-versions';
import type { FlowNode } from '../../model/flow-document';
import { localeOf } from '../../model/node-manifests';

import { configMessages } from './messages';
import { useNodeActions } from './use-node-config';

const kinds = {
  added: configMessages.updateAdded,
  changed: configMessages.updateChanged,
  removed: configMessages.updateRemoved,
} as const;

/**
 * "New version available" (SPEC-014 R-ED-CFG-05, SPEC-003 R-VER-03): the changelog between the
 * version the node uses and the newest installed, a mark on every change that can alter what the
 * step does, and "Update this node", which edits the draft as one history entry.
 */
export function ConnectorUpdate({ node }: { node: FlowNode }) {
  const intl = useIntl();
  const language = localeOf(intl.locale);
  const actions = useNodeActions(node.id);
  const notices = useNoticeChannel();
  const update = availableUpdate(node);
  const [open, setOpen] = useState(false);
  const changelogId = useId();
  if (!update) return null;
  const hasChanges = update.releases.some((release) => release.changes.length > 0);
  const Chevron = open ? ChevronDown : ChevronRight;
  return (
    <Banner
      tone="info"
      title={intl.formatMessage(configMessages.updateTitle)}
      action={
        <Button
          onClick={() => {
            actions.updateConnector(update.latest.version, update.latest.digest);
            notices.notify('info', intl.formatMessage(configMessages.updated, { name: node.name, version: update.latest.version }));
          }}
        >
          {intl.formatMessage(configMessages.updateAction)}
        </Button>
      }
    >
      <div className="flex flex-col gap-2">
        <p>{intl.formatMessage(configMessages.updateText, { from: update.from, latest: update.latest.version })}</p>
        <p>{intl.formatMessage(update.needsReview ? configMessages.updateNeedsReview : configMessages.updateMechanical)}</p>
        {hasChanges ? (
          <button
            type="button"
            aria-expanded={open}
            aria-controls={changelogId}
            onClick={() => {
              setOpen(!open);
            }}
            className="inline-flex min-h-target items-center gap-1 self-start rounded-sm font-medium underline underline-offset-2"
          >
            <Chevron aria-hidden className="size-icon-16" />
            {intl.formatMessage(configMessages.updateChangelog)}
          </button>
        ) : null}
        {hasChanges && open ? (
          <section id={changelogId} aria-label={intl.formatMessage(configMessages.updateChangelog)} className="flex flex-col gap-2">
            {update.releases
              .filter((release) => release.changes.length > 0)
              .map((release) => (
                <div key={release.version} className="flex flex-col gap-1">
                  <h4 className="font-semibold">{intl.formatMessage(configMessages.updateRelease, { version: release.version })}</h4>
                  <ul className="flex flex-col gap-1">
                    {release.changes.map((change) => (
                      <li key={change.text.en} className="flex flex-wrap items-start gap-x-2">
                        <span className="font-medium">{intl.formatMessage(kinds[change.kind])}:</span>
                        <span>{change.text[language]}</span>
                        {change.review === true ? <Badge icon={<ListChecks aria-hidden className="size-icon-16" />}>{intl.formatMessage(configMessages.updateReview)}</Badge> : null}
                      </li>
                    ))}
                  </ul>
                </div>
              ))}
          </section>
        ) : null}
        <p className="text-xs">{intl.formatMessage(configMessages.updateNote)}</p>
      </div>
    </Banner>
  );
}
