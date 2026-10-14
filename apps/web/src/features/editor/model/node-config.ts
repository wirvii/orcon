// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { ParamDef, ParamType } from './node-manifests';

/**
 * The rules of a node's parameter form that do not depend on React: which parameters show, what a
 * value is (fixed or a formula), what is wrong with it. A string value that starts with `=` is an
 * expression (ADR-0018 §1); everything else is a fixed value.
 */

export type Parameters = Readonly<Record<string, unknown>>;
export type ParamMode = 'fixed' | 'expression';

export const isExpression = (value: unknown): value is string => typeof value === 'string' && value.startsWith('=');

/** The formula of an expression value, without the leading `=`. */
export const expressionSource = (value: unknown): string => (isExpression(value) ? value.slice(1) : '');

export const toExpression = (source: string): string => `=${source}`;

const FORMULA_TYPES: ReadonlySet<ParamType> = new Set(['STRING', 'NUMBER', 'OPTIONS', 'DATE_TIME', 'JSON', 'BINARY_PROPERTY']);

/** Every text-like parameter can be a fixed value or an expression (SPEC-014 R-ED-CFG-02). */
export const expressionCapable = (def: ParamDef): boolean => FORMULA_TYPES.has(def.type) && def.widget === undefined;

/** The value a parameter has now: what the node holds, or the default when it holds nothing. */
export function valueOf(def: ParamDef, parameters: Parameters): unknown {
  return parameters[def.name] ?? def.default;
}

export function modeOf(def: ParamDef, parameters: Parameters): ParamMode {
  const value = parameters[def.name];
  if (isExpression(value)) return 'expression';
  const empty = value === undefined || value === '';
  return empty && def.expression === 'default' ? 'expression' : 'fixed';
}

const text = (value: unknown): string => (typeof value === 'string' ? value : typeof value === 'number' || typeof value === 'boolean' ? String(value) : '');

/** The value after switching a parameter between fixed and expression: the text carries over. */
export function switchMode(def: ParamDef, parameters: Parameters, to: ParamMode): unknown {
  const value = parameters[def.name];
  if (to === 'expression') return toExpression(isExpression(value) ? expressionSource(value) : text(valueOf(def, parameters)));
  const source = expressionSource(value);
  if (def.type === 'NUMBER') return source.trim() !== '' && Number.isFinite(Number(source)) ? Number(source) : undefined;
  return source === '' ? undefined : source;
}

/** True when the parameter shows for the values the node has now (`displayWhen`, all conditions). */
export function isVisible(def: ParamDef, parameters: Parameters): boolean {
  return (def.displayWhen ?? []).every((condition) => {
    const value = parameters[condition.param];
    return typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean' ? condition.oneOf.includes(value) : false;
  });
}

export type ProblemCode = 'required' | 'number' | 'json' | 'option' | 'incomplete';

const empty = (value: unknown): boolean => value === undefined || value === null || value === '' || (isExpression(value) && value.trim() === '=') || (Array.isArray(value) && value.length === 0);

/** What is wrong with the value of one parameter, if anything. Formulas are checked by the server and the expression editor, not here. */
export function problemOf(def: ParamDef, parameters: Parameters, isJson: (text: string) => boolean = () => true): ProblemCode | undefined {
  const value = valueOf(def, parameters);
  if (def.type === 'BOOLEAN') return undefined;
  if (def.type === 'FIXED_COLLECTION' || def.type === 'COLLECTION') {
    const entries = Array.isArray(value) ? (value as unknown[]) : [];
    if (def.required === true && entries.length === 0) return 'required';
    const needed = (def.fields ?? []).filter((field) => field.required === true);
    const incomplete = entries.some((entry) => typeof entry !== 'object' || entry === null || needed.some((field) => empty((entry as Record<string, unknown>)[field.name])));
    return incomplete ? 'incomplete' : undefined;
  }
  if (empty(value)) return def.required === true ? 'required' : undefined;
  if (isExpression(value)) return undefined;
  if (def.type === 'NUMBER' && (typeof value !== 'number' || !Number.isFinite(value))) return 'number';
  if (def.type === 'JSON' && typeof value === 'string' && !isJson(value)) return 'json';
  if (def.type === 'OPTIONS' && def.options && typeof value === 'string' && !def.options.includes(value)) return 'option';
  return undefined;
}

/** The problems of every shown parameter, by parameter name. */
export function problemsOf(defs: readonly ParamDef[], parameters: Parameters, isJson?: (text: string) => boolean): Record<string, ProblemCode> {
  const found: Record<string, ProblemCode> = {};
  for (const def of defs) {
    if (def.widget !== undefined || !isVisible(def, parameters)) continue;
    const problem = problemOf(def, parameters, isJson);
    if (problem) found[def.name] = problem;
  }
  return found;
}

/** What to store for a value a person typed: an optional text left empty is removed, not stored as "". */
export function normalize(def: ParamDef, value: unknown): unknown {
  if (value === '' && def.required !== true && (def.type === 'STRING' || def.type === 'BINARY_PROPERTY' || def.type === 'DATE_TIME')) return undefined;
  return value;
}

/** The same members without the ones set to `undefined`: a block is written without them, never with them. */
export function withoutUndefined(block: Readonly<Record<string, unknown>>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(block).filter(([, member]) => member !== undefined));
}
