// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

export const flowSchema = z.object({ id: z.string(), name: z.string() });
export type Flow = z.infer<typeof flowSchema>;

export function sortFlows(flows: readonly Flow[]): Flow[] {
  return [...flows].sort((a, b) => a.name.localeCompare(b.name));
}
