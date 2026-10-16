// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Info, ShieldCheck, type LucideIcon } from 'lucide-react';
import { useIntl } from 'react-intl';

import { createMessages } from '../model/messages';
import type { Service } from '../model/services';

/**
 * The right column of the create dialog: the steps to get what the form asks for, next to the form so nobody
 * has to leave for the documentation, a note for people who come from n8n and why the domains matter.
 */
export function ConnectionGuide({ service }: { service: Service }) {
  const intl = useIntl();
  return (
    <aside aria-label={intl.formatMessage(createMessages.guideHeading)} className="flex flex-col gap-3">
      <h3 className="text-fg-muted text-sm font-medium">{intl.formatMessage(createMessages.guideHeading)}</h3>
      <ol className="flex list-decimal flex-col gap-2 pl-5">
        {service.steps.map((step) => (
          <li key={step.id}>{intl.formatMessage(step)}</li>
        ))}
      </ol>
      <Note icon={Info} title={intl.formatMessage(createMessages.fromN8nTitle)}>
        {intl.formatMessage(service.fromN8n)}
      </Note>
      <Note icon={ShieldCheck} title={intl.formatMessage(createMessages.protectionTitle)}>
        {intl.formatMessage(service.protection)}
      </Note>
    </aside>
  );
}

/** A static note of the guide. It is not a live region: it is part of the page, not news. */
function Note({ icon: Icon, title, children }: { icon: LucideIcon; title: string; children: string }) {
  return (
    <div className="border-border bg-surface-sunken flex items-start gap-2 rounded-md border p-3 text-sm">
      <Icon aria-hidden className="text-fg-muted mt-1 size-icon-16 shrink-0" />
      <p className="wrap-anywhere">
        <b className="font-semibold">{title}</b> {children}
      </p>
    </div>
  );
}
