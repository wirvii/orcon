// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Link } from '@orcon/ui';
import { getRouteApi, Link as RouterLink, useNavigate } from '@tanstack/react-router';
import { ArrowLeft } from 'lucide-react';
import { useIntl } from 'react-intl';

import { runMessages } from '../components/messages';

import { ExecutionDetail } from './ExecutionDetail';

const orgRoute = getRouteApi('/_session/orgs/$orgSlug/executions/$executionId');
const projectRoute = getRouteApi('/_session/orgs/$orgSlug/projects/$projectHandle/executions/$executionId');

/** One run beside the list of the organization. On a phone it is its own screen, with a way back to the list. */
export function OrgExecutionScreen() {
  const intl = useIntl();
  const navigate = useNavigate({ from: '/orgs/$orgSlug/executions/$executionId' });
  const { organization } = orgRoute.useRouteContext();
  const { orgSlug, executionId } = orgRoute.useParams();
  const search = orgRoute.useSearch();
  return (
    <ExecutionDetail
      orgId={organization.id}
      executionId={executionId}
      mark="/orgs/$orgSlug/executions/$executionId"
      onRetried={(created) => {
        void navigate({ to: '/orgs/$orgSlug/executions/$executionId', params: { orgSlug, executionId: created }, search });
      }}
      back={
        <Link asChild className="md:hidden">
          <RouterLink to="/orgs/$orgSlug/executions" params={{ orgSlug }} search={search} className="inline-flex items-center gap-1">
            <ArrowLeft aria-hidden className="size-icon-16" />
            {intl.formatMessage(runMessages.back)}
          </RouterLink>
        </Link>
      }
    />
  );
}

/** One run of a project, on a page of its own (the editor and the list of the project link here). */
export function ProjectExecutionScreen() {
  const intl = useIntl();
  const navigate = useNavigate({ from: '/orgs/$orgSlug/projects/$projectHandle/executions/$executionId' });
  const { organization } = projectRoute.useRouteContext();
  const { orgSlug, projectHandle, executionId } = projectRoute.useParams();
  return (
    <ExecutionDetail
      orgId={organization.id}
      executionId={executionId}
      headingLevel={1}
      mark="/orgs/$orgSlug/projects/$projectHandle/executions/$executionId"
      onRetried={(created) => {
        void navigate({ to: '/orgs/$orgSlug/projects/$projectHandle/executions/$executionId', params: { orgSlug, projectHandle, executionId: created } });
      }}
      back={
        <Link asChild>
          <RouterLink to="/orgs/$orgSlug/projects/$projectHandle/executions" params={{ orgSlug, projectHandle }} className="inline-flex items-center gap-1">
            <ArrowLeft aria-hidden className="size-icon-16" />
            {intl.formatMessage(runMessages.back)}
          </RouterLink>
        </Link>
      }
    />
  );
}
