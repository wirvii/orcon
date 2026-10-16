// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * What a connector operation declares about repeating it (SPEC-003 operation contract, ADR-0011): whether it
 * changes something outside Orcon (`purity`) and how a repeat is made safe (`safetyLevel`). The editor shows it
 * in words: a safe repeat is "Never sends twice", an unsafe one is a warning.
 */
export type Purity = 'PURE' | 'READ' | 'EFFECT';
export type SafetyLevel = 'SAFE' | 'IDEMPOTENCY_KEY' | 'RECONCILE' | 'UNSAFE';

export interface OperationSafety {
  readonly purity: Purity;
  readonly safetyLevel: SafetyLevel;
  /**
   * The expression that gives the unique key of a send, relative to the node's input item, for steps that use one
   * (`message.wamid`). It is shown as a chip in the duplicate protection box.
   */
  readonly key?: string;
}

/**
 * True for a step that changes something outside and is protected against a repeat: its safety level is a unique key
 * the service honors, or a check with the service before sending again (SPEC-019 R-UX-EDT-03). Never for an unsafe
 * step: promising protection that does not exist would be worse than saying nothing.
 */
export function neverSendsTwice(safety: OperationSafety | undefined): boolean {
  return safety?.purity === 'EFFECT' && (safety.safetyLevel === 'IDEMPOTENCY_KEY' || safety.safetyLevel === 'RECONCILE');
}

/** True for a step that changes something outside and cannot be repeated safely: a retry may send twice. */
export function mayRepeat(safety: OperationSafety | undefined): boolean {
  return safety?.purity === 'EFFECT' && safety.safetyLevel === 'UNSAFE';
}
