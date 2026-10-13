// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { existsSync, readFileSync } from 'node:fs';
import { dirname, isAbsolute, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parse } from 'yaml';

import { MUTATING, isObject } from '../model.js';
import { SNAKE, STABILITIES, defineRule, lastLiteral } from '../rule.js';
import { READ_VERBS, RUN_VERBS } from '../verbs.js';

const headerParam = (model, entry, name) => model.parameters(entry).find((p) => p.in === 'header' && p.name.toLowerCase() === name.toLowerCase());

const REQUIRED_IDEMPOTENCY_IDS = /^(runFlow|createApiKey|rotateApiKey|createOauthClient|createOAuthClient|createEventSubscription|importFlows\w*|startImport\w*|syncGit\w*|\w*GitSync\w*)$/;

function mustBeRequired(entry, options) {
  if (entry.op['x-orcon-long-running'] !== undefined) return 'is long-running';
  const last = lastLiteral(entry.key);
  if (last === 'redeliver' || last === 'replay') return `is a ${last}`;
  if (['continue', 'repeat', 'cancel'].includes(last) && entry.key.includes('awaiting-human')) return `is a ${last} of a human-attention item`;
  const pattern = options.requiredIdempotencyOperationIds ? new RegExp(options.requiredIdempotencyOperationIds) : REQUIRED_IDEMPOTENCY_IDS;
  if (typeof entry.op.operationId === 'string' && pattern.test(entry.op.operationId)) return `is ${entry.op.operationId}`;
  return undefined;
}

export const idempotency = defineRule((model, report, options) => {
  for (const entry of model.contractOperations) {
    if (entry.method !== 'post') continue;
    const value = entry.op['x-orcon-idempotency'];
    if (value !== 'required' && value !== 'optional') {
      report('IDEM-1', `POST ${entry.key} declares x-orcon-idempotency: required | optional.`, entry.loc);
      continue;
    }
    const header = headerParam(model, entry, 'Idempotency-Key');
    if (!header) {
      report('IDEM-1', `POST ${entry.key} declares x-orcon-idempotency but takes no Idempotency-Key header.`, entry.loc);
    } else if (value === 'required' && header.param.required !== true) {
      report('IDEM-1', 'Idempotency-Key must be a required header when x-orcon-idempotency is required.', header.loc);
    }
    if (value === 'optional') {
      const why = mustBeRequired(entry, options);
      if (why) report('IDEM-2', `x-orcon-idempotency must be required: the operation ${why}.`, entry.loc.child(['x-orcon-idempotency']));
    }
  }
});

export const concurrency = defineRule((model, report) => {
  for (const entry of model.contractOperations) {
    if (['put', 'patch', 'delete'].includes(entry.method)) {
      const header = headerParam(model, entry, 'If-Match');
      if (!header || header.param.required !== true) {
        report('IDEM-5', `${entry.method.toUpperCase()} ${entry.key} needs a required If-Match header.`, header ? header.loc : entry.loc);
      }
      const statuses = model.responses(entry).map((r) => r.status);
      for (const needed of ['412', '428']) {
        if (!statuses.includes(needed)) report('IDEM-5', `${entry.method.toUpperCase()} ${entry.key} declares no ${needed} response.`, entry.loc.child(['responses']));
      }
    }
    if (entry.method === 'get' && ['item', 'singleton'].includes(entry.item['x-orcon-resource']) && ['put', 'patch', 'delete'].some((m) => isObject(entry.item[m]))) {
      for (const r of model.responses(entry)) {
        if (r.status !== '200') continue;
        const hasEtag = model.responseHeaders(r.resp, r.loc).some((h) => h.name.toLowerCase() === 'etag');
        if (!hasEtag) report('IDEM-5', `GET ${entry.key} returns an editable resource and must send an ETag header.`, r.loc);
      }
    }
  }
});

const RATE_CLASSES = ['read', 'write', 'run', 'stream'];
const RUN_OPERATION_IDS = /^(run|execute|testRun|retry|repeat|redeliver|replay)/;

