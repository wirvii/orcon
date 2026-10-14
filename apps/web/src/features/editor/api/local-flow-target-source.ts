// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { EXAMPLE_FLOW_ID, SAMPLE_SUBFLOWS } from '../model/example-flow';

import type { FlowTarget, FlowTargetSource } from './flow-target-source';

/** The flows of the preview: the sample flow itself and the two small flows it can call. */
export function createLocalFlowTargetSource(): FlowTargetSource {
  const targets: readonly FlowTarget[] = [
    { id: EXAMPLE_FLOW_ID, name: 'Order notifications', published: true },
    ...SAMPLE_SUBFLOWS.map((flow, index): FlowTarget => ({ id: flow.id, name: flow.name, published: index === 0 })),
  ];
  return { list: () => Promise.resolve(targets) };
}
