// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/**
 * An organization slug as it appears in an address: lowercase letters, digits and single hyphens,
 * up to 63 characters. It is a readable name, not an identifier (ids are UUIDs).
 */
export const orgSlugSchema = z.string().regex(/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/);

/** The search parameters of the sign-in page. A malformed `redirect` falls back to the default page. */
export const loginSearchSchema = z.object({
  redirect: z.string().optional().catch(undefined),
});

/** The landing page takes the same redirect: it is checked again by `landingFor` before it is used. */
export const landingSearchSchema = loginSearchSchema;
