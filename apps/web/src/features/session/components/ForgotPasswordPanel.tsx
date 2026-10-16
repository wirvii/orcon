// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Banner, Button, FormField, Input } from '@orcon/ui';
import { ArrowLeft, Mail } from 'lucide-react';
import { useEffect, useRef, useState, type SyntheticEvent } from 'react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  back: { id: 'session.forgot.back', defaultMessage: 'Back', description: 'Button on the password recovery panel that returns to the sign-in form.' },
  title: { id: 'session.forgot.title', defaultMessage: 'Recover your password', description: 'Main heading of the password recovery panel of the sign-in page.' },
  descriptionByEmail: {
    id: 'session.forgot.descriptionByEmail',
    defaultMessage: 'We will send you a link to create a new one. It expires in 30 minutes.',
    description: 'One line under the heading of the recovery panel when recovery by email works.',
  },
  descriptionByAdministrator: {
    id: 'session.forgot.descriptionByAdministrator',
    defaultMessage: 'Recovery by email is not available yet.',
    description: 'One line under the heading of the recovery panel when recovery by email does not exist yet.',
  },
  administratorTitle: {
    id: 'session.forgot.administratorTitle',
    defaultMessage: 'Ask whoever administers your organization',
    description: 'Title of the explanation on the recovery panel: who can set a new password today.',
  },
  administratorText: {
    id: 'session.forgot.administratorText',
    defaultMessage: 'They can set a new password for your account. On the server this is done with the command {command}.',
    description: 'Explains who can set a new password while recovery by email does not exist. {command} is the command, not translated.',
  },
  email: { id: 'session.forgot.email', defaultMessage: 'Email', description: 'Label of the email field of the recovery panel.' },
  send: { id: 'session.forgot.send', defaultMessage: 'Send the link', description: 'Button that asks for the recovery link.' },
  sentTitle: { id: 'session.forgot.sentTitle', defaultMessage: 'Check your email.', description: 'Title of the confirmation after asking for a recovery link. It is the same for every address.' },
  sentText: {
    id: 'session.forgot.sentText',
    defaultMessage: 'If an account exists with that email, you will get the link in a minute. Check your spam folder too.',
    description: 'Text of the confirmation after asking for a recovery link. It never says whether the account exists.',
  },
});

/** The command an administrator runs on the server to set a password (SPEC-016 R-SH-CMD-02). It is not translated. */
const SET_PASSWORD_COMMAND = 'orcon accounts set-password';

export interface ForgotPasswordPanelProps {
  /** Returns to the sign-in form. */
  onBack: () => void;
  /**
   * Asks the server to email a recovery link. It exists only once the server has that operation (SPEC-004 email
   * flows); while it is missing the panel says that an administrator can set a new password and asks for nothing.
   */
  requestReset?: ((email: string) => Promise<void>) | undefined;
  /** The email typed on the sign-in form, so the person does not type it twice. */
  initialEmail?: string | undefined;
}

/**
 * The password recovery panel of the sign-in page (SPEC-019 R-UX-LGN-05). Without the reset operation it is honest:
 * it does not ask for an email it cannot use, it says who can set a new password. With the operation it asks for the
 * email and always answers the same way, so it never reveals whether an account exists.
 */
export function ForgotPasswordPanel({ onBack, requestReset, initialEmail = '' }: ForgotPasswordPanelProps) {
  const intl = useIntl();
  const heading = useRef<HTMLHeadingElement>(null);
  const [email, setEmail] = useState(initialEmail);
  const [sent, setSent] = useState(false);
  const [sending, setSending] = useState(false);

  // The panel replaces the form: focus moves to its heading so a screen reader says where the person is.
  useEffect(() => {
    heading.current?.focus();
  }, []);

  const submit = async (event: SyntheticEvent) => {
    event.preventDefault();
    if (requestReset === undefined || email.trim() === '') return;
    setSending(true);
    try {
      await requestReset(email.trim());
    } catch {
      // The answer is the same whatever happened to the request: a different one would say whether the account exists.
    } finally {
      setSending(false);
      setSent(true);
    }
  };

  return (
    <div className="flex flex-col gap-5">
      <div>
        <Button variant="ghost" onClick={onBack} className="-ml-2">
          <ArrowLeft aria-hidden className="size-icon-16" />
          {intl.formatMessage(messages.back)}
        </Button>
      </div>
      <div className="flex flex-col gap-1">
        <h1 ref={heading} tabIndex={-1} className="text-display font-bold wrap-anywhere outline-hidden">
          {intl.formatMessage(messages.title)}
        </h1>
        <p className="text-fg-muted wrap-anywhere">{intl.formatMessage(requestReset === undefined ? messages.descriptionByAdministrator : messages.descriptionByEmail)}</p>
      </div>

      {requestReset === undefined ? (
        <Banner tone="info" title={intl.formatMessage(messages.administratorTitle)}>
          {intl.formatMessage(messages.administratorText, { command: <code className="font-mono text-mono">{SET_PASSWORD_COMMAND}</code> })}
        </Banner>
      ) : sent ? (
        <Banner tone="success" title={intl.formatMessage(messages.sentTitle)}>
          {intl.formatMessage(messages.sentText)}
        </Banner>
      ) : (
        <form noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <FormField label={intl.formatMessage(messages.email)} name="/email" required>
            <Input
              type="email"
              autoComplete="username"
              spellCheck={false}
              startIcon={Mail}
              value={email}
              onChange={(event) => {
                setEmail(event.target.value);
              }}
            />
          </FormField>
          <Button type="submit" loading={sending} className="w-full">
            {intl.formatMessage(messages.send)}
          </Button>
        </form>
      )}
    </div>
  );
}
