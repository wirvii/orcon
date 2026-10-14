// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, Input } from '@orcon/ui';
import { useMemo, useState, type SyntheticEvent } from 'react';
import { useIntl } from 'react-intl';

import { editorMessages } from './messages';

export interface FoundNode {
  id: string;
  name: string;
}

export interface FindNodeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  nodes: readonly FoundNode[];
  /** Called with the node to jump to; the dialog closes first. */
  onPick: (id: string) => void;
}

/** Finds a node of the flow by name and moves focus to it (SPEC-014 R-ED-CAN-07). Enter picks the first match. */
export function FindNodeDialog({ open, onOpenChange, nodes, onPick }: FindNodeDialogProps) {
  const intl = useIntl();
  const [query, setQuery] = useState('');
  const matches = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase();
    return needle === '' ? nodes : nodes.filter((node) => node.name.toLocaleLowerCase().includes(needle));
  }, [nodes, query]);

  const pick = (id: string) => {
    onOpenChange(false);
    setQuery('');
    onPick(id);
  };
  const submit = (event: SyntheticEvent) => {
    event.preventDefault();
    const first = matches[0];
    if (first) pick(first.id);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setQuery('');
        onOpenChange(next);
      }}
      title={intl.formatMessage(editorMessages.findTitle)}
    >
      <form onSubmit={submit} className="flex flex-col gap-3">
        <Input
          autoFocus
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
          }}
          aria-label={intl.formatMessage(editorMessages.findLabel)}
          placeholder={intl.formatMessage(editorMessages.findLabel)}
        />
        {matches.length === 0 ? (
          <p className="text-fg-muted">{intl.formatMessage(editorMessages.findEmpty)}</p>
        ) : (
          <ul aria-label={intl.formatMessage(editorMessages.findResults)} className="flex flex-col gap-1">
            {matches.map((node) => (
              <li key={node.id}>
                <Button
                  type="button"
                  variant="ghost"
                  className="w-full justify-start text-left"
                  onClick={() => {
                    pick(node.id);
                  }}
                >
                  {node.name}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </form>
    </Dialog>
  );
}
