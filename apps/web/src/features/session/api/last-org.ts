// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { readStored, writeStored } from '../../../app/boundary/storage';

const KEY = 'orcon.lastOrgSlug';
const schema = z.string().min(1).max(200);

/** Remembers the organization the person was working in, so pages outside an organization (settings) can still link back to it. */
export function rememberOrganization(slug: string): void {
  writeStored(KEY, slug);
}

export function rememberedOrganization(): string | undefined {
  return readStored<string | undefined>(KEY, schema, undefined);
}
