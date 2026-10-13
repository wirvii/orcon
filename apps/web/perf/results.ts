// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const file = join(dirname(fileURLToPath(import.meta.url)), '..', 'test-results', 'perf.json');

/** The median of a list of numbers (the mean of the two middle values for an even count). */
export function median(values: readonly number[]): number {
  if (values.length === 0) throw new Error('A median needs at least one value.');
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  const low = sorted[middle - 1] ?? 0;
  const high = sorted[middle] ?? 0;
  return sorted.length % 2 === 0 ? (low + high) / 2 : high;
}

interface PerfFile {
  sections: Record<string, unknown>;
}

function read(): PerfFile {
  try {
    const parsed: unknown = JSON.parse(readFileSync(file, 'utf8'));
    if (typeof parsed === 'object' && parsed !== null && 'sections' in parsed) return parsed as PerfFile;
  } catch {
    // No results yet, or an unreadable file: start again.
  }
  return { sections: {} };
}

/**
 * Adds one measurement to `test-results/perf.json`, the file CI keeps as an artifact. Every section
 * says how it was measured, so nobody mistakes an emulated number for a device measurement.
 */
export function recordResult(section: string, measurement: Record<string, unknown> & { method: string }): void {
  const current = read();
  current.sections[section] = measurement;
  mkdirSync(dirname(file), { recursive: true });
  writeFileSync(file, `${JSON.stringify({ updated: new Date().toISOString(), sections: current.sections }, null, 2)}\n`);
}
