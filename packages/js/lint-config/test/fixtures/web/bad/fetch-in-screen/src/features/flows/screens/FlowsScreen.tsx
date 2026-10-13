// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

export async function loadFlows(): Promise<Response> {
  return fetch('/api/v1/flows');
}
