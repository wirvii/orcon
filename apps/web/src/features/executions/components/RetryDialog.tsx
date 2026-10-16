// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, ComingSoon, Dialog, ProblemAlert } from '@orcon/ui';
import { useIntl } from 'react-intl';

import { toProblemDetails } from '../../../app/errors/to-problem-details';

import { runMessages } from './messages';

export interface RetryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The version of the flow the run executed. Absent for a run of a draft. */
  version: number | undefined;
  /** The retry is being sent. */
  pending: boolean;
  /** Why the last try failed, when it did. */
  error: unknown;
  onConfirm: () => void;
}

/**
 * The confirmation before a retry. It says what is repeated and what is kept, because the engine rule (a retry repeats
 * only what failed) is not yet proven by the server: until it is, the dialog does not promise it (SPEC-019 Q10). A
 * server that cannot retry yet is told as such, not as a failure.
 */
export function RetryDialog({ open, onOpenChange, version, pending, error, onConfirm }: RetryDialogProps) {
  const intl = useIntl();
  const problem = error === undefined || error === null ? undefined : toProblemDetails(error);
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={intl.formatMessage(runMessages.retryTitle)}
      description={version === undefined ? intl.formatMessage(runMessages.retryImpactDraft) : intl.formatMessage(runMessages.retryImpact, { version })}
      footer={
        <>
          <Button
            variant="secondary"
            onClick={() => {
              onOpenChange(false);
            }}
          >
            {intl.formatMessage(runMessages.retryCancel)}
          </Button>
          <Button loading={pending} onClick={onConfirm}>
            {intl.formatMessage(runMessages.retryConfirm)}
          </Button>
        </>
      }
    >
      {problem === undefined ? null : problem.code === 'service-unavailable' ? (
        <ComingSoon title={intl.formatMessage(runMessages.retrySoonTitle)} description={intl.formatMessage(runMessages.retrySoonText)} headingLevel={3} />
      ) : (
        <ProblemAlert problem={problem} />
      )}
    </Dialog>
  );
}
