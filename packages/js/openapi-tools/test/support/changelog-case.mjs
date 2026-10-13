// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The fixed pair of the changelog self-test (T10): `base` is the compatibility fixture and
 * `head` adds an operation, adds a response field, narrows a request enum (a breaking change),
 * deprecates an operation and a field, and adds an experimental operation.
 * testdata/changelog/{base,head}.yaml are generated from here; expected.md is the golden
 * output of `node scripts/changelog-selftest.mjs --update`, reviewed by a person.
 */

import { stringify } from 'yaml';

import { baseDocument } from './compat-cases.mjs';

export function changelogPair() {
  const head = baseDocument();
  head.paths['/flows/{flowId}'].delete = {
    operationId: 'deleteFlow',
    summary: 'Delete a flow',
    'x-orcon-stability': 'stable',
    responses: { 204: { description: 'Deleted.' } },
  };
  head.components.schemas.Flow.properties.description = { type: 'string' };
  head.components.schemas.CreateFlowRequest.properties.status.enum = ['ACTIVE'];
  head.paths['/flows'].get.deprecated = true;
  head.paths['/flows'].get['x-orcon-sunset'] = '2027-06-30';
  head.paths['/flows'].get['x-orcon-deprecation-guide'] = 'https://orcon.example/docs/deprecations/list-flows';
  head.components.schemas.Flow.properties.updatedAt.deprecated = true;
  head.components.schemas.Flow.properties.updatedAt['x-orcon-sunset'] = '2027-06-30';
  head.components.schemas.Flow.properties.updatedAt['x-orcon-deprecation-guide'] = 'https://orcon.example/docs/deprecations/updated-at';
  head.paths['/probes/{probeId}'] = {
    parameters: [{ name: 'probeId', in: 'path', required: true, schema: { type: 'string', format: 'uuid' } }],
    get: {
      operationId: 'getProbe',
      summary: 'Get a probe',
      'x-orcon-stability': 'experimental',
      responses: { 200: { description: 'The probe.' } },
    },
  };
  head.paths['/probes'].get.responses['200'].content['application/json'].schema.properties.total = { type: 'integer' };
  return { base: stringify(baseDocument(), { lineWidth: 0 }), head: stringify(head, { lineWidth: 0 }) };
}
