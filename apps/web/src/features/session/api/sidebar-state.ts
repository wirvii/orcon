// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { readStored, writeStored } from '../../../app/boundary/storage';

const schema = z.boolean();
const keyOf = (userId: string) => `orcon.sidebar.collapsed.${userId}`;

/**
 * Whether the person chose the sidebar collapsed (true) or open (false). `undefined` means they never chose, so the
 * page decides: collapsed inside the flow editor, open everywhere else (SPEC-019 R-UX-SHL-04). It is kept in this
 * browser, per person.
 */
export function readSidebarCollapsed(userId: string): boolean | undefined {
  return readStored<boolean | undefined>(keyOf(userId), schema, undefined);
}

export function writeSidebarCollapsed(userId: string, collapsed: boolean): void {
  writeStored(keyOf(userId), collapsed);
}
