// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Build entry: Style Dictionary v4 parses the DTCG sources (scripts/load.mjs)
// and the emitters (scripts/emit.mjs) write dist/. Run with
// `pnpm --filter @orcon/tokens build`.

import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

import { emitCss, emitDts, emitJs, emitJson, emitTailwindTheme } from './scripts/emit.mjs';
import { loadModel, packageDir } from './scripts/load.mjs';

const model = await loadModel();
const dist = join(packageDir, 'dist');
mkdirSync(dist, { recursive: true });

const outputs = {
  'tokens.css': emitCss(model),
  'tailwind-theme.css': emitTailwindTheme(model),
  'tokens.js': emitJs(model),
  'tokens.d.ts': emitDts(model),
  'tokens.json': emitJson(model),
};

for (const [name, text] of Object.entries(outputs)) {
  writeFileSync(join(dist, name), text);
}
console.log(`@orcon/tokens: wrote ${Object.keys(outputs).join(', ')} to dist/`);
