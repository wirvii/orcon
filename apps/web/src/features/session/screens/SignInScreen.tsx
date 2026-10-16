// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { zodResolver } from '@hookform/resolvers/zod';
import { Banner, BrandMark, Button, ErrorSummary, FormField, IconButton, Input, Link, ProblemAlert, fieldIdForName, type ProblemDetails } from '@orcon/ui';
import { getRouteApi, Link as RouterLink, useNavigate } from '@tanstack/react-router';
import { Eye, EyeOff, Lock, Mail } from 'lucide-react';
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
import { ForgotPasswordPanel } from '../components/ForgotPasswordPanel';
import { SignInBrandPanel } from '../components/SignInBrandPanel';
import { SignInPreferences } from '../components/SignInPreferences';
import { safeRedirect } from '../model/redirect';

const messages = defineMessages({
  title: {
    id: 'session.signIn.title',
    defaultMessage: 'Sign in',
    description: 'Main heading of the sign-in page.',
  },
  description: {
    id: 'session.signIn.description',
    defaultMessage: 'Welcome back to Orcon.',
    description: 'One line under the heading of the sign-in page.',
  },
  email: { id: 'session.signIn.email', defaultMessage: 'Email', description: 'Label of the email field on the sign-in page.' },
  password: { id: 'session.signIn.password', defaultMessage: 'Password', description: 'Label of the password field on the sign-in page.' },
  forgot: { id: 'session.signIn.forgot', defaultMessage: 'Forgot it?', description: 'Link next to the password label that opens the password recovery panel.' },
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
  invalidHelp: {
    id: 'session.signIn.invalidHelp',
    defaultMessage: 'Check your details or recover your password.',
    description: 'Second line of the failed sign-in notice: what to do next.',
  },
  throttled: {
    id: 'session.signIn.throttled',
    defaultMessage: 'Too many attempts. Wait {minutes, plural, one {# minute} other {# minutes}} and try again.',
    description: 'Shown when the server stops sign-in attempts for an account for a while. The minutes come from the server.',
  },
  signedOut: {
    id: 'session.signIn.signedOut',
    defaultMessage: 'You signed out.',
    description: 'Notice on the sign-in page after the person chose to sign out.',
  },
  noAccount: {
    id: 'session.signIn.noAccount',
    defaultMessage: "Don't have an account? Ask whoever administers your organization for an invitation.",
    description: 'Text under the sign-in form: accounts are created by an administrator, so there is no sign-up.',
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
 * The sign-in page (SPEC-016 R-SH-UI-01, SPEC-019 R-UX-LGN). It is the only page a person without a session can open,
 * so it sets its own layout: the brand panel on the left from md up, the form on the right, language and theme at the
 * top right. A success goes to the landing, which picks the organization, and the page the person came from is kept
 * when it is a safe address. The recovery panel is a state of the same page (`?step=forgot-password`).
 */
export function SignInScreen() {
  const intl = useIntl();
  const navigate = useNavigate();
  const search = route.useSearch();
  const signIn = useSignIn();
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const [revealed, setRevealed] = useState(false);
  const redirect = safeRedirect(search.redirect);

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
    await navigate({ to: '/', search: redirect === undefined ? {} : { redirect } });
  });

  const summary = Object.entries(errors).flatMap(([name, error]) =>
    error.message ? [{ fieldId: fieldIdForName(`/${name}`), message: error.message }] : [],
  );

  return (
    <div className="bg-app flex min-h-dvh">
      <SignInBrandPanel />
      <main className="relative flex min-w-0 flex-1 flex-col items-center px-6 pt-16 pb-10 md:justify-center md:pt-10">
        <div className="absolute top-4 right-4 md:top-6 md:right-6">
          <SignInPreferences />
        </div>
        <div className="flex w-full max-w-form-width flex-col gap-5">
          <ConnectionBanners />
          {/* The brand panel is not shown on a phone, so the logo is. */}
          <BrandMark size="large" className="md:hidden" />

          {search.step === 'forgot-password' ? (
            <ForgotPasswordPanel
              initialEmail={form.getValues('email')}
              onBack={() => {
                void navigate({ to: '/login', search: redirect === undefined ? {} : { redirect } });
              }}
            />
          ) : (
            <>
              {search.signedOut === true && failure === undefined ? <Banner tone="success" title={intl.formatMessage(messages.signedOut)} /> : null}

              <div className="flex flex-col gap-1">
                <h1 className="text-display font-bold wrap-anywhere">{intl.formatMessage(messages.title)}</h1>
                <p className="text-fg-muted wrap-anywhere">{intl.formatMessage(messages.description)}</p>
              </div>

              {failure ? <FailureAlert failure={failure} /> : null}

              <form noValidate onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
                {submitCount > 0 ? <ErrorSummary errors={summary} focusOnMount /> : null}

                <FormField label={intl.formatMessage(messages.email)} name="/email" required error={errors.email?.message}>
                  <Input type="email" autoComplete="username" spellCheck={false} startIcon={Mail} {...form.register('email')} />
                </FormField>

                <FormField
                  label={intl.formatMessage(messages.password)}
                  name="/password"
                  required
                  error={errors.password?.message}
                  labelAction={
                    <Link asChild>
                      <RouterLink to="/login" search={{ step: 'forgot-password', ...(redirect === undefined ? {} : { redirect }) }}>
                        {intl.formatMessage(messages.forgot)}
                      </RouterLink>
                    </Link>
                  }
                >
                  <Input
                    type={revealed ? 'text' : 'password'}
                    autoComplete="current-password"
                    startIcon={Lock}
                    endAdornment={
                      <IconButton
                        variant="ghost"
                        icon={revealed ? EyeOff : Eye}
                        label={intl.formatMessage(revealed ? messages.hidePassword : messages.showPassword)}
                        aria-pressed={revealed}
                        onClick={() => {
                          setRevealed((value) => !value);
                        }}
                        className="size-8"
                      />
                    }
                    {...form.register('password')}
                  />
                </FormField>

                <Button type="submit" loading={signIn.isPending} className="h-10 w-full">
                  {intl.formatMessage(messages.submit)}
                </Button>
              </form>

              <p className="text-fg-muted text-center text-sm wrap-anywhere">{intl.formatMessage(messages.noAccount)}</p>
            </>
          )}
        </div>
      </main>
    </div>
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
    alert = (
      <Banner tone="danger" title={intl.formatMessage(messages.invalid)}>
        {intl.formatMessage(messages.invalidHelp)}
      </Banner>
    );
  } else if (failure.kind === 'throttled') {
    const minutes = Math.max(1, Math.ceil(failure.seconds / 60));
    alert = <Banner tone="danger" title={intl.formatMessage(messages.throttled, { minutes })} />;
  } else {
    alert = <ProblemAlert problem={failure.problem} />;
  }
  return (
    <div ref={region} tabIndex={-1} className="outline-none">
      {alert}
    </div>
  );
}
