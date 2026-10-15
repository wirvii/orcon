// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute, notFound, redirect } from '@tanstack/react-router';
import { z } from 'zod';

import { apiRequest } from '../../../app/api/http';
import { RouteError } from '../../../app/errors/RouteError';
import { problemStatus } from '../../../app/errors/to-problem-details';

const flowIdSchema = z.uuid();

/**
 * The address the editor had before flows lived under a project (SPEC-016 Q1 = A). It names only the
 * flow, so this route finds the flow's project and organization and forwards the person to the
 * address that carries both. It sits under the session layout: a signed-out person is sent to sign-in
 * first, and a flow of an organization the person does not belong to is a page that does not exist.
 *
 * The three reads are made here, not through the projects, flows and session APIs: the boundary rules
 * (orcon-web AR-5) let a route import only its own area's API, and this forwarding spans three areas.
 * Only the fields the redirect needs are declared.
 */
export const Route = createFileRoute('/_session/flows/$flowId')({
  params: {
    parse: (raw) => {
      const flowId = flowIdSchema.safeParse(raw.flowId);
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
      if (!flowId.success) throw notFound();
      return { flowId: flowId.data };
    },
    stringify: ({ flowId }) => ({ flowId }),
  },
  beforeLoad: async ({ params }) => {
    const target = await findAddress(params.flowId);
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- redirect() is a marker the router catches
    throw redirect({ to: '/orgs/$orgSlug/projects/$projectHandle/flows/$flowId', params: { ...target, flowId: params.flowId }, replace: true });
  },
  errorComponent: RouteError,
});

const flowSchema = z.looseObject({ projectId: z.uuid() });
const projectSchema = z.looseObject({ orgId: z.uuid(), handle: z.string() });
const organizationPageSchema = z.object({ items: z.array(z.looseObject({ id: z.uuid(), slug: z.string() })) });

/** The organization slug and project handle of a flow, or the not-found answer when the person cannot see it. */
async function findAddress(flowId: string): Promise<{ orgSlug: string; projectHandle: string }> {
  try {
    const flow = await apiRequest({ method: 'GET', path: `/flows/${encodeURIComponent(flowId)}`, schema: flowSchema, source: 'response of GET /flows/{flowId}' });
    const project = await apiRequest({
      method: 'GET',
      path: `/projects/${encodeURIComponent(flow.projectId)}`,
      schema: projectSchema,
      source: 'response of GET /projects/{projectId}',
    });
    const orgs = await apiRequest({ method: 'GET', path: '/orgs', query: { limit: 200 }, schema: organizationPageSchema, source: 'response of GET /orgs' });
    const organization = orgs.items.find((org) => org.id === project.orgId);
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
    if (organization === undefined) throw notFound();
    return { orgSlug: organization.slug, projectHandle: project.handle };
  } catch (error) {
    // A flow the person cannot read, or that does not exist, answers 404 from the API (SPEC-016 R-SH-URL-04).
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- the router's notFound() is a marker object it catches, not an Error
    if (problemStatus(error) === 404) throw notFound();
    throw error;
  }
}
