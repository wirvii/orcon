// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Checkbox, radio and switch rows. The whole row is the control: the label is
 * part of the button, so the hit area is as wide as the row and at least as
 * tall as the minimum target. The visible box or track sits inside it.
 */
export const choiceRow = [
  'group flex min-h-target w-full items-center gap-2 rounded-md px-1 text-left text-base text-fg',
  'transition-colors hover:bg-surface-sunken',
  'disabled:pointer-events-none disabled:text-fg-disabled disabled:hover:bg-transparent',
].join(' ');

/** The small visible box of a checkbox or radio. */
export const choiceBox = [
  'flex size-icon-16 shrink-0 items-center justify-center border border-control bg-surface text-action-primary-fg',
  'group-aria-invalid:border-status-danger group-disabled:border-border',
].join(' ');
