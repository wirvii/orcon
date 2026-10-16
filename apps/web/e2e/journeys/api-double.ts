// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Page, Route } from '@playwright/test';

import { subflowDocument } from '../../src/features/editor/model/example-flow';

/**
 * A stateful stand-in for the API behind the routes the sign-in journey uses (SPEC-016 §3.1, §3.4, §3.5
 * and the draft operations of SPEC-014). It keeps what a person creates for the length of one page, so the
 * journey can create a project and a flow and open them, the way the real server does. It is used only when
 * the journey runs without a server; a run against a real server answers with the server's own data.
 */

export const account = { email: 'ana@example.com', password: 'correct-horse-battery', displayName: 'Ana Ruiz' } as const;
export const org = { id: '0195f1a0-0000-7000-8000-0000000000c1', slug: 'acme', name: 'Acme' } as const;

interface Project {
  id: string;
  name: string;
  handle: string;
  environment: string | null;
}

interface Flow {
  id: string;
  projectId: string;
  name: string;
  revision: number;
  document: unknown;
  /** Set by the first publication; a flow without it cannot be activated. */
  activeVersion?: number;
  active: boolean;
  /** The activity a list shows next to the flow (SPEC-019 UXB1): only the demo data has any. */
  lastExecution?: { id: string; status: string; hasUncertainOperation: boolean; createdAt: string };
  executionCountLast24h: number;
}

const session = {
  id: '0195f1a0-0000-7000-8000-0000000000a1',
  createdAt: '2026-10-14T10:00:00Z',
  expiresAt: '2026-10-15T10:00:00Z',
  idleExpiresAt: '2026-10-14T12:00:00Z',
  assuranceLevel: 'AAL1',
} as const;

const problem = (status: number, title: string, code: string) => ({
  status,
  contentType: 'application/problem+json',
  body: JSON.stringify({ type: 'about:blank', status, title, code }),
});

const json = (status: number, body: unknown, headers: Record<string, string> = {}) => ({
  status,
  contentType: 'application/json',
  headers,
  body: JSON.stringify(body),
});

/** The handle the server derives from a project name: the name in readable form. */
const handleOf = (name: string) =>
  name
    .toLowerCase()
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');

/** The figures of the home page for the demo data (the answer of `getOrgActivitySummary`, SPEC-019 UXB2). */
const demoSummary = {
  activeFlowCount: 2,
  flowCount: 3,
  projectCount: 3,
  executionsLast24h: { total: 4812, failed: 7, succeeded: 4803, uncertain: 2, byHour: [30, 22, 18, 25, 40, 62, 80, 95, 88, 76, 70, 82, 91, 85, 77, 64, 58, 49, 43, 38, 35, 33, 31, 29] },
  timeSaved: { windowDays: 7, succeededExecutions: 620, minutesPerExecution: 3, minutes: 1860 },
};

/**
 * Installs the stand-in on a page. Each page starts signed out, with no projects, unless `signedIn` says otherwise.
 * `demo` gives the flows a last execution and an activity count and answers the home page figures, so a screenshot
 * shows a lived-in product; without it the flows have never run and the figures say they are not available.
 */
