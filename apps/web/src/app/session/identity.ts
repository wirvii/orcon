// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** The name the server stores for an account created without one (`orcon accounts create`). It means "no name". */
const PLACEHOLDER_NAMES = new Set(['owner']);

export interface Identity {
  /** What the person is called in the menu: their name, or a readable form of the part of the email before the `@`. */
  name: string;
  /** Up to two letters for the avatar. */
  initials: string;
}

function capitalize(word: string): string {
  return word.charAt(0).toUpperCase() + word.slice(1);
}

/**
 * Who to show for the signed-in person. The account name wins; an empty name or the server's placeholder
 * ("Owner") counts as unset, and the name is then derived from the email: `ana.ruiz@x` is "Ana Ruiz" (AR),
 * `jftamayop@x` is "jftamayop" (JF).
 */
export function identityOf(user: { displayName: string; email: string }): Identity {
  const name = user.displayName.trim();
  if (name !== '' && !PLACEHOLDER_NAMES.has(name.toLowerCase())) {
    return { name, initials: initialsOf(name.split(/\s+/)) };
  }
  const local = (user.email.split('@')[0] ?? '').trim();
  const parts = local.split(/[._+-]+/).filter(Boolean);
  if (parts.length === 0) return { name: user.email, initials: initialsOf([user.email]) };
  return { name: parts.length > 1 ? parts.map(capitalize).join(' ') : local, initials: initialsOf(parts) };
}

function initialsOf(words: readonly string[]): string {
  const [first = '', second] = words;
  const letters = second === undefined ? first.slice(0, 2) : first.charAt(0) + second.charAt(0);
  return letters.toUpperCase();
}
