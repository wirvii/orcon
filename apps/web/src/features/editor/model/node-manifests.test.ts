// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { describe, expect, it } from 'vitest';

import { CREDENTIAL_TYPES } from './credential-types';
import { exampleFlowDocument } from './example-flow';
import { findNode, type FlowNode } from './flow-document';
import { catalogText, effectOf, isUnsupported, localeOf, manifestOf, parametersFor, type NodeManifest, type ParamDef } from './node-manifests';
import { NODE_TYPES, UNSUPPORTED_TYPE } from './node-types';

/** The manifests the editor knows, found through the nodes of the palette and the sample flow. */
function allManifests(): NodeManifest[] {
  const found = new Map<string, NodeManifest>();
  for (const def of NODE_TYPES) {
    const manifest = manifestOf({ type: def.type, element: def.element, parameters: def.initial().parameters });
    if (manifest) found.set(manifest.key, manifest);
  }
  const call = manifestOf({ type: 'orcon.callFlow@1', element: 'callActivity', parameters: {} });
  if (call) found.set(call.key, call);
  return [...found.values()];
}

const node = (id: string): FlowNode => {
  const found = findNode(exampleFlowDocument(), id);
  if (!found) throw new Error(`no node ${id}`);
  return found;
};

describe('node manifests', () => {
  it('covers every node type of the palette and the call node', () => {
    const keys = allManifests().map((manifest) => manifest.key);
    for (const expected of ['webhook', 'manual', 'if', 'http', 'set', 'noop', 'merge', 'loop', 'end', 'call']) expect(keys).toContain(expected);
  });

  it('chooses the trigger manifest by whether the start node has a path', () => {
    expect(manifestOf(node('webhook'))?.key).toBe('webhook');
    expect(manifestOf({ type: 'orcon.start@1', element: 'startEvent', parameters: {} })?.key).toBe('manual');
  });

  it('chooses the call manifest by the element, whatever the type is called', () => {
    expect(manifestOf({ type: 'acme.subflow@3', element: 'callActivity', parameters: {} })?.key).toBe('call');
  });

  it('has no manifest for a type it does not know, and flags the import placeholder', () => {
    expect(manifestOf({ type: 'acme.unknown@1', element: 'task', parameters: {} })).toBeUndefined();
    expect(isUnsupported({ type: UNSUPPORTED_TYPE })).toBe(true);
    expect(isUnsupported({ type: 'orcon.http@1' })).toBe(false);
  });

  describe('the language catalogs', () => {
    const catalogKeys = (manifest: NodeManifest, locale: 'en' | 'es') => Object.keys(manifest.catalog[locale]).sort();

    it('have the same keys in English and Spanish', () => {
      for (const manifest of allManifests()) expect(catalogKeys(manifest, 'es'), manifest.key).toEqual(catalogKeys(manifest, 'en'));
      for (const type of CREDENTIAL_TYPES) expect(Object.keys(type.catalog.es).sort(), type.id).toEqual(Object.keys(type.catalog.en).sort());
    });

    it('give a label to every parameter, every option and every field of a list, and a line to every operation', () => {
      const labelled = (manifest: NodeManifest, def: ParamDef) => {
        const missing: string[] = [];
        const need = (key: string) => {
          if (!(key in manifest.catalog.en)) missing.push(key);
        };
        if (def.widget === undefined) need(`param.${def.name}.label`);
        for (const option of def.options ?? []) need(`param.${def.name}.option.${option}`);
        for (const field of def.fields ?? []) need(`param.${def.name}.field.${field.name}`);
        return missing;
      };
      for (const manifest of allManifests()) {
        for (const def of manifest.parameters) expect(labelled(manifest, def), `${manifest.key}.${def.name}`).toEqual([]);
        for (const operation of manifest.operations) expect(manifest.catalog.en[operation], `${manifest.key} ${operation}`).toBeTruthy();
        expect(manifest.catalog.en['description'], manifest.key).toBeTruthy();
        for (const slot of manifest.credentials) {
          expect(manifest.catalog.en[`credential.${slot.slot}.label`]).toBeTruthy();
          expect(manifest.catalog.en[`credential.${slot.slot}.note`]).toBeTruthy();
        }
      }
    });

    it('fall back to English, then to the key', () => {
      const manifest = manifestOf(node('slack'));
      if (!manifest) throw new Error('no manifest');
      expect(catalogText(manifest, 'es', 'param.url.label')).toBe('URL');
      expect(catalogText(manifest, 'es', 'param.method.label')).toBe('Método');
      expect(catalogText(manifest, 'en', 'nothing.here')).toBe('nothing.here');
    });

    it('picks the language from the locale tag', () => {
      expect(localeOf('es-CO')).toBe('es');
      expect(localeOf('es')).toBe('es');
      expect(localeOf('en-US')).toBe('en');
      expect(localeOf('xx-LS')).toBe('en');
    });
  });

  it('hides the parameters a pinned connector version does not have yet', () => {
    const manifest = manifestOf(node('slack'));
    if (!manifest) throw new Error('no manifest');
    const names = (version: string | undefined) => parametersFor(manifest, version).map((param) => param.name);
    expect(names('1.0.0')).not.toContain('responseFormat');
    expect(names('1.1.0')).toContain('responseFormat');
    expect(names('1.1.0')).not.toContain('followRedirects');
    expect(names('1.2.0')).toContain('followRedirects');
    // A node that is not pinned has the newest.
    expect(names(undefined)).toContain('followRedirects');
  });

  it('classifies an HTTP request by its method for the retry default', () => {
    const manifest = manifestOf(node('slack'));
    if (!manifest) throw new Error('no manifest');
    expect(effectOf(manifest, { method: 'GET' })).toBe('read');
    expect(effectOf(manifest, { method: 'head' })).toBe('read');
    expect(effectOf(manifest, {})).toBe('read');
    expect(effectOf(manifest, { method: 'POST' })).toBe('write');
    // A method chosen by a formula cannot be known: the careful reading is that it writes.
    expect(effectOf(manifest, { method: '={{ $json.method }}' })).toBe('write');
    const check = manifestOf(node('validate'));
    if (!check) throw new Error('no manifest');
    expect(effectOf(check, {})).toBe('pure');
  });
});
