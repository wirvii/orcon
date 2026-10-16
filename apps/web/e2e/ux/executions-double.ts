// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Page, Route } from '@playwright/test';

import { org } from '../journeys/api-double';

/**
 * Demo runs for the executions screens, served on top of the stand-in API (SPEC-019 UX7): a failed run whose step failed
 * for seven of twelve items, a run that finished but needs review, and a run that succeeded. They exist only in the
 * browser of a check or a capture; nothing here is data of the product.
 */

export interface DemoRuns {
  failedId: string;
  reviewId: string;
  succeededId: string;
}

const json = (body: unknown) => ({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });

const failedId = '0195f1a0-0000-7000-8000-00000000f001';
const reviewId = '0195f1a0-0000-7000-8000-00000000f002';
const succeededId = '0195f1a0-0000-7000-8000-00000000f003';

const nodes = [
  { id: 'cron', name: 'Cada hora' },
  { id: 'shopify', name: 'Pedidos sin pagar (Shopify)' },
  { id: 'loop', name: 'Por cada pedido' },
  { id: 'ai', name: 'Redactar recordatorio (IA)' },
  { id: 'send', name: 'Enviar por WhatsApp' },
  { id: 'mark', name: 'Marcar como recordado' },
];

/** Offsets in milliseconds from the start of the run, and how long each step took. */
const timings: Record<string, [number, number]> = { cron: [0, 2], shopify: [40, 1100], loop: [1150, 40], ai: [1200, 1700], send: [2900, 900] };

export async function mockExecutions(page: Page): Promise<DemoRuns> {
  const flows = await page.evaluate(async ({ orgId }) => {
    const response = await fetch(`/api/v1/orgs/${orgId}/flows?limit=100`);
    return ((await response.json()) as { items: { id: string; name: string; projectId: string }[] }).items;
  }, { orgId: org.id });
  const flow = flows.find((candidate) => candidate.name === 'Recordatorio de pago') ?? flows[0];
  if (flow === undefined) throw new Error('The stand-in API has no flow to attach the demo runs to.');

  const now = Date.now();
  const at = (minutesAgo: number, offset = 0) => new Date(now - minutesAgo * 60_000 + offset).toISOString();
  const base = { flowId: flow.id, projectId: flow.projectId, flowVersion: 14, dataHidden: false };
  const failure = {
    // The error code as the service reports it: a number in parentheses and its name.
    code: '(' + String.fromCharCode(35) + '131047) Re-engagement message',
    title: 'WhatsApp rechazó 7 mensajes: la ventana de 24 horas está cerrada.',
    detail: 'Estos clientes no te escriben hace más de un día. WhatsApp solo permite escribirles con una plantilla aprobada.',
    nodeId: 'send',
  };
  const runs = [
    { ...base, id: failedId, mode: 'TRIGGER', status: 'FAILED', hasUncertainOperation: false, createdAt: at(12), startedAt: at(12), finishedAt: at(12, 3800), failure },
    { ...base, id: reviewId, mode: 'WEBHOOK', status: 'SUCCEEDED', hasUncertainOperation: true, createdAt: at(31), startedAt: at(31), finishedAt: at(31, 2600) },
    { ...base, id: succeededId, mode: 'TRIGGER', status: 'SUCCEEDED', hasUncertainOperation: false, createdAt: at(75), startedAt: at(75), finishedAt: at(75, 3100) },
  ];
  const stepsOf = (runId: string, startedMinutesAgo: number, failing: boolean) => {
    const rows = Object.entries(timings).flatMap(([nodeId, [from, took]]) =>
      nodeId === 'send'
        ? Array.from({ length: 12 }, (_, item) => ({ nodeId, from, took, failed: failing && item < 7, item }))
        : [{ nodeId, from, took, failed: false, item: 0 }],
    );
    return rows.map((row, index) => ({
      id: `0195f1a0-0000-7000-8000-00000001${runId.slice(-2)}${String(index).padStart(2, '0')}`,
      executionId: runId,
      nodeId: row.nodeId,
      status: row.failed ? 'FAILED' : 'SUCCEEDED',
      attemptCount: row.failed ? 3 : 1,
      startedAt: at(startedMinutesAgo, row.from),
      finishedAt: at(startedMinutesAgo, row.from + row.took),
      ...(row.failed ? { failure: { code: failure.code, detail: failure.detail } } : {}),
    }));
  };

  const answer = (route: Route) => {
    const url = new URL(route.request().url());
    const path = url.pathname.replace('/api/v1', '');
    if (route.request().method() !== 'GET') return route.fallback();
    if (path === `/orgs/${org.id}/executions`) {
      const query = url.searchParams;
      const items = runs
        .filter((run) => query.get('uncertain') !== 'true' || run.hasUncertainOperation)
        .filter((run) => query.get('status') === null || run.status === query.get('status'))
        .filter((run) => query.get('id') === null || run.id === query.get('id'));
      return route.fulfill(json({ items }));
    }
    const one = /^\/executions\/([^/]+)(?:\/(nodes|data))?$/.exec(path);
    if (one !== null) {
      const run = runs.find((candidate) => candidate.id === one[1]);
      if (run === undefined) return route.fallback();
      const minutesAgo = run.id === failedId ? 12 : run.id === reviewId ? 31 : 75;
      if (one[2] === undefined) return route.fulfill(json(run));
      if (one[2] === 'nodes') return route.fulfill(json({ items: stepsOf(run.id, minutesAgo, run.id === failedId) }));
      const sample = stepsOf(run.id, minutesAgo, false).find((step) => step.nodeId === 'send');
      return route.fulfill(
        json({
          items: sample === undefined ? [] : [
            { nodeExecutionId: sample.id, nodeId: 'send', side: 'INPUT', port: 'main', itemCount: 12, dataItems: [{ telefono: '573104567890', mensaje: 'Hola Camila, tu pedido sigue pendiente de pago.' }] },
            { nodeExecutionId: sample.id, nodeId: 'send', side: 'OUTPUT', port: 'main', itemCount: 5, dataItems: [{ estado: 'enviado', id: 'wamid.HBgMNTczMTA0NTY3ODkw' }] },
          ],
        }),
      );
    }
    if (/^\/flows\/[^/]+\/versions\/14$/.test(path)) {
      return route.fulfill(json({ flowId: flow.id, version: 14, contentHash: 'sha256:demo', publishedAt: at(1440), warnings: [], document: { format: 'orcon/flow@1', nodes: nodes.map((node) => ({ ...node, type: 'set' })), edges: [] } }));
    }
    return route.fallback();
  };
  await page.route('**/api/v1/**', answer);
  return { failedId, reviewId, succeededId };
}
