// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { parse, TYPE, type MessageFormatElement } from '@formatjs/icu-messageformat-parser';
import { describe, expect, it } from 'vitest';

// The catalogs of the design system and of the app. They are read as files: this checks the words, not the code.
import uiEn from '../../../../packages/js/ui/src/i18n/en.json';
import uiEs from '../../../../packages/js/ui/src/i18n/es.json';
import en from './en.json';
import es from './es.json';
import sameInBoth from './same-in-both.json';

type Catalog = Record<string, string>;

const catalogs: { name: string; en: Catalog; es: Catalog }[] = [
  { name: 'the design system', en: uiEn, es: uiEs },
  { name: 'the app', en, es },
];

/** The words of a message, without its placeholders and plural machinery. */
function literals(elements: MessageFormatElement[]): string {
  return elements
    .map((element) => {
      if (element.type === TYPE.literal) return element.value;
      if (element.type === TYPE.plural || element.type === TYPE.select) {
        return Object.values(element.options)
          .map((option) => literals(option.value))
          .join(' ');
      }
      return '';
    })
    .join(' ');
}

/** True when a message has no word in it, only placeholders, numbers and punctuation: it reads the same in any language. */
function hasNoWords(message: string): boolean {
  return !/\p{L}/u.test(literals(parse(message)));
}

/** The reasons a message may be the same in both languages. */
const reasons = new Set(['proper noun', 'acronym or symbol', 'unit', 'word spelled the same in both languages']);

describe('message catalogs', () => {
  it('catalogs have the same keys in en and es', () => {
    for (const catalog of catalogs) {
      expect(Object.keys(catalog.es).sort(), catalog.name).toEqual(Object.keys(catalog.en).sort());
    }
  });

  it('no es value equals its en value outside the allowlist', () => {
    const untranslated = catalogs.flatMap((catalog) =>
      Object.keys(catalog.en)
        .filter((key) => catalog.es[key] === catalog.en[key] && !(key in sameInBoth) && !hasNoWords(catalog.en[key] ?? ''))
        .map((key) => `${catalog.name}: ${key} = ${catalog.en[key]}`),
    );
    expect(untranslated).toEqual([]);
  });

  it('the allowlist only names proper nouns and acronyms', () => {
    // Every entry gives one of a closed set of reasons, names a message that exists and really is the same in both.
    const all = { ...uiEn, ...en };
    const allEs = { ...uiEs, ...es };
    for (const [key, reason] of Object.entries(sameInBoth)) {
      expect(reasons.has(reason), `${key}: "${reason}" is not a reason`).toBe(true);
      expect(all[key as keyof typeof all], `${key} is not a message`).toBeDefined();
      expect(allEs[key as keyof typeof allEs], `${key} differs in es, so it does not belong in the list`).toBe(all[key as keyof typeof all]);
    }
  });

  it('the sign-in tagline is Spanish in es', () => {
    expect(uiEs['brand.tagline']).toBe('Construye, orquesta y ejecuta a tu manera.');
    expect(uiEn['brand.tagline']).toBe('Build. Orchestrate. Execute. Your way.');
  });
});
