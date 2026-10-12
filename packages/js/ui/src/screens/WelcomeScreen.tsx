// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { PlugZap, Play, Workflow, type LucideIcon } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { Button } from '../components/button/Button';
import { Link } from '../components/link/Link';
import { Mascot } from '../components/mascot/Mascot';

import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.welcome.title', defaultMessage: 'Welcome to Orcon', description: 'Main heading of the first screen a person sees.' },
  subtitle: {
    id: 'screen.welcome.subtitle',
    defaultMessage: 'Open-source orchestration engine',
    description: 'One line under the welcome heading that says what Orcon is.',
  },
  intro: {
    id: 'screen.welcome.intro',
    defaultMessage: 'Build a flow, connect your services and see every run, step by step.',
    description: 'Short paragraph on the welcome screen that says what a person can do with Orcon.',
  },
  create: { id: 'screen.welcome.create', defaultMessage: 'Create your first flow', description: 'Main button of the welcome screen.' },
  docs: { id: 'screen.welcome.docs', defaultMessage: 'Read the documentation', description: 'Link on the welcome screen to the documentation.' },
  stepsTitle: { id: 'screen.welcome.steps', defaultMessage: 'Getting started', description: 'Heading of the list of first steps on the welcome screen.' },
  stepCreateTitle: { id: 'screen.welcome.step.create.title', defaultMessage: 'Create a flow', description: 'First getting-started step: build a flow.' },
  stepCreateText: {
    id: 'screen.welcome.step.create.text',
    defaultMessage: 'Add a trigger and the steps that follow it.',
    description: 'Explains the first getting-started step.',
  },
  stepConnectTitle: { id: 'screen.welcome.step.connect.title', defaultMessage: 'Connect a service', description: 'Second getting-started step: connect an outside service.' },
  stepConnectText: {
    id: 'screen.welcome.step.connect.text',
    defaultMessage: 'Give the flow access to the tools it uses.',
    description: 'Explains the second getting-started step.',
  },
  stepRunTitle: { id: 'screen.welcome.step.run.title', defaultMessage: 'Run it and watch', description: 'Third getting-started step: run the flow and follow it.' },
  stepRunText: {
    id: 'screen.welcome.step.run.text',
    defaultMessage: 'Every run shows what each step did.',
    description: 'Explains the third getting-started step.',
  },
});

export interface WelcomeScreenProps {
  onCreateFlow?: () => void;
  /** Where the documentation link goes. */
  docsHref?: string;
}

/**
 * S1: the shell and the first-run welcome. The mascot placeholder is allowed here
 * (welcome and onboarding), and the page says what to do next.
 */
export function WelcomeScreen({ onCreateFlow, docsHref = '#docs' }: WelcomeScreenProps) {
  const intl = useIntl();
  const steps: { icon: LucideIcon; title: string; text: string }[] = [
    { icon: Workflow, title: intl.formatMessage(messages.stepCreateTitle), text: intl.formatMessage(messages.stepCreateText) },
    { icon: PlugZap, title: intl.formatMessage(messages.stepConnectTitle), text: intl.formatMessage(messages.stepConnectText) },
    { icon: Play, title: intl.formatMessage(messages.stepRunTitle), text: intl.formatMessage(messages.stepRunText) },
  ];
  return (
    <ProductShell current="flows" attentionCount={0}>
      <div className="mx-auto flex max-w-screen-md flex-col items-center gap-6 py-8 text-center">
        <Mascot pose="welcome" />
        <div className="flex flex-col gap-2">
          <h1 className="text-2xl font-semibold break-words">{intl.formatMessage(messages.title)}</h1>
          <p className="text-fg-muted text-md break-words">{intl.formatMessage(messages.subtitle)}</p>
          <p className="break-words">{intl.formatMessage(messages.intro)}</p>
        </div>
        <div className="flex flex-wrap items-center justify-center gap-3">
          <Button onClick={onCreateFlow}>{intl.formatMessage(messages.create)}</Button>
          <Link href={docsHref} className="inline-flex min-h-target items-center">
            {intl.formatMessage(messages.docs)}
          </Link>
        </div>
        <section aria-labelledby="welcome-steps" className="w-full text-left">
          <h2 id="welcome-steps" className="text-lg font-semibold break-words">
            {intl.formatMessage(messages.stepsTitle)}
          </h2>
          <ol className="mt-3 grid gap-3 md:grid-cols-3">
            {steps.map(({ icon: Icon, title, text }) => (
              <li key={title} className="bg-surface border-border flex flex-col gap-1 rounded-lg border p-4">
                <Icon aria-hidden className="text-fg-muted size-icon-24" />
                <p className="font-semibold break-words">{title}</p>
                <p className="text-fg-muted break-words">{text}</p>
              </li>
            ))}
          </ol>
        </section>
      </div>
    </ProductShell>
  );
}
