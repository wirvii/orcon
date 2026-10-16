// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Input } from '@orcon/ui';
import { Search } from 'lucide-react';
import { useState } from 'react';
import { useIntl } from 'react-intl';

import type { ServiceIcons } from '../model/service-icons';
import { createMessages } from '../model/messages';
import type { Service } from '../model/services';

import { ServiceIcon } from './ServiceIcon';

export interface ServiceListProps {
  services: readonly Service[];
  selectedId: string;
  onSelect: (id: string) => void;
  icons: ServiceIcons | undefined;
}

/** The services that match what was typed: by name, summary or keyword, ignoring case. */
function matching(services: readonly Service[], query: string, textOf: (service: Service) => string): readonly Service[] {
  const needle = query.trim().toLowerCase();
  if (needle === '') return services;
  return services.filter((service) => `${textOf(service)} ${service.keywords.join(' ')}`.toLowerCase().includes(needle));
}

/** The left column of the create dialog: a search box and the services, each with its icon. The chosen one is marked, not only colored. */
export function ServiceList({ services, selectedId, onSelect, icons }: ServiceListProps) {
  const intl = useIntl();
  const [query, setQuery] = useState('');
  const shown = matching(services, query, (service) => `${intl.formatMessage(service.name)} ${intl.formatMessage(service.summary)}`);
  return (
    <div className="flex flex-col gap-3">
      <h3 className="text-fg-muted text-sm font-medium">{intl.formatMessage(createMessages.servicesHeading)}</h3>
      <Input
        type="search"
        startIcon={Search}
        aria-label={intl.formatMessage(createMessages.search)}
        placeholder={intl.formatMessage(createMessages.search)}
        autoComplete="off"
        value={query}
        onChange={(event) => {
          setQuery(event.target.value);
        }}
      />
      {shown.length === 0 ? (
        <p role="status" className="text-fg-muted text-sm">
          {intl.formatMessage(createMessages.noService, {
            query: query.trim(),
          })}
        </p>
      ) : (
        <ul aria-label={intl.formatMessage(createMessages.servicesHeading)} className="flex flex-col gap-1">
          {shown.map((service) => {
            const name = intl.formatMessage(service.name);
            const selected = service.id === selectedId;
            return (
              <li key={service.id}>
                <button
                  type="button"
                  aria-pressed={selected}
                  onClick={() => {
                    onSelect(service.id);
                  }}
                  className="hover:bg-surface-hover aria-pressed:bg-surface-sunken aria-pressed:border-action-primary border-border flex min-h-target w-full items-center gap-3 rounded-md border p-2 text-left"
                >
                  {/* The name is in the text next to it, so the icon is not read out twice. */}
                  <span aria-hidden className="shrink-0">
                    <ServiceIcon connector={service.connector} name={name} icons={icons} />
                  </span>
                  <span className="flex min-w-0 flex-col">
                    <span className="font-medium wrap-anywhere">{name}</span>
                  </span>
                </button>
              </li>
            );
          })}
        </ul>
      )}
      <p className="text-fg-muted text-sm">{intl.formatMessage(createMessages.noServices)}</p>
    </div>
  );
}
