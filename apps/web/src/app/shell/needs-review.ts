// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * How many executions of the organization have an operation whose outcome is unknown and wait for a person to
 * review it. It is shown next to Executions in the sidebar.
 *
 * The contract has no operation that answers it yet: the count arrives with `listOrgExecutions` and its
 * `uncertain` filter (SPEC-019 UXB3). Until then this is `undefined`, which the sidebar draws as nothing, never as
 * zero: a number the product does not know is not shown.
 */
export function useNeedsReviewCount(): number | undefined {
  return undefined;
}
