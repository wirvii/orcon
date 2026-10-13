// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export function parseFlow(raw: string): unknown {
  return JSON.parse(raw);
}
