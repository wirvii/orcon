// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test, type Page } from '@playwright/test';

import { median, recordResult } from './results';

// How these numbers were taken (orcon-web PF-4, maintainer decision of 2026-10-12): a headless Chromium
// on the CI runner does not reproduce a real GPU or display, so "60 fps" is measured as the frame
// intervals below and the result is an approximation.
const method = 'emulated-approximation';

const RUNS = 3;
const OPEN_BUDGET_MS = 1000;
/** A frame longer than this is a lost frame at 60 fps (16.7 ms) with some room. */
const LOST_FRAME_MS = 25;
const MAX_LOST_RATIO = 0.05;
const MAX_FRAME_MS = 50;
const GESTURE_MS = 3000;
/** Wheel units per step of the zoom gesture: with Control held, a trackpad pinch sends steps of this size. */
const ZOOM_WHEEL_DELTA = 40;

interface FrameRecord {
  running: boolean;
  intervals: number[];
}

declare global {
  interface Window {
    /** The frame recorder the page keeps while a gesture runs. */
    orconFrames?: FrameRecord;
  }
}

/** Starts recording the time between animation frames in the page. */
async function startFrames(page: Page): Promise<void> {
  await page.evaluate(() => {
    const record: FrameRecord = { running: true, intervals: [] };
    window.orconFrames = record;
    let last = performance.now();
    const tick = (now: number) => {
      if (!record.running) return;
      record.intervals.push(now - last);
      last = now;
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });
}

async function stopFrames(page: Page): Promise<number[]> {
  return page.evaluate(() => {
    const record = window.orconFrames;
    if (!record) throw new Error('No frame recording is running.');
    record.running = false;
    return record.intervals;
  });
}

/** Runs `step` about every frame for `durationMs`, recording the frames the page produced meanwhile. */
async function gesture(page: Page, durationMs: number, step: (progress: number) => Promise<void>): Promise<number[]> {
  await startFrames(page);
  const started = Date.now();
  let progress = 0;
  while (Date.now() - started < durationMs) {
    progress = (Date.now() - started) / durationMs;
    await step(progress);
    await page.waitForTimeout(8);
  }
  return stopFrames(page);
}

test('the editor with 500 nodes opens fast and stays smooth', async ({ browser }, testInfo) => {
  const harnessURL = String(testInfo.project.metadata['harnessURL'] ?? '');
  if (harnessURL === '') throw new Error('The project declares no harness URL.');

  const opens: number[] = [];
  const lostRatios: number[] = [];
  const maxFrames: number[] = [];

  for (let run = 0; run < RUNS; run += 1) {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'en-US', timezoneId: 'UTC' });
    const view = await context.newPage();
    await view.goto(harnessURL, { waitUntil: 'commit' });
    await view.waitForFunction(() => performance.getEntriesByName('orcon:editor-ready').length > 0, undefined, { timeout: 60_000 });
    opens.push(await view.evaluate(() => performance.getEntriesByName('orcon:editor-ready')[0]?.startTime ?? Number.NaN));

    // Let the first fit settle before the gestures start.
    await view.waitForTimeout(500);
    const canvas = view.locator('.react-flow__pane');
    const box = await canvas.boundingBox();
    if (!box) throw new Error('The canvas has no box.');
    const center = { x: box.x + box.width / 2, y: box.y + box.height / 2 };

    // Pan: the wheel scrolls the canvas (a plain drag on empty space draws a selection instead).
    await view.mouse.move(center.x, center.y);
    const pan = await gesture(view, GESTURE_MS, async (progress) => {
      await view.mouse.wheel(Math.sin(progress * Math.PI * 4) * 60, Math.cos(progress * Math.PI * 4) * 60);
    });

    // Zoom: Control and the wheel, in and out.
    await view.mouse.move(center.x, center.y);
    await view.keyboard.down('Control');
    const zoom = await gesture(view, GESTURE_MS, async (progress) => {
      await view.mouse.wheel(0, Math.sin(progress * Math.PI * 6) * ZOOM_WHEEL_DELTA);
    });
    await view.keyboard.up('Control');

    // Drag one node across the canvas.
    const node = view.locator('.react-flow__node').first();
    const nodeBox = await node.boundingBox();
    if (!nodeBox) throw new Error('There is no node on screen to drag.');
    const grab = { x: nodeBox.x + nodeBox.width / 2, y: nodeBox.y + nodeBox.height / 2 };
    await view.mouse.move(grab.x, grab.y);
    await view.mouse.down();
    const drag = await gesture(view, GESTURE_MS, async (progress) => {
      await view.mouse.move(grab.x + Math.sin(progress * Math.PI * 4) * 200, grab.y + Math.cos(progress * Math.PI * 4) * 100);
    });
    await view.mouse.up();

    const intervals = [...pan, ...zoom, ...drag];
    lostRatios.push(intervals.filter((interval) => interval > LOST_FRAME_MS).length / intervals.length);
    maxFrames.push(Math.max(...intervals));
    await context.close();
  }

  const result = { open: median(opens), lostFrameRatio: median(lostRatios), longestFrame: median(maxFrames) };
  recordResult('editor-500', {
    method,
    profile: 'desktop',
    nodes: 500,
    runs: RUNS,
    budgets: { openMs: OPEN_BUDGET_MS, lostFrameRatio: MAX_LOST_RATIO, longestFrameMs: MAX_FRAME_MS, lostFrameOverMs: LOST_FRAME_MS },
    samples: { openMs: opens, lostFrameRatio: lostRatios, longestFrameMs: maxFrames },
    median: result,
  });

  expect(result.open, 'time from navigation start to orcon:editor-ready (median)').toBeLessThan(OPEN_BUDGET_MS);
  expect(result.lostFrameRatio, `share of frames longer than ${String(LOST_FRAME_MS)} ms while panning, zooming and dragging (median)`).toBeLessThanOrEqual(MAX_LOST_RATIO);
  expect(result.longestFrame, 'longest frame while panning, zooming and dragging (median)').toBeLessThanOrEqual(MAX_FRAME_MS);
});
