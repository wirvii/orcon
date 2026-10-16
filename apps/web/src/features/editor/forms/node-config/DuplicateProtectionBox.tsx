// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, ExpressionChip } from '@orcon/ui';
import { ShieldCheck } from 'lucide-react';
import { useIntl } from 'react-intl';

import { mayRepeat, neverSendsTwice, type OperationSafety } from '../../model/effect-safety';

import { configMessages } from './messages';

export interface DuplicateProtectionBoxProps {
  /** What the operation of the step declares about repeating it. */
  safety: OperationSafety | undefined;
  /** The name of the service the step talks to, for the sentence. */
  service: string;
}

/**
 * The box about sending twice (SPEC-019 R-UX-NDV-06). A step protected by a unique key says what Orcon does when it is
 * retried and shows the key as a chip; one that checks with the service first says that; a step that cannot be repeated
 * safely says so in amber and that an unknown result leaves it for a person. A step that changes nothing outside has no box.
 */
export function DuplicateProtectionBox({ safety, service }: DuplicateProtectionBoxProps) {
  const intl = useIntl();
  if (neverSendsTwice(safety) && safety) {
    return (
      <div data-duplicate-protection="protected">
        <Banner tone="success" icon={ShieldCheck} title={intl.formatMessage(configMessages.dupTitle)}>
          <p>{intl.formatMessage(safety.safetyLevel === 'IDEMPOTENCY_KEY' ? configMessages.dupKey : configMessages.dupReconcile, { service })}</p>
          {safety.safetyLevel === 'IDEMPOTENCY_KEY' && safety.key ? (
            <p className="mt-1">
              <ExpressionChip kind="path" label={safety.key} />
            </p>
          ) : null}
        </Banner>
      </div>
    );
  }
  if (mayRepeat(safety)) {
    return (
      <div data-duplicate-protection="unsafe">
        <Banner tone="warning" title={intl.formatMessage(configMessages.dupUnsafeTitle)}>
          {intl.formatMessage(configMessages.dupUnsafeText)}
        </Banner>
      </div>
    );
  }
  return null;
}