export async function installApiDouble(page: Page, options: { signedIn?: boolean; demo?: boolean } = {}): Promise<void> {
  let signedIn = options.signedIn ?? false;
  const demo = options.demo ?? false;
  const projects: Project[] = [];
  const flows: Flow[] = [];

  const answer = async (route: Route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace(/^\/api\/v1/, '');
    const method = request.method();

    if (method === 'POST' && path === '/users/me/sessions') {
      const body = request.postDataJSON() as { email: string; password: string };
      if (body.email === account.email && body.password === account.password) {
        signedIn = true;
        return route.fulfill(json(201, session));
      }
      return route.fulfill(problem(401, 'unauthenticated', 'unauthenticated'));
    }
    if (method === 'POST' && /^\/users\/me\/sessions\/[^/]+\/revoke$/.test(path)) {
      signedIn = false;
      return route.fulfill({ status: 204 });
    }
    if (path === '/users/me') {
      if (!signedIn) return route.fulfill(problem(401, 'unauthenticated', 'unauthenticated'));
      return route.fulfill(json(200, { id: '0195f1a0-0000-7000-8000-0000000000b1', email: account.email, displayName: account.displayName, currentSession: session }));
    }
    if (!signedIn) return route.fulfill(problem(401, 'unauthenticated', 'unauthenticated'));

    if (method === 'GET' && path === '/orgs') {
      const slug = new URL(request.url()).searchParams.get('slug');
      return route.fulfill(json(200, { items: slug === null || slug === org.slug ? [org] : [] }));
    }
    if (path === `/orgs/${org.id}/projects`) {
      if (method === 'POST') {
        const body = request.postDataJSON() as { name: string; environment?: string };
        const handle = handleOf(body.name);
        if (projects.some((project) => project.handle === handle)) return route.fulfill(problem(409, 'conflict', 'conflict'));
        const project: Project = { id: crypto.randomUUID(), name: body.name, handle, environment: body.environment ?? null };
        projects.push(project);
        return route.fulfill(json(201, { id: project.id, orgId: org.id, name: project.name, slug: handle, handle, environment: project.environment, protected: true, createdAt: '2026-10-14T10:00:00Z', updatedAt: '2026-10-14T10:00:00Z' }));
      }
      const handle = new URL(request.url()).searchParams.get('handle');
      const items = projects.filter((project) => handle === null || project.handle === handle).map((project) => ({
        id: project.id,
        orgId: org.id,
        name: project.name,
        slug: project.handle,
        handle: project.handle,
        environment: project.environment,
        protected: true,
        flowCount: flows.filter((flow) => flow.projectId === project.id).length,
        createdAt: '2026-10-14T10:00:00Z',
        updatedAt: '2026-10-14T10:00:00Z',
      }));
      return route.fulfill(json(200, { items }));
    }

    const projectFlows = /^\/projects\/([^/]+)\/flows$/.exec(path);
    if (projectFlows) {
      const projectId = projectFlows[1] ?? '';
      if (method === 'POST') {
        const body = request.postDataJSON() as { name: string };
        const id = crypto.randomUUID();
        const flow: Flow = { id, projectId, name: body.name, revision: 1, document: subflowDocument(id, body.name), active: false, executionCountLast24h: 0 };
        if (demo) giveActivity(flow, flows.length);
        flows.push(flow);
        return route.fulfill(json(201, flowState(flow)));
      }
      return route.fulfill(json(200, { items: flows.filter((flow) => flow.projectId === projectId).map(flowState) }));
    }

    if (method === 'GET' && path === `/orgs/${org.id}/flows`) {
      const query = new URL(request.url()).searchParams;
      const wanted = query.get('q')?.toLowerCase();
      const items = flows
        .filter((flow) => query.get('projectId') === null || flow.projectId === query.get('projectId'))
        .filter((flow) => query.get('activationState') === null || (flow.active ? 'ACTIVE' : 'INACTIVE') === query.get('activationState'))
        .filter((flow) => wanted === undefined || flow.name.toLowerCase().startsWith(wanted))
        .map((flow) => ({ ...flowState(flow), ...(flow.lastExecution === undefined ? {} : { lastExecution: flow.lastExecution }), executionCountLast24h: flow.executionCountLast24h }));
      return route.fulfill(json(200, { items }));
    }
    if (method === 'GET' && path === `/orgs/${org.id}/activity-summary`) {
      return demo ? route.fulfill(json(200, demoSummary)) : route.fulfill(problem(503, 'Service Unavailable', 'service-unavailable'));
    }

    const activation = /^\/flows\/([^/]+)\/(activate|deactivate)$/.exec(path);
    if (method === 'POST' && activation) {
      const flow = flows.find((candidate) => candidate.id === activation[1]);
      if (flow === undefined) return route.fulfill(problem(404, 'Flow not found', 'not-found'));
      if (request.headers()['if-match'] !== `"${flow.revision}"`) return route.fulfill(problem(412, 'The flow changed', 'precondition-failed'));
      if (activation[2] === 'activate' && flow.activeVersion === undefined) return route.fulfill(problem(409, 'The flow was never published', 'conflict'));
      flow.active = activation[2] === 'activate';
      return route.fulfill(json(200, flowState(flow), { ETag: `"${flow.revision}"` }));
    }

    const flowDraft = /^\/flows\/([^/]+)\/draft$/.exec(path);
    if (flowDraft) {
      const flow = flows.find((candidate) => candidate.id === flowDraft[1]);
      if (flow === undefined) return route.fulfill(problem(404, 'Flow not found', 'not-found'));
      if (method === 'PUT') {
        if (request.headers()['if-match'] !== `"${flow.revision}"`) return route.fulfill(problem(412, 'The draft changed', 'precondition-failed'));
        flow.document = (request.postDataJSON() as { document: unknown }).document;
        flow.revision += 1;
      }
      return route.fulfill(json(200, { flowId: flow.id, revision: flow.revision, document: flow.document, updatedAt: '2026-10-14T10:00:00Z' }, { ETag: `"${flow.revision}"` }));
    }

    const flowState_ = /^\/flows\/([^/]+)$/.exec(path);
    if (flowState_) {
      const flow = flows.find((candidate) => candidate.id === flowState_[1]);
      if (flow === undefined) return route.fulfill(problem(404, 'Flow not found', 'not-found'));
      return route.fulfill(json(200, flowState(flow), { ETag: `"${flow.revision}"` }));
    }

    // The lists of a project's credentials and executions: there are none.
    if (method === 'GET' && /^\/projects\/[^/]+\/(credentials|executions)$/.test(path)) return route.fulfill(json(200, { items: [] }));

    return route.fulfill(problem(404, 'Not found', 'not-found'));
  };

  await page.route('**/api/v1/**', answer);
}

/** Demo activity: the flows alternate between a flow that is live and ran fine, one that failed, one that needs review and one never published. */
function giveActivity(flow: Flow, index: number) {
  const kinds = [
    { status: 'SUCCEEDED', hasUncertainOperation: false, count: 124, published: true, active: true },
    { status: 'FAILED', hasUncertainOperation: false, count: 7, published: true, active: true },
    { status: 'SUCCEEDED', hasUncertainOperation: true, count: 31, published: true, active: false },
    { status: 'SUCCEEDED', hasUncertainOperation: false, count: 0, published: false, active: false },
  ] as const;
  const kind = kinds[index % kinds.length] ?? kinds[0];
  flow.active = kind.active;
  flow.executionCountLast24h = kind.count;
  if (kind.published) {
    flow.activeVersion = 1;
    flow.lastExecution = { id: crypto.randomUUID(), status: kind.status, hasUncertainOperation: kind.hasUncertainOperation, createdAt: new Date(Date.now() - (index + 1) * 7 * 60_000).toISOString() };
  }
}

function flowState(flow: Flow) {
  return {
    id: flow.id,
    projectId: flow.projectId,
    name: flow.name,
    activationState: flow.active ? 'ACTIVE' : 'INACTIVE',
    ...(flow.activeVersion === undefined ? {} : { activeVersion: flow.activeVersion }),
    hasUnpublishedChanges: flow.activeVersion === undefined,
    draftRevision: flow.revision,
    createdAt: '2026-10-14T10:00:00Z',
    updatedAt: '2026-10-14T10:00:00Z',
  };
}
