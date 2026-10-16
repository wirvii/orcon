// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, FilterChip, Input, Select, type SelectOption } from '@orcon/ui';
import { CircleX, Search, ShieldQuestion } from 'lucide-react';
import { useId, useState } from 'react';
import { useIntl } from 'react-intl';

import { readSearchText, type ExecutionRange, type ExecutionShow, type ExecutionsSearch } from '../model/params';

import { runMessages } from './messages';

/** Radix selects cannot hold an empty value, so "no filter" is a word of its own. */
const ANY = '__any';

export interface ExecutionFiltersProps {
  search: ExecutionsSearch;
  /** The flows to choose from, by id and name. */
  flows: readonly SelectOption[];
  /** The projects to choose from. Absent on the list of one project, which is already one project. */
  projects?: readonly SelectOption[] | undefined;
  onChange: (patch: Partial<ExecutionsSearch>) => void;
  onClear: () => void;
}

/**
 * The filters of the list of runs: the search by identifier, the status chips (All, Failed, Needs review), the date
 * range, the flow and the project. They live in the address, so this component only reports what changed. The search
 * takes a whole run identifier: anything else filters nothing and says so, because looking inside messages needs a
 * security decision first (SPEC-019 Q9). There is no "Live" chip: the list is not updated by events yet.
 */
export function ExecutionFilters({ search, flows, projects, onChange, onClear }: ExecutionFiltersProps) {
  const intl = useIntl();
  const hintId = useId();
  const [text, setText] = useState(search.q ?? '');
  const read = readSearchText(text);

  const changeText = (next: string) => {
    setText(next);
    const result = readSearchText(next);
    // Only a whole identifier (or an empty box) reaches the address; a half-typed one waits.
    if (result.kind === 'id') onChange({ q: result.id });
    else if (result.kind === 'empty') onChange({ q: undefined });
  };

  const chip = (show: ExecutionShow | undefined, label: string, icon?: typeof CircleX) => (
    <FilterChip
      label={label}
      icon={icon}
      pressed={search.show === show}
      onClick={() => {
        onChange({ show });
      }}
    />
  );

  const ranges: SelectOption[] = [
    { value: ANY, label: intl.formatMessage(runMessages.rangeAny) },
    { value: '24h', label: intl.formatMessage(runMessages.range24h) },
    { value: '7d', label: intl.formatMessage(runMessages.range7d) },
    { value: '30d', label: intl.formatMessage(runMessages.range30d) },
  ];
  const filtering = search.show !== undefined || search.range !== undefined || search.flowId !== undefined || search.projectId !== undefined || search.q !== undefined;

  return (
    <div role="search" aria-label={intl.formatMessage(runMessages.filters)} className="flex flex-col gap-3">
      <Input
        type="search"
        startIcon={Search}
        value={text}
        aria-label={intl.formatMessage(runMessages.search)}
        placeholder={intl.formatMessage(runMessages.search)}
        aria-describedby={read.kind === 'invalid' ? hintId : undefined}
        aria-invalid={read.kind === 'invalid' ? true : undefined}
        onChange={(event) => {
          changeText(event.target.value);
        }}
      />
      {read.kind === 'invalid' ? (
        <p id={hintId} className="text-fg-muted text-sm">
          {intl.formatMessage(runMessages.searchHint)}
        </p>
      ) : null}
      <div role="group" aria-label={intl.formatMessage(runMessages.stateLabel)} className="flex flex-wrap gap-2">
        {chip(undefined, intl.formatMessage(runMessages.stateAll))}
        {chip('failed', intl.formatMessage(runMessages.stateFailed), CircleX)}
        {chip('review', intl.formatMessage(runMessages.stateReview), ShieldQuestion)}
      </div>
      <div className="grid grid-cols-2 gap-2">
        <Select
          aria-label={intl.formatMessage(runMessages.range)}
          value={search.range ?? ANY}
          options={ranges}
          onValueChange={(value) => {
            onChange({ range: value === ANY ? undefined : (value as ExecutionRange) });
          }}
        />
        <Select
          aria-label={intl.formatMessage(runMessages.flowFilter)}
          value={search.flowId ?? ANY}
          options={[{ value: ANY, label: intl.formatMessage(runMessages.flowAny) }, ...flows]}
          onValueChange={(value) => {
            onChange({ flowId: value === ANY ? undefined : value });
          }}
        />
        {projects === undefined ? null : (
          <Select
            aria-label={intl.formatMessage(runMessages.projectFilter)}
            value={search.projectId ?? ANY}
            options={[{ value: ANY, label: intl.formatMessage(runMessages.projectAny) }, ...projects]}
            onValueChange={(value) => {
              onChange({ projectId: value === ANY ? undefined : value });
            }}
          />
        )}
        {filtering ? (
          <Button
            variant="ghost"
            className="col-span-2"
            onClick={() => {
              setText('');
              onClear();
            }}
          >
            {intl.formatMessage(runMessages.clear)}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
