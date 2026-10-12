// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { Banner } from '../components/banner/Banner';
import { BrandMark } from '../components/brand-mark/BrandMark';
import { Button } from '../components/button/Button';
import { CodeBlock } from '../components/code-block/CodeBlock';
import { DateTime } from '../components/date-time/DateTime';
import { ErrorSummary } from '../components/error-summary/ErrorSummary';
import { FormField } from '../components/form-field/FormField';
import { ProblemAlert } from '../components/problem-alert/ProblemAlert';
import { fieldIdForName, type ProblemDetails } from '../components/problem-alert/problem';
import { RedactedValue } from '../components/redacted-value/RedactedValue';
import { RelativeTime } from '../components/relative-time/RelativeTime';
import { Textarea } from '../components/textarea/Textarea';

import { FIXTURE_NOW, approvalRequest, type ApprovalRequest } from './fixtures';

const messages = defineMessages({
  title: { id: 'screen.approval.title', defaultMessage: 'Approval needed', description: 'Main heading of the page where a person approves or denies a request.' },
  intro: {
    id: 'screen.approval.intro',
    defaultMessage: 'The flow {flow} wants to run a tool and waits for your decision.',
    description: 'Explains who asks for the approval. {flow} is the name of the flow.',
  },
  tool: { id: 'screen.approval.tool', defaultMessage: 'Tool', description: 'Label before the name of the tool the flow wants to run.' },
  requestedBy: { id: 'screen.approval.requestedBy', defaultMessage: 'Requested by', description: 'Label before the flow that asks for the approval.' },
  expires: { id: 'screen.approval.expires', defaultMessage: 'Expires', description: 'Label before when the request stops being valid.' },
  expiredNotice: {
    id: 'screen.approval.expired',
    defaultMessage: 'This request has expired. Nobody can decide it any more.',
    description: 'Notice shown when the time to decide an approval request is over.',
  },
  argumentsLabel: { id: 'screen.approval.arguments', defaultMessage: 'Arguments', description: 'Heading of the data the tool will receive.' },
  argumentsName: { id: 'screen.approval.arguments.name', defaultMessage: 'Arguments the tool will receive', description: 'Accessible name of the block that shows the tool arguments.' },
  approve: { id: 'screen.approval.approve', defaultMessage: 'Approve', description: 'Button that approves the request.' },
  deny: { id: 'screen.approval.deny', defaultMessage: 'Deny…', description: 'Button that starts denying the request. A reason is asked next.' },
  edit: { id: 'screen.approval.edit', defaultMessage: 'Edit arguments…', description: 'Button that lets the person change the arguments before approving.' },
  reasonLabel: { id: 'screen.approval.reason', defaultMessage: 'Reason for denying', description: 'Label of the field where a person explains why the request is denied.' },
  reasonHelp: {
    id: 'screen.approval.reason.help',
    defaultMessage: 'The flow and the person who set it up will see this.',
    description: 'Helper text under the reason field: who reads the reason.',
  },
  reasonRequired: {
    id: 'screen.approval.reason.error',
    defaultMessage: 'Write why you are denying this request.',
    description: 'Error under the reason field when it is empty.',
  },
  confirmDeny: { id: 'screen.approval.confirmDeny', defaultMessage: 'Deny the request', description: 'Button that sends the denial with its reason.' },
  argsLabel: { id: 'screen.approval.args.label', defaultMessage: 'Arguments (JSON)', description: 'Label of the field where the person edits the tool arguments.' },
  argsInvalid: {
    id: 'screen.approval.args.error',
    defaultMessage: 'These arguments are not valid JSON. Fix the text and try again.',
    description: 'Error under the arguments field when the text is not valid JSON.',
  },
  approveEdited: { id: 'screen.approval.approveEdited', defaultMessage: 'Approve with these arguments', description: 'Button that approves the request with the edited arguments.' },
  cancel: { id: 'screen.approval.cancel', defaultMessage: 'Cancel', description: 'Button that closes the deny or edit form without sending anything.' },
  hiddenEdit: {
    id: 'screen.approval.hiddenEdit',
    defaultMessage: 'You cannot edit arguments you are not allowed to see.',
    description: 'Explains why editing is not offered when the arguments are hidden from the person.',
  },
  outcome: {
    id: 'screen.approval.outcome',
    defaultMessage: '{decision, select, approved {Approved} other {Denied}} by {name}',
    description: 'The decision that is already in effect on a request someone else decided. {name} is the person.',
  },
  done: {
    id: 'screen.approval.done',
    defaultMessage: '{decision, select, approved {You approved this request.} other {You denied this request.}}',
    description: 'Confirmation shown after the person decided the request.',
  },
});

