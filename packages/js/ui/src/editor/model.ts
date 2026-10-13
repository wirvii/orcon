// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/** Why a value is hidden: no permission to read execution data, a secret, or masked in the stored history. */
export type ProtectedKind = 'permission' | 'secret' | 'history';

/** A value the person may not see. It never carries the real value: the data layer replaces it before it reaches the page. */
export interface ProtectedValue {
  readonly $protected: ProtectedKind;
}

export const protectedValue = (kind: ProtectedKind = 'permission'): ProtectedValue => ({ $protected: kind });

export function isProtected(value: unknown): value is ProtectedValue {
  return typeof value === 'object' && value !== null && '$protected' in value && typeof (value as ProtectedValue).$protected === 'string';
}

/** A file produced by a node: shown by name, media type and size, with a download link. */
export interface BinaryValue {
  readonly $binary: { name: string; mediaType: string; size: number; href?: string };
}

export function isBinary(value: unknown): value is BinaryValue {
  return typeof value === 'object' && value !== null && '$binary' in value && typeof (value as BinaryValue).$binary === 'object';
}

/** One item of data: an object with named fields. */
export type DataItem = Record<string, unknown>;

export interface DataRun {
  /** 1-based index of the execution shown. */
  index: number;
  total: number;
}

/** The data a pane shows. */
export interface PaneData {
  items: readonly DataItem[];
  /** The node the data comes from, for "from HTTP Request". */
  from?: string | undefined;
  run?: DataRun | undefined;
  /** The output is pinned: later runs reuse it. */
  pinned?: boolean | undefined;
}

export type DataView = 'table' | 'json' | 'schema';

/** A JSON-like value of an item, with protected values and files recognised. */
export type JsonLike = unknown;

/** The text a leaf value shows: strings, numbers, booleans, bigints and symbols print as themselves (as `String` does), a function as nothing. */
export function plainText(value: unknown): string {
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean' || typeof value === 'bigint' || typeof value === 'symbol') return value.toString();
  return typeof value === 'function' ? '' : JSON.stringify(value);
}

/** The text of a value for the clipboard. A protected value is never copied: it becomes a fixed notice. */
export function visibleValue(value: unknown, hidden = '[hidden]'): unknown {
  if (isProtected(value)) return hidden;
  if (isBinary(value)) return { name: value.$binary.name, mediaType: value.$binary.mediaType, size: value.$binary.size };
  if (Array.isArray(value)) return value.map((entry) => visibleValue(entry, hidden));
  if (typeof value === 'object' && value !== null) {
    return Object.fromEntries(Object.entries(value).map(([key, entry]) => [key, visibleValue(entry, hidden)]));
  }
  return value;
}

export type SchemaType = 'string' | 'number' | 'boolean' | 'null' | 'object' | 'array' | 'file' | 'hidden';

export interface SchemaField {
  /** Field name, or the index for array entries. */
  key: string;
  type: SchemaType;
  /** A short example value, at most 60 characters. Absent for hidden values. */
  example?: string | undefined;
  /** The value is protected: the example is masked. */
  sensitive?: boolean | undefined;
  children?: SchemaField[] | undefined;
}

export const EXAMPLE_LIMIT = 60;

export const typeOf = (value: unknown): SchemaType => {
  if (isProtected(value)) return 'hidden';
  if (isBinary(value)) return 'file';
  if (value === null || value === undefined) return 'null';
  if (Array.isArray(value)) return 'array';
  switch (typeof value) {
    case 'string':
      return 'string';
    case 'number':
    case 'bigint':
      return 'number';
    case 'boolean':
      return 'boolean';
    default:
      return 'object';
  }
};

const clip = (text: string) => (text.length > EXAMPLE_LIMIT ? `${text.slice(0, EXAMPLE_LIMIT - 1)}…` : text);

function describe(key: string, value: unknown, depth: number): SchemaField {
  const type = typeOf(value);
  if (type === 'hidden') return { key, type, sensitive: true };
  if (type === 'object' && depth < 4) {
    return { key, type, children: Object.entries(value as Record<string, unknown>).map(([name, entry]) => describe(name, entry, depth + 1)) };
  }
  if (type === 'array' && depth < 4) {
    const entries = (value as unknown[]).slice(0, 1);
    return { key, type, example: clip(`[${(value as unknown[]).length}]`), children: entries.map((entry, index) => describe(String(index), entry, depth + 1)) };
  }
  if (type === 'file') return { key, type, example: clip((value as BinaryValue).$binary.name) };
  if (type === 'null') return { key, type, example: 'null' };
  return { key, type, example: clip(typeof value === 'string' ? value : JSON.stringify(value)) };
}

/** The fields of the first item, with a type and a short example each. Fields that are protected are marked sensitive. */
export function inferSchema(items: readonly DataItem[]): SchemaField[] {
  const first = items[0];
  if (!first) return [];
  return Object.entries(first).map(([key, value]) => describe(key, value, 0));
}

/** The columns of a table: every top-level field name, in the order they first appear. */
export function columnsOf(items: readonly DataItem[]): string[] {
  const seen = new Set<string>();
  for (const item of items) for (const key of Object.keys(item)) seen.add(key);
  return [...seen];
}
