// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import {
  expressionCapable,
  expressionSource,
  isExpression,
  isVisible,
  modeOf,
  normalize,
  problemOf,
  problemsOf,
  switchMode,
  toExpression,
  valueOf,
  withoutUndefined,
} from './node-config';
import type { ParamDef } from './node-manifests';

const text: ParamDef = { name: 'url', type: 'STRING', required: true };
const count: ParamDef = { name: 'limit', type: 'NUMBER' };
const choice: ParamDef = { name: 'method', type: 'OPTIONS', default: 'GET', options: ['GET', 'POST'] };
const condition: ParamDef = { name: 'condition', type: 'STRING', required: true, expression: 'default' };

describe('expressions', () => {
  it('are strings that start with an equals sign', () => {
    expect(isExpression('={{ $json.id }}')).toBe(true);
    expect(isExpression('plain')).toBe(false);
    expect(isExpression(5)).toBe(false);
    expect(expressionSource('={{ $json.id }}')).toBe('{{ $json.id }}');
    expect(expressionSource('plain')).toBe('');
    expect(toExpression('{{ 1 }}')).toBe('={{ 1 }}');
  });

  it('apply to text-like parameters and not to booleans, lists or custom widgets', () => {
    expect(expressionCapable(text)).toBe(true);
    expect(expressionCapable(count)).toBe(true);
    expect(expressionCapable(choice)).toBe(true);
    expect(expressionCapable({ name: 'on', type: 'BOOLEAN' })).toBe(false);
    expect(expressionCapable({ name: 'headers', type: 'FIXED_COLLECTION' })).toBe(false);
    expect(expressionCapable({ name: 'call', type: 'RESOURCE_LOCATOR', widget: 'flow-target' })).toBe(false);
  });
});

describe('the mode of a parameter', () => {
  it('is an expression when the value is one, or when it is empty and the parameter starts as a formula', () => {
    expect(modeOf(text, { url: '=https://x/{{ $json.id }}' })).toBe('expression');
    expect(modeOf(text, { url: 'https://x' })).toBe('fixed');
    expect(modeOf(text, {})).toBe('fixed');
    expect(modeOf(condition, {})).toBe('expression');
    expect(modeOf(condition, { condition: '' })).toBe('expression');
  });

  it('carries the text over when it switches', () => {
    expect(switchMode(text, { url: 'https://x' }, 'expression')).toBe('=https://x');
    expect(switchMode(text, { url: '=https://x' }, 'fixed')).toBe('https://x');
    expect(switchMode(text, { url: '=' }, 'fixed')).toBeUndefined();
    // The default of a parameter that holds nothing is what the person sees, so it is what becomes the formula.
    expect(switchMode(choice, {}, 'expression')).toBe('=GET');
  });

  it('gives a number back only when the formula is one', () => {
    expect(switchMode(count, { limit: '=42' }, 'fixed')).toBe(42);
    expect(switchMode(count, { limit: '={{ $json.n }}' }, 'fixed')).toBeUndefined();
    expect(switchMode(count, { limit: 7 }, 'expression')).toBe('=7');
  });
});

describe('what a node holds and what shows', () => {
  it('uses the default when the node holds nothing', () => {
    expect(valueOf(choice, {})).toBe('GET');
    expect(valueOf(choice, { method: 'POST' })).toBe('POST');
  });

  it('shows a parameter only while all its conditions hold', () => {
    const body: ParamDef = { name: 'body', type: 'STRING', displayWhen: [{ param: 'method', oneOf: ['POST', 'PUT'] }] };
    expect(isVisible(body, { method: 'POST' })).toBe(true);
    expect(isVisible(body, { method: 'GET' })).toBe(false);
    expect(isVisible(body, {})).toBe(false);
    expect(isVisible(body, { method: '=$json.m' })).toBe(false);
    expect(isVisible(text, {})).toBe(true);
    const both: ParamDef = { name: 'x', type: 'STRING', displayWhen: [{ param: 'a', oneOf: [true] }, { param: 'b', oneOf: [1] }] };
    expect(isVisible(both, { a: true, b: 1 })).toBe(true);
    expect(isVisible(both, { a: true, b: 2 })).toBe(false);
  });
});

describe('the problem with a value', () => {
  it('is none for a filled field and for a formula', () => {
    expect(problemOf(text, { url: 'https://x' })).toBeUndefined();
    expect(problemOf(count, { limit: '={{ $json.n }}' })).toBeUndefined();
    expect(problemOf({ name: 'json', type: 'JSON' }, { json: '={{ $json }}' }, () => false)).toBeUndefined();
  });

  it('says a required field is empty, also for a formula with nothing after the equals sign', () => {
    expect(problemOf(text, {})).toBe('required');
    expect(problemOf(text, { url: '' })).toBe('required');
    expect(problemOf(text, { url: '=' })).toBe('required');
    expect(problemOf({ name: 'note', type: 'STRING' }, {})).toBeUndefined();
  });

  it('says a number field holds text, a JSON field holds invalid JSON, an option is unknown', () => {
    expect(problemOf(count, { limit: '12a' })).toBe('number');
    expect(problemOf(count, { limit: 12 })).toBeUndefined();
    expect(problemOf({ name: 'json', type: 'JSON' }, { json: '{' }, (value) => value === '{}')).toBe('json');
    expect(problemOf({ name: 'json', type: 'JSON' }, { json: '{}' }, (value) => value === '{}')).toBeUndefined();
    expect(problemOf(choice, { method: 'TRACE' })).toBe('option');
  });

  it('never faults a switch', () => {
    expect(problemOf({ name: 'on', type: 'BOOLEAN', required: true }, {})).toBeUndefined();
  });

  it('says a list is incomplete when an entry lacks a required field, and empty when it is required and has none', () => {
    const headers: ParamDef = { name: 'headers', type: 'FIXED_COLLECTION', fields: [{ name: 'name', type: 'STRING', required: true }, { name: 'value', type: 'STRING' }] };
    expect(problemOf(headers, {})).toBeUndefined();
    expect(problemOf(headers, { headers: [{ name: 'X-A', value: '1' }] })).toBeUndefined();
    expect(problemOf(headers, { headers: [{ name: '', value: '1' }] })).toBe('incomplete');
    expect(problemOf({ ...headers, required: true }, { headers: [] })).toBe('required');
  });

  it('is collected for the shown parameters only', () => {
    const body: ParamDef = { name: 'body', type: 'STRING', required: true, displayWhen: [{ param: 'method', oneOf: ['POST'] }] };
    expect(problemsOf([text, body], { method: 'GET' })).toEqual({ url: 'required' });
    expect(problemsOf([text, body], { method: 'POST', url: 'x' })).toEqual({ body: 'required' });
    expect(problemsOf([{ name: 'call', type: 'RESOURCE_LOCATOR', required: true, widget: 'flow-target' }], {})).toEqual({});
  });
});

describe('what is stored for what was typed', () => {
  it('drops an optional text left empty and keeps a required one', () => {
    expect(normalize({ name: 'note', type: 'STRING' }, '')).toBeUndefined();
    expect(normalize(text, '')).toBe('');
    expect(normalize({ name: 'note', type: 'STRING' }, 'a')).toBe('a');
    expect(normalize(count, 0)).toBe(0);
  });

  it('writes a block without the members set to undefined', () => {
    expect(withoutUndefined({ a: 1, b: undefined, c: null, d: false })).toEqual({ a: 1, c: null, d: false });
  });
});
