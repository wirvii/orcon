// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Brand comparison page. Reads brand/manifest.json, shows every asset beside
// its region of the approved primary board and measures the shape match
// (intersection over union) in a canvas. No dependencies.

const ROOT = '/';
const K = 4; // canvas scale used for the shape match
const BG = { 'brand-hue': '#111928', 'light-on-dark': '#111928', 'dark-on-light': '#f2f2f7', tile: '#f2f2f7' };
const DARK = '#0f172a';
const LIGHT = '#ffffff';

const $ = (sel, el = document) => el.querySelector(sel);
function h(tag, attrs = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'style') el.setAttribute('style', v);
    else el.setAttribute(k, v === true ? '' : String(v));
  }
  for (const kid of kids.flat()) {
    if (kid === null || kid === undefined || kid === false) continue;
    el.append(kid.nodeType ? kid : document.createTextNode(String(kid)));
  }
  return el;
}

const loadImage = (src) =>
  new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error(`cannot load ${src}`));
    img.src = src;
  });

// ---- masks (the same definitions the tracing used) --------------------------------------------
function brand(r, g, b) {
  const mx = Math.max(r, g, b);
  const mn = Math.min(r, g, b);
  const d = mx - mn;
  if (mx / 255 < 0.5 || mx === 0 || d / mx < 0.35) return false;
  let hue;
  if (mx === r) hue = ((g - b) / d) % 6;
  else if (mx === g) hue = (b - r) / d + 2;
  else hue = (r - g) / d + 4;
  hue = (hue * 60 + 360) % 360;
  return hue >= 60 && hue <= 200;
}
const luma = (r, g, b) => (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255;
const MASKS = {
  'brand-hue': (r, g, b) => brand(r, g, b),
  'light-on-dark': (r, g, b) => luma(r, g, b) >= 0.55 || brand(r, g, b),
  'dark-on-light': (r, g, b) => luma(r, g, b) <= 0.5,
  tile: (r, g, b) => luma(r, g, b) <= 0.6 || brand(r, g, b),
};

function canvas(w, h) {
  const c = document.createElement('canvas');
  c.width = w;
  c.height = h;
  return c;
}

/** IoU between the board region (masked) and the SVG alpha (>= 128) over the same region. */
function measureIoU(board, asset, fid, svgImg) {
  const [fx, fy, fw, fh] = fid.region;
  const [ax, ay, aw, ah] = asset.source.region;
  const W = Math.round(fw * K);
  const H = Math.round(fh * K);
  const a = canvas(W, H);
  const actx = a.getContext('2d', { willReadFrequently: true });
  actx.imageSmoothingEnabled = true;
  actx.imageSmoothingQuality = 'high';
  actx.drawImage(board, fx, fy, fw, fh, 0, 0, W, H);
  const b = canvas(W, H);
  const bctx = b.getContext('2d', { willReadFrequently: true });
  bctx.drawImage(svgImg, (ax - fx) * K, (ay - fy) * K, aw * K, ah * K);
  const da = actx.getImageData(0, 0, W, H).data;
  const db = bctx.getImageData(0, 0, W, H).data;
  const mask = MASKS[fid.mask];
  let inter = 0;
  let union = 0;
  for (let i = 0; i < da.length; i += 4) {
    const m = mask(da[i], da[i + 1], da[i + 2]);
    const s = db[i + 3] >= 128;
    if (m && s) inter++;
    if (m || s) union++;
  }
  return union === 0 ? 0 : inter / union;
}

// ---- contrast --------------------------------------------------------------------------------
function lin(c) {
  const v = c / 255;
  return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
}
function relLum(hex) {
  const n = parseInt(hex.slice(1), 16);
  return 0.2126 * lin((n >> 16) & 255) + 0.7152 * lin((n >> 8) & 255) + 0.0722 * lin(n & 255);
}
function ratio(a, b) {
  const [hi, lo] = [relLum(a), relLum(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}

// ---- UI pieces -------------------------------------------------------------------------------
const state = { zoom: 2, manifest: null, boardUrl: null };

function statusBadge(asset) {
  const a = asset.approval ?? { status: 'pending' };
  return h('span', { class: 'badge', 'data-status': a.status }, a.status + (a.date ? ` ${a.date}` : ''), a.note ? ` (${a.note})` : '');
}

function cropDiv(region, z, boardUrl, extra = '') {
  const [x, y, w, hh] = region;
  return h('div', {
    class: 'crop',
    style: `width:${w * z}px;height:${hh * z}px;background-image:url(${boardUrl});background-size:${1536 * z}px ${1024 * z}px;background-position:${-x * z}px ${-y * z}px;${extra}`,
  });
}

function shapeCard(asset, fid, boardUrl) {
  const [fx, fy, fw, fh] = fid.region;
  const z = Math.min(state.zoom, 700 / fw);
  const [ax, ay, aw, ah] = asset.source.region;
  const bg = BG[fid.mask];
  const card = h('article', { class: 'card', 'data-fidelity': fid.name, 'data-iou-min': fid.iou_min });
  const iouEl = h('span', { class: 'iou' }, 'measuring...');
  card.append(h('h3', {}, fid.name, h('span', { class: 'file' }, asset.path), statusBadge(asset), h('span', {}, 'shape match ', iouEl, ` (floor ${(fid.iou_min * 100).toFixed(0)}%)`)));
  const svgUrl = ROOT + asset.path;
  const svgAt = (zz, style = '') =>
    h('div', { style: `position:relative;width:${fw * zz}px;height:${fh * zz}px;overflow:hidden;background:${bg};${style}` },
      h('img', { src: svgUrl, alt: '', style: `position:absolute;left:${(ax - fx) * zz}px;top:${(ay - fy) * zz}px;width:${aw * zz}px;height:${ah * zz}px` }));
  const opacity = h('input', { type: 'range', min: 0, max: 100, value: 50 });
  const overlay = h('div', { class: 'stack', style: `width:${fw * z}px;height:${fh * z}px` },
    cropDiv(fid.region, z, boardUrl, 'position:absolute;inset:0;border:0'),
    h('div', { class: 'layer-svg', style: `opacity:.5;background:${bg}` }, svgAt(z).firstChild));
  const overlayImg = $('img', overlay);
  overlayImg.style.cssText = `position:absolute;left:${(ax - fx) * z}px;top:${(ay - fy) * z}px;width:${aw * z}px;height:${ah * z}px`;
  const layer = $('.layer-svg', overlay);
  opacity.addEventListener('input', () => { layer.style.opacity = opacity.value / 100; });
  const diff = h('div', { class: 'stack diff', style: `width:${fw * z}px;height:${fh * z}px;background:#000` },
    cropDiv(fid.region, z, boardUrl, 'position:absolute;inset:0;border:0'),
    h('div', { class: 'layer-svg', style: `background:${bg};position:absolute;inset:0` },
      h('img', { src: svgUrl, alt: '', style: `position:absolute;left:${(ax - fx) * z}px;top:${(ay - fy) * z}px;width:${aw * z}px;height:${ah * z}px` })));
  $('.layer-svg', diff).style.mixBlendMode = 'difference';
  card.append(h('div', { class: 'row pairs' },
    h('div', { class: 'cell' }, cropDiv(fid.region, z, boardUrl), h('small', {}, 'board region (original pixels)')),
    h('div', { class: 'cell' }, svgAt(z, 'border:1px solid #cbd5e1'), h('small', {}, 'vector (true paths)')),
    h('div', { class: 'cell' }, overlay, h('small', {}, 'overlay ', opacity)),
    h('div', { class: 'cell' }, diff, h('small', {}, 'difference (black = identical)'))));
  return { card, iouEl };
}

function useSizes(asset) {
  const url = ROOT + asset.path;
  const [, , aw, ah] = asset.source.region;
  const ratioWH = aw / ah;
  const isLockup = /lockup|wordmark/.test(asset.path);
  const heights = isLockup ? [24, 32, 48] : [16, 24, 32, 48, 160];
  const rows = [];
  for (const [name, cls] of [['light', 'surface-light'], ['dark', 'surface-dark']]) {
    rows.push(h('div', { class: `pad ${cls}`, 'data-surface': name }, heights.map((px) =>
      h('div', { class: 'cell' }, h('img', { src: url, alt: '', style: `height:${px}px;width:${(px * ratioWH).toFixed(1)}px` }), h('small', { style: `color:${name === 'dark' ? '#94a3b8' : '#64748b'}` }, `${px} px`)))));
  }
  return h('div', { class: 'row' }, rows);
}

function colorsOf(text) {
  return [...new Set((text.match(/#[0-9a-fA-F]{6}\b/g) ?? []).map((c) => c.toUpperCase()))];
}

// ---- main ------------------------------------------------------------------------------------
async function main() {
  const manifest = await (await fetch(`${ROOT}brand/manifest.json`)).json();
  state.manifest = manifest;
  const zoomSel = $('#zoom');
  state.zoom = Number(new URLSearchParams(location.search).get('zoom') ?? zoomSel.value);
  zoomSel.value = String(state.zoom);
  const boardUrl = ROOT + manifest.boards.primary.path;
  state.boardUrl = boardUrl;
  const board = await loadImage(boardUrl);

  const nav = $('#nav');
  for (const s of document.querySelectorAll('section')) {
    nav.append(h('a', { href: `#${s.id}` }, s.querySelector('h2').textContent));
  }

  // shape match
  const cards = $('#cards');
  cards.textContent = '';
  const results = [];
  for (const asset of manifest.assets.filter((a) => a.kind === 'svg' && a.fidelity?.length)) {
    const svgImg = await loadImage(ROOT + asset.path);
    const useCard = h('article', { class: 'card', 'data-uses': asset.path }, h('h3', {}, 'Sizes in use', h('span', { class: 'file' }, asset.path), statusBadge(asset)), useSizes(asset));
    for (const fid of asset.fidelity) {
      const { card, iouEl } = shapeCard(asset, fid, boardUrl);
      cards.append(card);
      const iou = measureIoU(board, asset, fid, svgImg);
      card.dataset.iou = iou.toFixed(4);
      iouEl.textContent = `${(iou * 100).toFixed(1)}%`;
      iouEl.dataset.pass = String(iou >= fid.iou_min);
      results.push({ name: fid.name, iou, min: fid.iou_min });
    }
    cards.append(useCard);
  }

  // icons
  const iconCards = $('#icon-cards');
  const icons = manifest.assets.filter((a) => a.path.startsWith('brand/icon/'));
  const tabSizes = icons.filter((a) => /favicon-(16|32|48)\.png$/.test(a.path));
  const tabRow = h('div', { class: 'row' });
  for (const [mode, cls] of [['light', 'tab-light'], ['dark', 'tab-dark']]) {
    const tabs = h('div', { class: 'cell' });
    for (const a of tabSizes) {
      const px = a.width;
      tabs.append(h('div', { class: `tab ${cls}` }, h('img', { src: ROOT + a.path, alt: '', width: px, height: px, class: 'pixelated' }), `Orcon (${px} px, 1x)`));
    }
    tabRow.append(h('div', { class: 'cell' }, h('small', {}, `${mode} browser tab`), tabs));
  }
  const zoomRow = h('div', { class: 'row' });
  for (const a of icons.filter((x) => x.kind === 'png')) {
    const big = a.width <= 48 ? a.width * 8 : Math.min(a.width, 192);
    zoomRow.append(h('div', { class: 'cell' },
      h('img', { src: ROOT + a.path, alt: '', width: big, height: big, class: a.width <= 48 ? 'pixelated checker' : 'checker' }),
      h('small', {}, `${a.path.split('/').pop()} ${a.width}x${a.height}${a.width <= 48 ? ' (8x, no smoothing)' : ''}`), statusBadge(a)));
  }
  for (const a of icons.filter((x) => x.kind === 'svg')) {
    zoomRow.append(h('div', { class: 'cell' }, h('img', { src: ROOT + a.path, alt: '', width: 128, height: 128, class: 'checker' }), h('small', {}, a.path.split('/').pop()), statusBadge(a)));
  }
  const icoAsset = icons.find((a) => a.kind === 'ico');
  let icoNote = '';
  if (icoAsset) {
    const buf = new DataView(await (await fetch(ROOT + icoAsset.path)).arrayBuffer());
    const n = buf.getUint16(4, true);
    const sizes = [];
    for (let i = 0; i < n; i++) sizes.push(buf.getUint8(6 + i * 16) || 256);
    icoNote = `favicon.ico entries: ${sizes.join(', ')} px`;
  }
  iconCards.append(h('article', { class: 'card', 'data-icons': 'true' }, h('h3', {}, 'Favicon set', h('span', { class: 'file' }, icoNote)), tabRow, zoomRow));

  // mascot
  const mascotCards = $('#mascot-cards');
  const mascots = manifest.assets.filter((a) => a.kind === 'mascot');
  if (mascots.length === 0) mascotCards.append(h('p', {}, 'No mascot cut-outs are listed in the manifest yet.'));
  for (const a of mascots) {
    const [x, y, w, hh] = a.source.region;
    const row = h('div', { class: 'row' },
      h('div', { class: 'cell' }, cropDiv(a.source.region, 1, boardUrl), h('small', {}, 'board region (1x)')),
      h('div', { class: 'cell' }, h('img', { src: ROOT + a.path, alt: '', width: w, height: hh, class: 'checker' }), h('small', {}, 'cut-out on checkerboard (1x)')),
      h('div', { class: 'cell' }, h('div', { class: 'surface-light pad' }, h('img', { src: ROOT + a.path, alt: '', style: 'height:160px' })), h('small', {}, 'light surface, 160 px')),
      h('div', { class: 'cell' }, h('div', { class: 'surface-dark pad' }, h('img', { src: ROOT + a.path, alt: '', style: 'height:160px' })), h('small', {}, 'dark surface, 160 px')),
      h('div', { class: 'cell' }, h('div', { class: 'surface-sunken pad' }, h('img', { src: ROOT + a.path, alt: '', style: 'height:160px' })), h('small', {}, 'sunken surface, 160 px')));
    mascotCards.append(h('article', { class: 'card', 'data-mascot': a.path }, h('h3', {}, a.path.split('/').pop(), h('span', { class: 'file' }, `${w}x${hh} px from board (${x}, ${y})`), statusBadge(a)), row));
  }

  if (mascots.length > 0) {
    // how the cut-outs will sit in the app: on a soft rounded box, light and dark theme (owner decision Q3 = A)
    const boxes = [['light theme', '#ecfdf5', '#0f172a'], ['dark theme', '#475569', '#e2e8f0']].map(([label, bg, fg]) =>
      h('div', { class: 'cell' },
        h('div', { style: `display:flex;gap:16px;padding:16px;border-radius:16px;background:${bg}` },
          mascots.map((a) => h('div', { style: `border-radius:16px;background:${bg};display:flex;align-items:flex-end` }, h('img', { src: ROOT + a.path, alt: '', style: 'height:160px;display:block' })))),
        h('small', { style: `color:${fg === '#e2e8f0' ? '#64748b' : '#64748b'}` }, `${label}: 160 px tall, soft rounded box`)));
    mascotCards.append(h('article', { class: 'card', 'data-composite': 'mascot-in-the-app' }, h('h3', {}, 'All four cut-outs as they will appear in the app', h('span', { class: 'file' }, 'preview colors are an assumption; the app uses design tokens')), h('div', { class: 'row' }, boxes)));
  }

  // contrast
  const rows = [];
  for (const asset of manifest.assets.filter((a) => a.kind === 'svg')) {
    const text = await (await fetch(ROOT + asset.path)).text();
    for (const c of colorsOf(text)) {
      rows.push(h('tr', {}, h('td', {}, asset.path.split('/').pop()), h('td', {}, h('span', { class: 'swatch', style: `background:${c}` }), c),
        h('td', {}, `${ratio(c, DARK).toFixed(2)}:1`), h('td', {}, `${ratio(c, LIGHT).toFixed(2)}:1`)));
    }
  }
  $('#contrast-table').append(h('table', {}, h('thead', {}, h('tr', {}, h('th', {}, 'File'), h('th', {}, 'Color in the file'), h('th', {}, `against dark surface ${DARK}`), h('th', {}, `against light surface ${LIGHT}`))), h('tbody', {}, rows)),
    h('p', {}, 'Logos are exempt from the WCAG contrast minimums; the figures state which surface each variant belongs on.'));

  document.body.dataset.results = JSON.stringify(results.map((r) => [r.name, Number(r.iou.toFixed(4)), r.min]));
  document.body.dataset.ready = 'true';
}

$('#zoom').addEventListener('change', (e) => {
  const q = new URLSearchParams(location.search);
  q.set('zoom', e.target.value);
  location.search = q.toString();
});
main().catch((err) => {
  document.body.prepend(h('pre', { class: 'note' }, String(err.stack ?? err)));
  document.body.dataset.ready = 'error';
});
