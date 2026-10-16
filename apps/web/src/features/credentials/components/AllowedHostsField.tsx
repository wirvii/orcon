// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { FormField, IconButton, Input } from '@orcon/ui';
import { X } from 'lucide-react';
import { useState, type KeyboardEvent } from 'react';
import { useIntl } from 'react-intl';

import { hostMessages } from '../model/messages';
import { MAX_ALLOWED_HOSTS, parseAllowedHost, withHost } from '../model/hosts';

export interface AllowedHostsFieldProps {
  value: readonly string[];
  onChange: (hosts: readonly string[]) => void;
  /** An error from the form (none added yet, or the server refused the list). The field's own typing errors come first. */
  error?: string | undefined;
  /** Replaces the default hint under the label. */
  description?: string | undefined;
}

const SEPARATORS = /[\s,;]+/;

/**
 * The allowed domains of a credential: the only sites its secret may be sent to. A domain is added by typing it
 * and pressing Enter, a comma or leaving the box; a pasted address is cut down to its host. What is added shows
 * as a removable tag, in a list a screen reader names. The same rules as the server run here, so a mistake shows
 * next to the box instead of after sending.
 */
export function AllowedHostsField({ value, onChange, error, description }: AllowedHostsFieldProps) {
  const intl = useIntl();
  const [draft, setDraft] = useState('');
  const [problem, setProblem] = useState<string | undefined>(undefined);

  /** Adds every domain in `raw`; what is not a domain stays in the box with the reason. */
  const commit = (raw: string) => {
    const pieces = raw.split(SEPARATORS).filter((piece) => piece !== '');
    let hosts = value;
    const rejected: string[] = [];
    let reason: string | undefined;
    for (const piece of pieces) {
      const parsed = parseAllowedHost(piece);
      if (parsed.ok) {
        if (!hosts.includes(parsed.host) && hosts.length >= MAX_ALLOWED_HOSTS) {
          reason = intl.formatMessage(hostMessages.tooMany, {
            max: MAX_ALLOWED_HOSTS,
          });
          rejected.push(piece);
        } else {
          hosts = withHost(hosts, parsed.host);
        }
      } else {
        rejected.push(piece);
        reason = intl.formatMessage(parsed.reason === 'wildcard-address' ? hostMessages.wildcardAddress : hostMessages.invalid);
      }
    }
    if (hosts !== value) onChange(hosts);
    setDraft(rejected.join(' '));
    setProblem(reason);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' || event.key === ',') {
      // Enter adds the domain; it must not submit the form behind it.
      event.preventDefault();
      commit(draft);
    } else if (event.key === 'Backspace' && draft === '' && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  };

  return (
    <FormField name="/allowedHosts" label={intl.formatMessage(hostMessages.label)} description={description ?? intl.formatMessage(hostMessages.hint)} required error={problem ?? error}>
      <div className="flex flex-col gap-2">
        <Input
          className="font-mono"
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
          placeholder={intl.formatMessage(hostMessages.placeholder)}
          value={draft}
          onChange={(event) => {
            setDraft(event.target.value);
            if (problem !== undefined) setProblem(undefined);
          }}
          onKeyDown={onKeyDown}
          onBlur={() => {
            if (draft.trim() !== '') commit(draft);
          }}
        />
        {value.length > 0 ? (
          <ul aria-label={intl.formatMessage(hostMessages.listLabel)} className="flex flex-wrap gap-2">
            {value.map((host) => (
              <li key={host} className="bg-surface-sunken border-border flex items-center gap-1 rounded-md border pl-2 font-mono text-sm">
                <span className="wrap-anywhere">{host}</span>
                <IconButton
                  variant="ghost"
                  icon={X}
                  label={intl.formatMessage(hostMessages.remove, { host })}
                  onClick={() => {
                    onChange(value.filter((other) => other !== host));
                  }}
                />
              </li>
            ))}
          </ul>
        ) : null}
      </div>
    </FormField>
  );
}
