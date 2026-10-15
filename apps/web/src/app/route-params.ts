// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

/**
 * A readable name in an address: lowercase letters, digits and single hyphens, up to 63 characters.
 * Organization slugs and project handles are names, not ids (ids are UUIDs and stay out of the URL).
 */
const readableName = z.string().regex(/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/);

/** The organization of an address: `/orgs/$orgSlug/...`. */
export const orgSlugSchema = readableName;

/** The project of an address: `/orgs/$orgSlug/projects/$projectHandle/...`. */
export const projectHandleSchema = readableName;
