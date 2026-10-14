// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Renders the PNG and ICO brand exports from the approved SVG files with the
// repository's Playwright Chromium (pnpm --filter @orcon/ui brand:render).
// Every output is declared with its exact size in brand/manifest.json.

import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { chromium } from '@playwright/test';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const NAVY = '#0F172A';

/** Width and height in px of a lockup SVG, read from its viewBox. */
function viewBoxSize(file) {
  const text = readFileSync(path.join(root, file), 'utf8');
  const m = /viewBox="0 0 ([\d.]+) ([\d.]+)"/.exec(text);
  if (!m) throw new Error(`${file}: no viewBox`);
  return [Number(m[1]), Number(m[2])];
}

const jobs = [];
const add = (out, w, h, svg, html) => jobs.push({ out, w, h, svg, html });

// Favicon and app icons (square tile, transparent outside the rounded tile).
add('brand/icon/favicon-16.png', 16, 16, 'brand/icon/favicon-16.svg');
add('brand/icon/favicon-32.png', 32, 32, 'brand/icon/favicon.svg');
add('brand/icon/favicon-48.png', 48, 48, 'brand/icon/favicon.svg');
add('brand/icon/icon-192.png', 192, 192, 'brand/icon/favicon.svg');
add('brand/icon/icon-512.png', 512, 512, 'brand/icon/favicon.svg');

// Apple touch icon: full-bleed square, the system rounds the corners.
add('brand/icon/apple-touch-icon.png', 180, 180, null, () => {
  const sym = pathToFileURL(path.join(root, 'brand/logo/orcon-symbol.svg')).href;
  return `<body style="margin:0;background:${NAVY};width:180px;height:180px;display:flex;align-items:center;justify-content:center"><img src="${sym}" style="height:104px;display:block"></body>`;
});

// Symbol on a transparent square canvas.
for (const size of [256, 512, 1024]) {
  add(`brand/png/orcon-symbol-${size}.png`, size, size, null, () => {
    const sym = pathToFileURL(path.join(root, 'brand/logo/orcon-symbol.svg')).href;
    return `<body style="margin:0;background:transparent;width:${size}px;height:${size}px;display:flex;align-items:center;justify-content:center"><img src="${sym}" style="height:${Math.round(size * 0.9)}px;display:block"></body>`;
  });
}

// Lockups at 2x of their viewBox.
for (const [file, out] of [
  ['brand/logo/orcon-lockup-on-dark.svg', 'brand/png/orcon-lockup-on-dark@2x.png'],
  ['brand/logo/orcon-lockup-on-light.svg', 'brand/png/orcon-lockup-on-light@2x.png'],
  ['brand/logo/orcon-lockup-descriptor-on-dark.svg', 'brand/png/orcon-lockup-descriptor-on-dark@2x.png'],
  ['brand/logo/orcon-lockup-descriptor-on-light.svg', 'brand/png/orcon-lockup-descriptor-on-light@2x.png'],
]) {
  const [w, h] = viewBoxSize(file);
  add(out, w * 2, h * 2, file);
}

/** ICO with PNG entries (Vista and later), written without dependencies. */
function ico(entries) {
  const head = Buffer.alloc(6);
  head.writeUInt16LE(0, 0);
  head.writeUInt16LE(1, 2);
  head.writeUInt16LE(entries.length, 4);
  const dir = Buffer.alloc(16 * entries.length);
  let offset = 6 + dir.length;
  entries.forEach((e, i) => {
    const o = i * 16;
    dir[o] = e.size >= 256 ? 0 : e.size;
    dir[o + 1] = e.size >= 256 ? 0 : e.size;
    dir.writeUInt16LE(1, o + 4);
    dir.writeUInt16LE(32, o + 6);
    dir.writeUInt32LE(e.data.length, o + 8);
    dir.writeUInt32LE(offset, o + 12);
    offset += e.data.length;
  });
  return Buffer.concat([head, dir, ...entries.map((e) => e.data)]);
}

const tmp = mkdtempSync(path.join(tmpdir(), 'orcon-brand-'));
const browser = await chromium.launch();
try {
  let n = 0;
  for (const job of jobs) {
    const page = await browser.newPage({ viewport: { width: job.w, height: job.h }, deviceScaleFactor: 1 });
    let body;
    if (job.html) {
      body = job.html();
    } else {
      const url = pathToFileURL(path.join(root, job.svg)).href;
      body = `<body style="margin:0;background:transparent"><img src="${url}" style="width:${job.w}px;height:${job.h}px;display:block"></body>`;
    }
    const wrapper = path.join(tmp, `w${n++}.html`);
    writeFileSync(wrapper, `<!doctype html><html>${body}</html>`);
    await page.goto(pathToFileURL(wrapper).href);
    await page.evaluate(() => Promise.all([...document.images].map((i) => i.decode())));
    const png = await page.screenshot({ omitBackground: true, clip: { x: 0, y: 0, width: job.w, height: job.h } });
    mkdirSync(path.dirname(path.join(root, job.out)), { recursive: true });
    writeFileSync(path.join(root, job.out), png);
    console.log(`${job.out} ${job.w}x${job.h}`);
    await page.close();
  }
  const read = (f) => readFileSync(path.join(root, f));
  writeFileSync(
    path.join(root, 'brand/icon/favicon.ico'),
    ico([
      { size: 16, data: read('brand/icon/favicon-16.png') },
      { size: 32, data: read('brand/icon/favicon-32.png') },
      { size: 48, data: read('brand/icon/favicon-48.png') },
    ]),
  );
  console.log('brand/icon/favicon.ico 16,32,48');
} finally {
  await browser.close();
  rmSync(tmp, { recursive: true, force: true });
}
