// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { BrandMark, Link, Mascot } from '@orcon/ui';
import { Code, ShieldCheck, Upload, type LucideIcon } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

const messages = defineMessages({
  headline: {
    id: 'session.signIn.brand.headline',
    defaultMessage: 'Build, orchestrate and execute <accent>your way.</accent>',
    description: 'Headline of the brand panel of the sign-in page. The part in the tag is shown in the accent color.',
  },
  fromN8n: {
    id: 'session.signIn.brand.fromN8n',
    defaultMessage: 'Bring your n8n flows: we import them and tell you what changed.',
    description: 'First reason to use the product, on the brand panel of the sign-in page.',
  },
  neverTwice: {
    id: 'session.signIn.brand.neverTwice',
    defaultMessage: 'A step that sends something out never sends it twice.',
    description: 'Second reason to use the product, on the brand panel of the sign-in page.',
  },
  openSource: {
    id: 'session.signIn.brand.openSource',
    defaultMessage: 'Open source (Apache-2.0). Your data, on your server.',
    description: 'Third reason to use the product, on the brand panel of the sign-in page.',
  },
  documentation: {
    id: 'session.signIn.brand.documentation',
    defaultMessage: 'Documentation',
    description: 'Link at the bottom of the brand panel of the sign-in page that opens the documentation.',
  },
});

const DOCS = 'https://github.com/wirvii/orcon/tree/main/docs/user/en';

const reasons: { icon: LucideIcon; message: (typeof messages)['fromN8n'] }[] = [
  { icon: Upload, message: messages.fromN8n },
  { icon: ShieldCheck, message: messages.neverTwice },
  { icon: Code, message: messages.openSource },
];

/**
 * The brand side of the sign-in page (SPEC-019 R-UX-LGN-01): the logo, the headline, three reasons and the
 * crossed-arms mascot. It keeps its own dark look in both themes, which `data-theme="dark"` gives it by re-scoping
 * the tokens below it. It is decoration and explanation, never a control, and a phone does not show it.
 */
export function SignInBrandPanel() {
  const intl = useIntl();
  return (
    <aside data-theme="dark" className="bg-surface text-fg bg-brand-glow relative hidden w-11/24 flex-col overflow-hidden p-12 md:flex">
      <div aria-hidden className="bg-dot-grid absolute inset-0" />
      <div className="relative flex min-h-full flex-1 flex-col">
        <BrandMark on="dark" size="large" className="self-start" />
        <h2 className="text-2xl mt-auto w-3/4 font-bold text-balance">
          {intl.formatMessage(messages.headline, { accent: (chunks) => <span className="text-fg-accent">{chunks}</span> })}
        </h2>
        <ul className="mt-6 flex w-3/5 flex-col gap-3">
          {reasons.map(({ icon: Icon, message }) => (
            <li key={message.id} className="text-fg-muted flex gap-3">
              <Icon aria-hidden className="text-fg-accent mt-1 size-icon-16 shrink-0" />
              <span>{intl.formatMessage(message)}</span>
            </li>
          ))}
        </ul>
        <div className="text-fg-subtle mt-10 text-xs">
          <Link href={DOCS} external className="text-fg-subtle">
            {intl.formatMessage(messages.documentation)}
          </Link>
        </div>
      </div>
      {/* On the edge of the panel, as in the design, so it does not push the text. Only where there is room. */}
      <Mascot pose="welcome" bare className="absolute right-4 bottom-0 hidden lg:inline-flex" />
    </aside>
  );
}