export const rateClass = defineRule((model, report) => {
  for (const entry of model.pathOperations) {
    const value = entry.op['x-orcon-rate-class'];
    if (!RATE_CLASSES.includes(value)) {
      report('RATE-1', `${entry.method.toUpperCase()} ${entry.key} declares x-orcon-rate-class: ${RATE_CLASSES.join(' | ')}.`, entry.loc);
      continue;
    }
    const responses = model.responses(entry);
    const streams = responses.some((r) => isObject(r.resp.content) && 'text/event-stream' in r.resp.content);
    const loc = entry.loc.child(['x-orcon-rate-class']);
    if (streams) {
      if (value !== 'stream') report('RATE-2', 'A text/event-stream operation uses the stream class.', loc);
    } else if (entry.method === 'get') {
      if (value !== 'read') report('RATE-2', 'GET uses the read class.', loc);
    } else {
      if (value === 'read' || value === 'stream') report('RATE-2', `A ${entry.method.toUpperCase()} mutation uses the write or run class, not ${value}.`, loc);
      const last = lastLiteral(entry.key);
      const runs = RUN_VERBS.includes(last) || RUN_OPERATION_IDS.test(entry.op.operationId ?? '');
      if (runs && value !== 'run') report('RATE-2', 'Operations that start executions, retries, repeats, redeliveries or replays use the run class.', loc);
    }
    for (const r of responses) {
      if (r.status !== '429') continue;
      const has = model.responseHeaders(r.resp, r.loc).some((h) => h.name.toLowerCase() === 'retry-after');
      if (!has) report('RATE-3', '429 responses carry a Retry-After header.', r.loc);
    }
  }
});

