// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The `orcon` Redocly plugin: the 31 contract rules of the orcon-api rulebook
 * (`.agents/skills/orcon-api/references/`). Each rule id is `orcon/<name>`; the
 * message of a finding starts with the rulebook rule id in brackets (for example
 * `[PATH-5]`). `redocly.yaml` enables every rule at severity `error` and none is
 * disabled for the real contract.
 */

import { docExamples, docFields, docOperation, codegenSubset, events } from './rules/docs.js';
import { enumUpperSnake, fieldCamelCase, idFormat, namedSchemas, secretFields, timeFormat } from './rules/fields.js';
import { audit, cliDeclared, concurrency, deprecation, idempotency, longRunning, mcpDeclared, permission, rateClass, stability } from './rules/operations.js';
import { actionVerb, headerNames, operationId, pathParamName, pathShape } from './rules/paths.js';
import { listPagination, problemCodesDeclared, problemResponses, responseShape, sortAllowlist } from './rules/responses.js';

/** Rule name -> implementation, in the order of the table in spec section 4.5. */
export const RULES = Object.freeze({
  'path-shape': pathShape,
  'action-verb': actionVerb,
  'path-param-name': pathParamName,
  'operation-id': operationId,
  'header-names': headerNames,
  'field-camel-case': fieldCamelCase,
  'id-format': idFormat,
  'enum-upper-snake': enumUpperSnake,
  'time-format': timeFormat,
  'secret-fields': secretFields,
  'named-schemas': namedSchemas,
  'response-shape': responseShape,
  'list-pagination': listPagination,
  'sort-allowlist': sortAllowlist,
  'problem-responses': problemResponses,
  'problem-codes-declared': problemCodesDeclared,
  idempotency,
  concurrency,
  'rate-class': rateClass,
  'long-running': longRunning,
  stability,
  deprecation,
  permission,
  audit,
  'cli-declared': cliDeclared,
  'mcp-declared': mcpDeclared,
  'doc-operation': docOperation,
  'doc-fields': docFields,
  'doc-examples': docExamples,
  events,
  'codegen-subset': codegenSubset,
});

export const RULE_NAMES = Object.freeze(Object.keys(RULES));

export default function orconPlugin() {
  return {
    id: 'orcon',
    rules: { oas3: RULES },
  };
}
