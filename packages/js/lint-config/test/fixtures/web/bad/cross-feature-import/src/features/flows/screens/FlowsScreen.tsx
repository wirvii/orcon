// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Execution } from '../../executions/model/execution';

export function FlowsScreen({ execution }: { execution: Execution }) {
  return <p>{execution.id}</p>;
}
