// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/** The search parameters of the sign-in page. A malformed `redirect` falls back to the default page. */
export const loginSearchSchema = z.object({
  redirect: z.string().optional().catch(undefined),
  /** Set by sign-out: the page says the person signed out. */
  signedOut: z.literal(true).optional().catch(undefined),
});

/** The landing page takes the same redirect: it is checked again by `landingFor` before it is used. */
export const landingSearchSchema = loginSearchSchema;
