// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createContext, useContext } from 'react';

import { scrim } from './control';

/** True inside the panel of a modal dialog: a dialog opened from there is a nested one. */
export const InsideModal = createContext(false);

/** Backdrop of a dialog opened from inside another one. Same layer as the modal panels, drawn after the parent. */
const nestedScrim = 'fixed inset-0 z-modal bg-scrim opacity-60';

/**
 * The backdrop class of a modal. A dialog opened from inside another dialog sits at the modal layer
 * instead of the overlay layer: the overlay layer is under the parent panel, so the parent would not
 * dim. Portaled later, it is drawn over the parent at the same layer, and its panel over its backdrop.
 */
export function useScrim(): string {
  return useContext(InsideModal) ? nestedScrim : scrim;
}
