// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, type CatalogItem, type PaletteSuggestions } from '@orcon/ui';
import { Globe, Sparkles } from 'lucide-react';
import { useCallback, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { ADDED_ID_PREFIX } from '../model/node-types';
import { nextStepSuggestions } from '../model/next-step-suggestions';
import { useEditorStoreApi } from '../store/editor-context';

const messages = defineMessages({
  heading: {
    id: 'editor.palette.suggested',
    defaultMessage: 'Suggested after "{name}"',
    description: 'Heading of the first section of the list of steps to add: the steps that usually come next. {name} is the name of the step before.',
  },
  noMatchTitle: {
    id: 'editor.palette.noMatch.title',
    defaultMessage: 'Not finding it? These always work:',
    description: 'Introduces the two ways out shown when a search in the list of steps finds nothing.',
  },
  http: {
    id: 'editor.palette.noMatch.http',
    defaultMessage: 'Use an HTTP request',
    description: 'Button shown when a search finds nothing: adds the step that calls any web address.',
  },
  httpText: {
    id: 'editor.palette.noMatch.httpText',
    defaultMessage: 'It can reach any service that has an API.',
    description: 'Explains why an HTTP request step is the way out when the service searched for has no step of its own.',
  },
  askAi: {
    id: 'editor.palette.noMatch.askAi',
    defaultMessage: 'Ask the AI for the step',
    description: 'Disabled option shown when a search finds nothing: an assistant that would build the step. Not available yet.',
  },
  askAiText: {
    id: 'editor.palette.noMatch.askAiText',
    defaultMessage: 'The assistant that builds steps is not available yet.',
    description: 'Says the assistant that would build a missing step does not exist yet.',
  },
  soon: { id: 'editor.palette.noMatch.soon', defaultMessage: 'Coming soon', description: 'Label on an option that is not available yet.' },
});

/**
 * The steps worth trying first in the list of steps to add: after the step the new one goes behind, from the
 * static table of `model/next-step-suggestions.ts`. A step the list does not offer is skipped.
 */
export function usePaletteSuggest(catalog: readonly CatalogItem[]): (previousNodeId: string | undefined) => PaletteSuggestions | undefined {
  const intl = useIntl();
  const api = useEditorStoreApi();
  return useCallback(
    (previousNodeId) => {
      if (previousNodeId === undefined) return undefined;
      const previous = api.getState().document.nodes.find((node) => node.id === previousNodeId);
      const { slugs } = nextStepSuggestions(previous);
      const items = slugs.flatMap((slug) => catalog.find((item) => item.id === `${ADDED_ID_PREFIX}${slug}`) ?? []);
      if (previous === undefined || items.length === 0) return undefined;
      return { heading: intl.formatMessage(messages.heading, { name: previous.name }), items };
    },
    [api, catalog, intl],
  );
}

/** What the list of steps offers when a search finds nothing: an HTTP request, and the AI assistant as coming soon. */
export function PaletteNoMatch({ pick }: { pick: (itemId: string) => void }): ReactNode {
  const intl = useIntl();
  return (
    <div className="border-border mx-2 flex flex-col gap-3 rounded-md border p-3" data-palette-no-match>
      <p className="text-fg-muted text-sm">{intl.formatMessage(messages.noMatchTitle)}</p>
      <div className="flex flex-col gap-1">
        <Button
          variant="secondary"
          onClick={() => {
            pick(`${ADDED_ID_PREFIX}http`);
          }}
        >
          <Globe aria-hidden className="size-icon-16" />
          {intl.formatMessage(messages.http)}
        </Button>
        <p className="text-fg-muted text-xs">{intl.formatMessage(messages.httpText)}</p>
      </div>
      <div className="flex flex-col gap-1">
        <Button variant="secondary" disabled>
          <Sparkles aria-hidden className="size-icon-16" />
          {intl.formatMessage(messages.askAi)}
          <Badge tone="neutral">{intl.formatMessage(messages.soon)}</Badge>
        </Button>
        <p className="text-fg-muted text-xs">{intl.formatMessage(messages.askAiText)}</p>
      </div>
    </div>
  );
}
