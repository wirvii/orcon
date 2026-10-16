// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest';

import type { FlowNode } from './flow-document';
import { nextStepSuggestions, noMatchActions } from './next-step-suggestions';

const node = (type: string, parameters: Record<string, unknown> = {}): FlowNode => ({ id: 'n', name: 'N', element: 'task', type, parameters });

test('nextStepSuggestions after an inbound message suggests AI if and reply', () => {
  const result = nextStepSuggestions(node('orcon.whatsapp.whatsapp@1'));
  expect(result.slugs).toEqual(['ai', 'if']);
  expect(result.replyWithSameConnector).toBe(true);
});

test('nextStepSuggestions without matches offers HTTP request', () => {
  expect(noMatchActions(0)).toEqual(['http-request', 'ask-ai']);
  expect(noMatchActions(3)).toEqual([]);
});

test('nextStepSuggestions reads the static table for a known step and nothing for an unknown one', () => {
  expect(nextStepSuggestions(node('orcon.http@1')).slugs).toEqual(['if', 'set', 'end']);
  expect(nextStepSuggestions(node('acme.thing@1')).slugs).toEqual([]);
  expect(nextStepSuggestions(undefined).slugs).toEqual([]);
});
