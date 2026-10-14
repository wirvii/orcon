// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** A flow a Call sub-flow node can target. */
export interface FlowTarget {
  readonly id: string;
  readonly name: string;
  /** The flow has a published version; a test run calls that version (SPEC-014 R-ED-SUB-02). */
  readonly published: boolean;
}

/**
 * Where the editor reads the flows a node may call (SPEC-014 R-ED-SUB-01): the flows of the project
 * and the allowed targets in other projects of the organization. Connecting it to the server later
 * means one more implementation (the flow list of SPEC-005) chosen in `queries.ts`.
 */
export interface FlowTargetSource {
  list: () => Promise<readonly FlowTarget[]>;
}