export const longRunning = defineRule((model, report) => {
  for (const entry of model.contractOperations) {
    const declared = entry.op['x-orcon-long-running'];
    const accepted = model.responses(entry).find((r) => r.status === '202');
    if (declared !== undefined) {
      if (!isObject(declared) || typeof declared.kind !== 'string' || declared.kind === '') {
        report('LRO-1', 'x-orcon-long-running is { kind }.', entry.loc.child(['x-orcon-long-running']));
      }
      if (!accepted) {
        report('LRO-1', 'A long-running operation returns 202 with Location and the Operation resource.', entry.loc.child(['responses']));
      }
      const key = headerParam(model, entry, 'Idempotency-Key');
      if (!key || key.param.required !== true || entry.op['x-orcon-idempotency'] !== 'required') {
        report('LRO-3', 'A long-running operation requires a required Idempotency-Key and x-orcon-idempotency: required.', entry.loc);
      }
    }
    if (accepted) {
      const hasLocation = model.responseHeaders(accepted.resp, accepted.loc).some((h) => h.name.toLowerCase() === 'location');
      if (!hasLocation) report('LRO-1', '202 responses carry a Location header.', accepted.loc);
      if (declared !== undefined) {
        const media = Object.values(accepted.resp.content ?? {})[0];
        const ref = media?.schema?.$ref;
        if (typeof ref !== 'string' || !/(^|[/#])Operation(\.ya?ml|\.json)?$/.test(ref)) {
          report('LRO-1', 'A long-running 202 returns the Operation resource.', accepted.loc);
        }
      }
    }
  }
});

export const stability = defineRule((model, report) => {
  for (const entry of model.operations) {
    const value = entry.op['x-orcon-stability'];
    if (!STABILITIES.includes(value)) {
      report('VER-1', `${entry.kind === 'webhook' ? 'Webhook' : entry.method.toUpperCase()} ${entry.key} declares x-orcon-stability: ${STABILITIES.join(' | ')}.`, entry.loc);
      continue;
    }
    if (value === 'experimental' && entry.kind === 'path') {
      if (!headerParam(model, entry, 'Orcon-Experimental')) report('VER-1', 'An experimental operation takes the Orcon-Experimental header.', entry.loc);
      const problems = entry.op['x-orcon-problems'];
      if (!Array.isArray(problems) || !problems.includes('experimental-not-enabled')) {
        report('VER-1', 'An experimental operation declares the experimental-not-enabled problem.', entry.loc);
      }
    }
  }
});

const DATE = /^\d{4}-\d{2}-\d{2}([Tt ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?([Zz]|[+-]\d{2}:\d{2})?)?$/;

function checkDeprecation(node, loc, report, what) {
  if (!isObject(node)) return;
  const deprecated = node.deprecated === true;
  const sunset = node['x-orcon-sunset'];
  const guide = node['x-orcon-deprecation-guide'];
  if (deprecated) {
    if (sunset === undefined) report('VER-6', `${what} is deprecated and needs x-orcon-sunset.`, loc);
    if (guide === undefined) report('VER-6', `${what} is deprecated and needs x-orcon-deprecation-guide.`, loc);
  } else if (sunset !== undefined || guide !== undefined) {
    report('VER-6', `${what} has x-orcon-sunset or x-orcon-deprecation-guide but is not deprecated: true.`, loc);
  }
  if (sunset !== undefined && !(typeof sunset === 'string' && DATE.test(sunset))) {
    report('VER-6', 'x-orcon-sunset is an RFC 3339 date.', loc.child(['x-orcon-sunset']));
  }
  if (guide !== undefined && !(typeof guide === 'string' && /^https?:\/\/\S+$|^\/\S*$/.test(guide))) {
    report('VER-6', 'x-orcon-deprecation-guide is a URL.', loc.child(['x-orcon-deprecation-guide']));
  }
}

export const deprecation = defineRule((model, report) => {
  for (const entry of model.operations) {
    checkDeprecation(entry.op, entry.loc, report, `Operation ${entry.op.operationId ?? entry.key}`);
    for (const p of model.parameters(entry)) checkDeprecation(p.param, p.loc, report, `Parameter ${p.name}`);
  }
  for (const { schema, loc } of model.allSchemas) checkDeprecation(schema, loc, report, 'Schema');
});

/** Loads the set of permission names from the catalog file (every `resource:action` string). */
const permissionCache = new Map();
function repoRoot() {
  let dir = dirname(fileURLToPath(import.meta.url));
  for (let i = 0; i < 8; i += 1) {
    if (existsSync(join(dir, 'pnpm-workspace.yaml'))) return dir;
    dir = dirname(dir);
  }
  return process.cwd();
}

export function loadPermissions(file) {
  const candidates = isAbsolute(file) ? [file] : [resolve(process.cwd(), file), resolve(repoRoot(), file)];
  const found = candidates.find((c) => existsSync(c));
  if (!found) return { error: `permissions file not found: ${file}` };
  if (permissionCache.has(found)) return permissionCache.get(found);
  const names = new Set();
  const visit = (v) => {
    if (typeof v === 'string') {
      if (/^[a-z][a-z0-9-]*:[a-z][a-z0-9-]*$/.test(v)) names.add(v);
    } else if (Array.isArray(v)) {
      v.forEach(visit);
    } else if (isObject(v)) {
      for (const [k, val] of Object.entries(v)) {
        visit(k);
        visit(val);
      }
    }
  };
  visit(parse(readFileSync(found, 'utf8')));
  const result = { names, file: found };
  permissionCache.set(found, result);
  return result;
}

export function permissionNames(value) {
  if (typeof value === 'string') return [value];
  if (isObject(value) && Array.isArray(value.allOf)) return value.allOf;
  if (isObject(value) && Array.isArray(value.anyOf)) return value.anyOf;
  return undefined;
}

export const permission = defineRule((model, report, options) => {
  const catalog = typeof options.permissionsFile === 'string' ? loadPermissions(options.permissionsFile) : undefined;
  if (catalog?.error) report('CHAN-7', `${catalog.error}. Set the permissionsFile option of orcon/permission.`, model.rootLoc);
  for (const entry of model.pathOperations) {
    const value = entry.op['x-orcon-permission'];
    const loc = entry.loc.child(['x-orcon-permission']);
    if (value === undefined) {
      report('CHAN-7', `${entry.method.toUpperCase()} ${entry.key} declares x-orcon-permission.`, entry.loc);
      continue;
    }
    const names = permissionNames(value);
    if (!names || names.length === 0 || names.some((n) => typeof n !== 'string')) {
      report('CHAN-7', 'x-orcon-permission is a permission, { allOf: [...] }, { anyOf: [...] }, authenticated or anonymous.', loc);
      continue;
    }
    for (const name of names) {
      if (name === 'anonymous') {
        if (!entry.protocol || typeof value !== 'string') report('CHAN-7', 'anonymous is allowed only on protocol operations.', loc);
      } else if (name !== 'authenticated' && catalog?.names && !catalog.names.has(name)) {
        report('CHAN-7', `Permission "${name}" is not in the permission catalog.`, loc);
      }
    }
  }
});

const AUDIT_ACTION = /^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$/;

export const audit = defineRule((model, report) => {
  for (const entry of model.pathOperations) {
    const value = entry.op['x-orcon-audit'];
    if (typeof value !== 'string' || value === '') {
      report('CHAN-8', `${entry.method.toUpperCase()} ${entry.key} declares x-orcon-audit (an action like flow.published, or none).`, entry.loc);
      continue;
    }
    if (value !== 'none' && !AUDIT_ACTION.test(value)) {
      report('CHAN-8', `Audit action "${value}" is <area>.<event> in lowercase.`, entry.loc.child(['x-orcon-audit']));
    }
    if (value === 'none' && MUTATING.includes(entry.method)) {
      const reason = entry.op['x-orcon-audit-reason'];
      if (typeof reason !== 'string' || reason.trim() === '') {
        report('CHAN-8', 'A mutation with x-orcon-audit: none needs x-orcon-audit-reason.', entry.loc.child(['x-orcon-audit']));
      }
    }
  }
});

const CLI_COMMAND = /^[a-z][a-z0-9]*(-[a-z0-9]+)*( [a-z][a-z0-9]*(-[a-z0-9]+)*){1,2}$/;
const CLI_CATEGORIES = ['protocol', 'ingress', 'interactive-session-only'];

export const cliDeclared = defineRule((model, report) => {
  const seen = new Map();
  for (const entry of model.pathOperations) {
    const value = entry.op['x-orcon-cli'];
    const loc = entry.loc.child(['x-orcon-cli']);
    if (!isObject(value)) {
      report('CHAN-3', `${entry.method.toUpperCase()} ${entry.key} declares x-orcon-cli: { command } or { none: { category, reason } }.`, entry.loc);
      continue;
    }
    const hasCommand = value.command !== undefined;
    const hasNone = value.none !== undefined;
    if (hasCommand === hasNone) {
      report('CHAN-3', 'x-orcon-cli has exactly one of command and none.', loc);
      continue;
    }
    if (hasCommand) {
      if (typeof value.command !== 'string' || !CLI_COMMAND.test(value.command)) {
        report('CHAN-2', 'A CLI command is "<resource-plural-kebab> <verb>" in lowercase, two or three words (flows publish).', loc.child(['command']));
      } else if (seen.has(value.command)) {
        report('CHAN-2', `CLI command "${value.command}" is already used by ${seen.get(value.command)}.`, loc.child(['command']));
      } else {
        seen.set(value.command, entry.op.operationId ?? `${entry.method} ${entry.key}`);
      }
    } else {
      const none = value.none;
      if (!isObject(none) || !CLI_CATEGORIES.includes(none.category) || typeof none.reason !== 'string' || none.reason.trim() === '') {
        report('CHAN-3', `x-orcon-cli none needs a category (${CLI_CATEGORIES.join(', ')}) and a reason.`, loc.child(['none']));
      }
    }
  }
});

const MCP_KEYS = ['level', 'tool', 'description'];
const MCP_LEVELS = ['none', 'read', 'write', 'destructive'];

/** V1 classification of the MCP tools (CHAN-6). */
export const MCP_V1 = Object.freeze({
  read: ['list_projects', 'search_flows', 'get_flow', 'get_draft', 'validate_draft', 'list_node_types', 'get_node_type', 'list_credentials', 'list_executions', 'get_execution', 'get_execution_events', 'explain_execution_error', 'list_awaiting_human'],
  write: ['create_flow', 'update_draft', 'test_run_draft', 'cancel_execution'],
  destructive: ['run_flow', 'retry_execution', 'publish_flow', 'activate_flow', 'deactivate_flow', 'continue_awaiting_human', 'repeat_awaiting_human', 'cancel_awaiting_human'],
});

export const mcpDeclared = defineRule((model, report) => {
  const tools = new Map();
  const v1Level = (tool) => Object.keys(MCP_V1).find((level) => MCP_V1[level].includes(tool));
  for (const entry of model.pathOperations) {
    const value = entry.op['x-orcon-mcp'];
    const loc = entry.loc.child(['x-orcon-mcp']);
    if (!isObject(value)) {
      report('CHAN-5', `${entry.method.toUpperCase()} ${entry.key} declares x-orcon-mcp: { level, tool?, description? }.`, entry.loc);
      continue;
    }
    if ('confirm' in value) report('CHAN-5', 'The key "confirm" no longer exists; confirmation is derived from level: destructive.', loc.child(['confirm']));
    for (const key of Object.keys(value)) {
      if (!MCP_KEYS.includes(key) && key !== 'confirm') report('CHAN-5', `x-orcon-mcp key "${key}" is not allowed (level, tool, description).`, loc.child([key]));
    }
    if (!MCP_LEVELS.includes(value.level)) {
      report('CHAN-5', `x-orcon-mcp.level is one of ${MCP_LEVELS.join(', ')}.`, loc.child(['level']));
      continue;
    }
    if (value.level !== 'none') {
      if (typeof value.tool !== 'string' || !SNAKE.test(value.tool)) {
        report('CHAN-5', 'An exposed operation names its tool in snake_case.', loc.child(['tool']));
      } else if (tools.has(value.tool)) {
        report('CHAN-5', `MCP tool "${value.tool}" is already used by ${tools.get(value.tool)}.`, loc.child(['tool']));
      } else {
        tools.set(value.tool, entry.op.operationId ?? `${entry.method} ${entry.key}`);
        const expected = v1Level(value.tool);
        if (expected && expected !== value.level) {
          report('CHAN-6', `Tool "${value.tool}" is classified ${expected} in V1; level is ${value.level}.`, loc.child(['level']));
        }
      }
    }
    if (entry.method === 'delete' && !['destructive', 'none'].includes(value.level)) {
      report('CHAN-5', 'Every exposed DELETE is destructive.', loc.child(['level']));
    }
    if (value.level === 'read' && entry.method !== 'get' && !READ_VERBS.includes(lastLiteral(entry.key))) {
      report('CHAN-5', `Level read is for GET or the ${READ_VERBS.join('/')} actions; ${entry.method.toUpperCase()} ${entry.key} changes state.`, loc.child(['level']));
    }
  }
});
