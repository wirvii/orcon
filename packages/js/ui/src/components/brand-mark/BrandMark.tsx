// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import appIcon from '../../assets/brand/orcon-app-icon.svg';
import lockupOnDark from '../../assets/brand/orcon-lockup-on-dark.svg';
import lockupOnLight from '../../assets/brand/orcon-lockup-on-light.svg';
import symbol from '../../assets/brand/orcon-symbol.svg';
import { cn } from '../../lib/cn';

/** The product name. It is a proper noun and is not translated. */
const PRODUCT_NAME = 'Orcon';

/** The approved file for each variant and theme, with the intrinsic size of its viewBox. */
const art = {
  lockup: {
    dark: { src: lockupOnDark, width: 657, height: 213 },
    light: { src: lockupOnLight, width: 499, height: 171 },
  },
  symbol: {
    dark: { src: symbol, width: 196, height: 213 },
    light: { src: appIcon, width: 182, height: 171 },
  },
} as const;

export interface BrandMarkProps {
  /** `lockup` shows the symbol and the wordmark; `symbol` only the symbol. */
  variant?: 'lockup' | 'symbol' | undefined;
  /**
   * Draw the art made for a dark or a light background whatever the theme is: for a surface that keeps its own
   * look in both themes, such as the brand panel of the sign-in page. Without it the theme chooses.
   */
  on?: 'dark' | 'light' | undefined;
  /** `large` is for a page of its own, such as the brand panel of the sign-in page. */
  size?: 'default' | 'large' | undefined;
  className?: string | undefined;
}

/**
 * The approved Orcon brand mark. The art for both themes is in the page and CSS
 * shows the one that fits the active theme (`.orcon-brand-on-dark` and
 * `.orcon-brand-on-light` in styles/index.css), so there is no flash and no
 * JavaScript reads the theme. The files are the approved assets of
 * brand/manifest.json, copied by `pnpm --filter @orcon/ui brand:sync`. Never
 * redraw, recolor or crop them.
 *
 * Dark theme: the symbol and the wordmark in light type. Light theme: the symbol
 * inside its dark rounded square and the wordmark in navy. The container carries
 * the accessible name; the images are decoration.
 */
export function BrandMark({ variant = 'lockup', on, size = 'default', className }: BrandMarkProps) {
  const { dark, light } = art[variant];
  return (
    <span className={cn('inline-flex shrink-0 items-center', className)} role="img" aria-label={PRODUCT_NAME} data-brand-variant={variant}>
      {on === 'light' ? null : <img {...dark} alt="" aria-hidden className={cn('w-auto', size === 'large' ? 'h-8' : 'h-icon-24', on === undefined && 'orcon-brand-on-dark')} />}
      {on === 'dark' ? null : <img {...light} alt="" aria-hidden className={cn('w-auto', size === 'large' ? 'h-8' : 'h-icon-24', on === undefined && 'orcon-brand-on-light')} />}
    </span>
  );
}
