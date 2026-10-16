// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { apiCall, apiRequest } from '../../../app/api/http';
import { ContractError } from '../../../app/errors/contract-error';
import { currentUserSchema, sessionSchema, type CurrentUser, type Session } from '../../../app/session/current-user';

const sessionPageSchema = z.object({
  items: z.array(sessionSchema),
  nextPageToken: z.string().optional(),
});

/** The most pages the list follows: 20 pages of 100 is far more live sessions than a person has. */
const MAX_PAGES = 20;

/** The person with the entity tag a change must send in `If-Match`. */
export interface UserWithTag {
  user: CurrentUser;
  etag: string;
}

/** The live sessions of the signed-in person, newest first (`listSessions`). */
export async function listSessions(): Promise<readonly Session[]> {
  const items: Session[] = [];
  let pageToken: string | undefined;
  for (let page = 0; page < MAX_PAGES; page += 1) {
    const answer = await apiRequest({
      method: 'GET',
      path: '/users/me/sessions',
      query: { limit: 100, pageToken },
      schema: sessionPageSchema,
      source: 'response of GET /users/me/sessions',
    });
    items.push(...answer.items);
    pageToken = answer.nextPageToken;
    if (pageToken === undefined) break;
  }
  return items;
}

/** The signed-in person with the entity tag of this representation (`getCurrentUser`). */
export async function getUserWithTag(): Promise<UserWithTag> {
  const source = 'response of GET /users/me';
  const { value, etag } = await apiCall({ method: 'GET', path: '/users/me', schema: currentUserSchema, source });
  if (etag === undefined) throw new ContractError(source, []);
  return { user: value, etag };
}

/** Changes the display name (`updateCurrentUser`, JSON Merge Patch). A stale tag is refused with 412. */
export async function updateDisplayName(displayName: string, etag: string): Promise<UserWithTag> {
  const source = 'response of PATCH /users/me';
  const answer = await apiCall({ method: 'PATCH', path: '/users/me', body: { displayName }, ifMatch: etag, schema: currentUserSchema, source });
  if (answer.etag === undefined) throw new ContractError(source, []);
  return { user: answer.value, etag: answer.etag };
}