export interface ApprovalOutcome {
  decision: 'approved' | 'denied';
  by: string;
  at: string;
}

export interface ApprovalScreenProps {
  request?: ApprovalRequest;
  /** The person lacks `execution:read-data`: the arguments are shown as hidden and cannot be edited. */
  argumentsHidden?: boolean;
  /** The instant relative times are measured from. */
  now?: string;
  /** A 409 answer: someone else decided first. Shows the decision in effect instead of a generic error. */
  conflict?: { problem: ProblemDetails; outcome: ApprovalOutcome };
  onApprove?: (argumentsJson?: string) => void;
  onDeny?: (reason: string) => void;
}

type Mode = 'review' | 'deny' | 'edit' | 'done';

/**
 * S7: the approval page. Shows the tool, its arguments (or a hidden notice), the
 * time left in relative terms, and three ways to answer: approve, deny with a reason
 * or edit the arguments first. Mistakes are explained next to the field and in a
 * summary, and what the person typed is never cleared. A 409 shows the decision that
 * is already in effect. No mascot: this is a security decision.
 */
export function ApprovalScreen({ request = approvalRequest, argumentsHidden = false, now = FIXTURE_NOW, conflict, onApprove, onDeny }: ApprovalScreenProps) {
  const intl = useIntl();
  const [mode, setMode] = useState<Mode>('review');
  const [decision, setDecision] = useState<'approved' | 'denied'>('approved');
  const [reason, setReason] = useState('');
  const [argsText, setArgsText] = useState(request.argumentsJson);
  const [errors, setErrors] = useState<{ fieldId: string; message: string }[]>([]);
  const expired = Date.parse(request.expiresAt) <= Date.parse(now);
  const locked = expired || conflict !== undefined || mode === 'done';

  const submitDeny = () => {
    if (reason.trim() === '') {
      setErrors([{ fieldId: fieldIdForName('/reason'), message: intl.formatMessage(messages.reasonRequired) }]);
      return;
    }
    setErrors([]);
    setDecision('denied');
    setMode('done');
    onDeny?.(reason);
  };

  const submitEdited = () => {
    try {
      JSON.parse(argsText);
    } catch {
      setErrors([{ fieldId: fieldIdForName('/arguments'), message: intl.formatMessage(messages.argsInvalid) }]);
      return;
    }
    setErrors([]);
    setDecision('approved');
    setMode('done');
    onApprove?.(argsText);
  };

  const approve = () => {
    setDecision('approved');
    setMode('done');
    onApprove?.();
  };

  const reasonError = errors.find((error) => error.fieldId === fieldIdForName('/reason'))?.message;
  const argsError = errors.find((error) => error.fieldId === fieldIdForName('/arguments'))?.message;

  return (
    <div className="bg-app text-fg flex min-h-screen flex-col">
      <header className="bg-surface border-border flex h-row min-h-target items-center border-b px-4">
        <BrandMark />
      </header>
      <main className="mx-auto flex w-full max-w-screen-md flex-1 flex-col gap-4 p-4">
        <div>
          <h1 className="text-xl font-semibold break-words">{intl.formatMessage(messages.title)}</h1>
          <p className="text-fg-muted break-words">{intl.formatMessage(messages.intro, { flow: request.flowName })}</p>
        </div>

        {conflict ? (
          <ProblemAlert
            problem={conflict.problem}
            currentResult={
              <span>
                {intl.formatMessage(messages.outcome, { decision: conflict.outcome.decision, name: conflict.outcome.by })}
                {' '}
                <RelativeTime value={conflict.outcome.at} now={now} />
              </span>
            }
          />
        ) : null}
        {expired && !conflict ? <Banner tone="warning" title={intl.formatMessage(messages.expiredNotice)} /> : null}
        {mode === 'done' ? <Banner tone="success" title={intl.formatMessage(messages.done, { decision })} /> : null}
        {errors.length > 0 ? <ErrorSummary errors={errors} focusOnMount /> : null}

        <dl className="bg-surface border-border grid gap-3 rounded-lg border p-4 sm:grid-cols-3">
          <div>
            <dt className="text-fg-muted text-sm break-words">{intl.formatMessage(messages.tool)}</dt>
            <dd className="font-mono text-mono break-all">{request.tool}</dd>
          </div>
          <div>
            <dt className="text-fg-muted text-sm break-words">{intl.formatMessage(messages.requestedBy)}</dt>
            <dd className="break-words">{request.requestedBy}</dd>
          </div>
          <div>
            <dt className="text-fg-muted text-sm break-words">{intl.formatMessage(messages.expires)}</dt>
            <dd>
              <RelativeTime value={request.expiresAt} now={now} />
              <span className="text-fg-muted block text-sm">
                <DateTime value={request.expiresAt} dateStyle="medium" timeStyle="short" showTimeZone />
              </span>
            </dd>
          </div>
        </dl>

        <section aria-labelledby="approval-arguments" className="flex flex-col gap-2">
          <h2 id="approval-arguments" className="text-lg font-semibold break-words">
            {intl.formatMessage(messages.argumentsLabel)}
          </h2>
          {argumentsHidden ? (
            <RedactedValue />
          ) : (
            <CodeBlock code={request.argumentsJson} label={intl.formatMessage(messages.argumentsName)} copyable />
          )}
        </section>

        {mode === 'deny' ? (
          <form
            className="flex flex-col gap-3"
            noValidate
            onSubmit={(event) => {
              event.preventDefault();
              submitDeny();
            }}
          >
            <FormField label={intl.formatMessage(messages.reasonLabel)} name="/reason" required description={intl.formatMessage(messages.reasonHelp)} error={reasonError}>
              <Textarea value={reason} onChange={(event) => setReason(event.target.value)} />
            </FormField>
            <div className="flex flex-wrap gap-2">
              <Button type="submit" variant="danger">
                {intl.formatMessage(messages.confirmDeny)}
              </Button>
              <Button variant="secondary" onClick={() => { setMode('review'); setErrors([]); }}>
                {intl.formatMessage(messages.cancel)}
              </Button>
            </div>
          </form>
        ) : null}

        {mode === 'edit' ? (
          <form
            className="flex flex-col gap-3"
            noValidate
            onSubmit={(event) => {
              event.preventDefault();
              submitEdited();
            }}
          >
            <FormField label={intl.formatMessage(messages.argsLabel)} name="/arguments" required error={argsError}>
              <Textarea value={argsText} onChange={(event) => setArgsText(event.target.value)} className="font-mono text-mono" rows={8} />
            </FormField>
            <div className="flex flex-wrap gap-2">
              <Button type="submit">{intl.formatMessage(messages.approveEdited)}</Button>
              <Button variant="secondary" onClick={() => { setMode('review'); setErrors([]); }}>
                {intl.formatMessage(messages.cancel)}
              </Button>
            </div>
          </form>
        ) : null}

        {mode === 'review' ? (
          <div className="flex flex-col gap-2">
            <div className="flex flex-wrap gap-2">
              <Button disabled={locked} onClick={approve}>
                {intl.formatMessage(messages.approve)}
              </Button>
              <Button variant="secondary" disabled={locked} onClick={() => setMode('deny')}>
                {intl.formatMessage(messages.deny)}
              </Button>
              <Button variant="secondary" disabled={locked || argumentsHidden} onClick={() => setMode('edit')}>
                {intl.formatMessage(messages.edit)}
              </Button>
            </div>
            {argumentsHidden ? <p className="text-fg-muted text-sm">{intl.formatMessage(messages.hiddenEdit)}</p> : null}
          </div>
        ) : null}
      </main>
    </div>
  );
}
