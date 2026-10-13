// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { userEvent } from 'storybook/test';

import type { Driver } from './env';
import { parityEnv } from './ParityStage';
import { parityScripts } from './scripts';

/** Synthetic input for Storybook. It cannot drag; the keyboard scripts do not need to. */
export const storybookDriver: Driver = {
  click: async (element) => void (await userEvent.click(element)),
  dblClick: async (element) => void (await userEvent.dblClick(element)),
  hover: async (element) => void (await userEvent.hover(element)),
  keyboard: async (keys) => void (await userEvent.keyboard(keys)),
  type: async (element, text) => void (await userEvent.type(element, text)),
};

/** The play function of a parity story: the keyboard script, then the end state check. */
export async function playParity(row: string, canvasElement: HTMLElement): Promise<void> {
  const script = parityScripts[row];
  if (!script) throw new Error(`No parity script for ${row}`);
  // Editing needs md or wider; on a phone the canvas is view only and there is nothing to play.
  if (!window.matchMedia('(min-width: 48rem)').matches) return;
  const env = parityEnv(storybookDriver, canvasElement, row);
  await script.keyboard(env);
  await script.verify(env);
  // The visual suite waits for this mark before it takes the end-state screenshot.
  canvasElement.setAttribute('data-parity', 'done');
}
