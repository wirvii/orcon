// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { BrandHero, Button, ErrorSummary, FormField, Input, Link, Mascot, ProblemAlert, fieldIdForName, type ProblemDetails } from '@orcon/ui';
import { getRouteApi, Link as RouterLink, useNavigate } from '@tanstack/react-router';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { useForm } from 'react-hook-form';
import { defineMessages, useIntl } from 'react-intl';
import { z } from 'zod';

import { formatIssue } from '../../../app/forms/zod-messages';
import { ConnectionBanners } from '../../../app/shell/ConnectionBanners';
import { toProblemDetails } from '../../../app/errors/to-problem-details';
import { markFirstUsefulScreen } from '../../../app/perf/marks';
import { ApiProblem } from '../../../app/api/http';
import { useSignIn } from '../api/mutations';
import { safeRedirect } from '../model/redirect';

const messages = defineMessages({
  title: {
    id: 'session.signIn.title',
    defaultMessage: 'Sign in',
    description: 'Main heading of the sign-in page.',
  },
  description: {
    id: 'session.signIn.description',
    defaultMessage: 'Use the email address and password of your account.',
    description: 'One line under the heading of the sign-in page: which credentials to use.',
  },
  email: { id: 'session.signIn.email', defaultMessage: 'Email', description: 'Label of the email field on the sign-in page.' },
  password: { id: 'session.signIn.password', defaultMessage: 'Password', description: 'Label of the password field on the sign-in page.' },
  showPassword: {
    id: 'session.signIn.showPassword',
    defaultMessage: 'Show password',
    description: 'Button that makes the typed password visible on the sign-in page.',
  },
  hidePassword: {
    id: 'session.signIn.hidePassword',
    defaultMessage: 'Hide password',
    description: 'Button that hides the typed password again on the sign-in page.',
  },
  submit: { id: 'session.signIn.submit', defaultMessage: 'Sign in', description: 'Button that sends the sign-in form.' },
  invalid: {
    id: 'session.signIn.invalid',
    defaultMessage: 'Email or password is incorrect.',
    description: 'Shown after a failed sign-in. It never says which of the two is wrong.',
  },
  throttled: {
    id: 'session.signIn.throttled',
    defaultMessage: 'Too many attempts. Wait {minutes, plural, one {# minute} other {# minutes}} and try again.',
    description: 'Shown when the server stops sign-in attempts for an account for a while. The minutes come from the server.',
  },
  languageLink: {
    id: 'session.signIn.languageLink',
    defaultMessage: 'Language and appearance',
    description: 'Link on the sign-in page to the language and appearance settings, which need no sign-in.',
  },
});

const signInSchema = z.object({
  email: z.string().min(1).max(320).pipe(z.email()),
  password: z.string().min(1).max(1024),
});

type SignInValues = z.infer<typeof signInSchema>;

/** Why the last attempt failed. A failed credential is one case: the page never says which part was wrong. */
type Failure = { kind: 'invalid' } | { kind: 'throttled'; seconds: number } | { kind: 'server'; problem: ProblemDetails };

const route = getRouteApi('/login');

/**
 * The sign-in page (SPEC-016 R-SH-UI-01). It is the only page a person without a session can open,
 * so it sets its own frame. A success goes to the landing, which picks the organization, and the
 * page the person came from is kept when it is a safe address.
 */
export function SignInScreen() {
  const intl = useIntl();
  const navigate = useNavigate();
  const search = route.useSearch();
  const signIn = useSignIn();
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const [revealed, setRevealed] = useState(false);

  const form = useForm<SignInValues>({
    resolver: zodResolver(signInSchema, { error: (issue) => formatIssue(intl, issue) }),
    defaultValues: { email: '', password: '' },
    mode: 'onTouched',
    reValidateMode: 'onChange',
  });
  const { errors, submitCount } = form.formState;

  useEffect(() => {
    markFirstUsefulScreen('/login');
  }, []);

  const submit = form.handleSubmit(async (values) => {
    setFailure(undefined);
    try {
      await signIn.mutateAsync(values);
    } catch (error) {
      if (error instanceof ApiProblem && error.status === 401) {
        // The password is cleared; the email stays, so the person only retypes what is secret.
        form.setValue('password', '');
        setFailure({ kind: 'invalid' });
      } else if (error instanceof ApiProblem && error.status === 429) {
        setFailure({ kind: 'throttled', seconds: error.retryAfterSeconds ?? 60 });
      } else {
        setFailure({ kind: 'server', problem: toProblemDetails(error) });
      }
      return;
    }
    const redirect = safeRedirect(search.redirect);
    await navigate({ to: '/', search: redirect === undefined ? {} : { redirect } });
  });

  const summary = Object.entries(errors).flatMap(([name, error]) =>
    error.message ? [{ fieldId: fieldIdForName(`/${name}`), message: error.message }] : [],
  );

  return (
    <main className="mx-auto flex min-h-dvh w-full max-w-screen-sm flex-col justify-center gap-6 p-4 md:p-8">
      <ConnectionBanners />
      <div className="flex flex-col items-center gap-4">
        <Mascot pose="welcome" />
        <BrandHero />
      </div>

      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold wrap-anywhere">{intl.formatMessage(messages.title)}</h1>
        <p className="text-fg-muted wrap-anywhere">{intl.formatMessage(messages.description)}</p>
      </div>

      {failure ? <FailureAlert failure={failure} /> : null}

      <form noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
        {submitCount > 0 ? <ErrorSummary errors={summary} focusOnMount /> : null}

        <FormField
          label={intl.formatMessage(messages.email)}
          name="/email"
          required
          error={errors.email?.message}
        >
          <Input type="email" autoComplete="username" spellCheck={false} {...form.register('email')} />
        </FormField>

        <FormField label={intl.formatMessage(messages.password)} name="/password" required error={errors.password?.message}>
          <Input type={revealed ? 'text' : 'password'} autoComplete="current-password" {...form.register('password')} />
        </FormField>
        <div>
          <Button variant="ghost" aria-pressed={revealed} onClick={() => setRevealed((value) => !value)}>
            {intl.formatMessage(revealed ? messages.hidePassword : messages.showPassword)}
          </Button>
        </div>

        <div>
          <Button type="submit" loading={signIn.isPending}>
            {intl.formatMessage(messages.submit)}
          </Button>
        </div>
      </form>

      <div>
        <Link asChild>
          <RouterLink to="/settings/appearance">{intl.formatMessage(messages.languageLink)}</RouterLink>
        </Link>
      </div>
    </main>
  );
}

/** The reason a sign-in failed. It takes the focus when it appears, so a screen reader announces it (WCAG focus on error). */
function FailureAlert({ failure }: { failure: Failure }) {
  const intl = useIntl();
  const region = useRef<HTMLDivElement>(null);
  useEffect(() => {
    region.current?.focus();
  }, [failure]);

  let alert: ReactNode;
  if (failure.kind === 'invalid') {
    alert = <ProblemAlert problem={{ status: 401, title: intl.formatMessage(messages.invalid) }} />;
  } else if (failure.kind === 'throttled') {
    const minutes = Math.max(1, Math.ceil(failure.seconds / 60));
    alert = <ProblemAlert problem={{ status: 429, title: intl.formatMessage(messages.throttled, { minutes }) }} />;
  } else {
    alert = <ProblemAlert problem={failure.problem} />;
  }
  return (
    <div ref={region} tabIndex={-1} className="outline-none">
      {alert}
    </div>
  );
}
