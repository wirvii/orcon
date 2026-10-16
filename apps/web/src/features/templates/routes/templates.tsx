// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { createFileRoute } from '@tanstack/react-router';
import { LayoutTemplate } from 'lucide-react';
import { defineMessages, useIntl } from 'react-intl';

import { RouteError } from '../../../app/errors/RouteError';
import { OrgSectionPlaceholder } from '../../../app/shell/OrgSectionPlaceholder';

const messages = defineMessages({
  title: { id: 'templates.title', defaultMessage: 'Templates', description: 'Main heading of the page of ready-made flows.' },
  description: {
    id: 'templates.description',
    defaultMessage: 'Ready-made flows are not available yet. For now, start from a blank flow.',
    description: 'One sentence on the templates page that says the feature does not exist yet and what to do meanwhile.',
  },
});

function TemplatesPage() {
  const intl = useIntl();
  return (
    <OrgSectionPlaceholder
      title={intl.formatMessage(messages.title)}
      description={intl.formatMessage(messages.description)}
      icon={LayoutTemplate}
      mark="/orgs/$orgSlug/templates"
    />
  );
}

/** Ready-made flows. There is no catalog of templates on the server yet, so the page says so (SPEC-019 R-UX-GEN-11). */
export const Route = createFileRoute('/_session/orgs/$orgSlug/templates')({
  errorComponent: RouteError,
  component: TemplatesPage,
});
