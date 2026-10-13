// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Flow } from '../model/flow';

export function FlowRow({ flow }: { flow: Flow }) {
  return <li>{flow.name}</li>;
}
