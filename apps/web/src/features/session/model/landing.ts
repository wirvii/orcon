// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { safeRedirect } from './redirect';

/** Where a signed-in person goes first (SPEC-016 §3.7). */
export type Landing =
  /** A safe address the person came from, such as the page they opened before signing in. */
  | { kind: 'redirect'; href: string }
  /** The person belongs to no organization: the page explains it and offers to sign out. */
  | { kind: 'none' }
  /** Exactly one organization: its projects. */
  | { kind: 'organization'; slug: string }
  /** Several organizations: the picker. */
  | { kind: 'organizations' };

/**
 * Chooses the landing from the organizations of the person. A safe `redirect` wins; otherwise
 * zero organizations shows the explanation, one goes straight into it and several open the picker.
 */
export function landingFor(organizations: readonly { slug: string }[], redirect?: unknown): Landing {
  const href = safeRedirect(redirect);
  if (href !== undefined) return { kind: 'redirect', href };
  if (organizations.length === 0) return { kind: 'none' };
  const [only] = organizations;
  if (organizations.length === 1 && only) return { kind: 'organization', slug: only.slug };
  return { kind: 'organizations' };
}
