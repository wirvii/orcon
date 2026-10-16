// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, UncertainResolutionPanel, type UncertainResolutionAction } from '@orcon/ui';
import { useIntl } from 'react-intl';

import { runMessages } from './messages';

export interface ReviewPanelProps {
  /** What the panel names: the flow, because the server does not say which step to review. */
  name: string;
}

/**
 * Where a person decides what to do with a run that has an operation of unknown outcome ("Needs review"). It shows the
 * resolutions the contract defines, without a retry as the main action, and says that they cannot be used from this
 * page yet: the server answers 503 to `resolveExternalOperation` and no operation lists the identifier it needs. The
 * choices are drawn disabled, never as working buttons, until that exists.
 */
export function ReviewPanel({ name }: ReviewPanelProps) {
  const intl = useIntl();
  const actions: UncertainResolutionAction[] = [
    { id: 'VERIFY_AGAIN', kind: 'other', label: intl.formatMessage(runMessages.verifyAgain) },
    {
      id: 'MARK_SUCCEEDED',
      kind: 'mark-applied',
      label: intl.formatMessage(runMessages.markApplied),
      requiresRiskAck: true,
      riskText: intl.formatMessage(runMessages.markAppliedRisk),
    },
    {
      id: 'MARK_FAILED_AND_REPEAT',
      kind: 'repeat',
      label: intl.formatMessage(runMessages.markNotApplied),
      requiresRiskAck: true,
      riskText: intl.formatMessage(runMessages.markNotAppliedRisk),
    },
  ];
  return (
    <div className="flex flex-col gap-3" data-review-panel>
      {/* A disabled fieldset disables every button and checkbox inside it. */}
      <fieldset disabled className="min-w-0">
        <UncertainResolutionPanel nodeName={name} actions={actions} />
      </fieldset>
      <Banner tone="info" title={intl.formatMessage(runMessages.reviewSoon)} />
    </div>
  );
}
