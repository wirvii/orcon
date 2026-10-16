// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod';

import { parseJsonText, tryParse } from './parse';

/** The path a dragged field carries: names and list positions, at least one and not unreasonably many. */
const fieldPathSchema = z.array(z.union([z.string().min(1).max(256), z.number().int().min(0)])).min(1).max(32);

export type DraggedPath = z.infer<typeof fieldPathSchema>;

/** Whether a drag carries a field of the input data. Checked while it moves, when the data itself cannot be read yet. */
export function carriesField(transfer: Pick<DataTransfer, 'types'>, type: string): boolean {
  return Array.from(transfer.types).includes(type);
}

/**
 * The path of the field a drag carries, or `undefined` for a drag that is not one (a file, text from another page, a
 * path that is not a path). What comes from a drag is external input: it is parsed here, never trusted.
 */
export function readDraggedField(transfer: Pick<DataTransfer, 'getData'>, type: string): DraggedPath | undefined {
  const text = transfer.getData(type);
  if (text === '') return undefined;
  const json = parseJsonText(text);
  if (!json.ok) return undefined;
  const parsed = tryParse(fieldPathSchema, json.value, 'dragged field');
  return parsed.ok ? parsed.value : undefined;
}
