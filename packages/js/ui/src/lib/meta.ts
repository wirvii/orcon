// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { SemanticTokenName } from '@orcon/tokens';

/** A foreground/background combination a component renders. It must be declared in @orcon/tokens src/pairs.json. */
export interface ColorPair {
  fg: SemanticTokenName;
  bg: SemanticTokenName;
}

/** Shorthand used by `<Name>.meta.ts` files. */
export const pair = (fg: SemanticTokenName, bg: SemanticTokenName): ColorPair => ({ fg, bg });
