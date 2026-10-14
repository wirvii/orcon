// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import descriptorOnDark from '../../assets/brand/orcon-lockup-descriptor-on-dark.svg';
import descriptorOnLight from '../../assets/brand/orcon-lockup-descriptor-on-light.svg';
import { cn } from '../../lib/cn';

/** The product name. It is a proper noun and is not translated. */
const PRODUCT_NAME = 'Orcon';

const messages = defineMessages({
  tagline: {
    id: 'brand.tagline',
    defaultMessage: 'Build. Orchestrate. Execute. Your way.',
    description: 'Brand tagline under the Orcon wordmark. Keep it in English: it is the brand line.',
  },
});

export interface BrandHeroProps {
  className?: string | undefined;
}

/**
 * The sign-in brand block: the approved lockup with the descriptor, in the file
 * that fits the active theme (chosen by CSS, like BrandMark), and the tagline as
 * real text under it. The lockup is 320 px wide, the available width on a phone and
 * never under 280 px. The container carries the accessible name; the images are
 * decoration.
 */
export function BrandHero({ className }: BrandHeroProps) {
  const intl = useIntl();
  return (
    <div className={cn('flex flex-col items-center gap-3 text-center', className)}>
      <span role="img" aria-label={PRODUCT_NAME} className="flex justify-center">
        <img src={descriptorOnDark} width={657} height={213} alt="" aria-hidden className="orcon-brand-on-dark orcon-brand-hero-image" />
        <img src={descriptorOnLight} width={567} height={171} alt="" aria-hidden className="orcon-brand-on-light orcon-brand-hero-image" />
      </span>
      <p className="text-fg-muted text-md wrap-anywhere">{intl.formatMessage(messages.tagline)}</p>
    </div>
  );
}
