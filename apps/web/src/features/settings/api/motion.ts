// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { readStored, writeStored } from '../../../app/boundary/storage';

/** The key the page reads before the first paint (`public/theme-init.js`). The value is the JSON `true`. */
export const MOTION_STORAGE_KEY = 'orcon.motion';

/** Puts the choice on <html>: the duration tokens become 0ms under `data-motion="reduce"`. */
export function applyReducedMotion(reduced: boolean, root: HTMLElement = document.documentElement): void {
  if (reduced) root.setAttribute('data-motion', 'reduce');
  else root.removeAttribute('data-motion');
}

/** Whether the person chose "Reduce animations" in this browser. */
export function readReducedMotion(): boolean {
  return readStored(MOTION_STORAGE_KEY, z.boolean(), false);
}

/** Keeps the choice in this browser and applies it at once. */
export function writeReducedMotion(reduced: boolean): void {
  writeStored(MOTION_STORAGE_KEY, reduced);
  applyReducedMotion(reduced);
}
