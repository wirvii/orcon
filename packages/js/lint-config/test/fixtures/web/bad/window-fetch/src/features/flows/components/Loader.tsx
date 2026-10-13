// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export async function load(): Promise<Response> {
  return window.fetch('/api/v1/flows');
}
