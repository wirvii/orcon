// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { Tooltip } from '@orcon/ui';
import { ChevronRight } from 'lucide-react';
import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { defineMessages, useIntl } from 'react-intl';

import { PublishControls } from '../forms/PublishControls';
import { usePendingChanges } from '../forms/use-pending-changes';
import type { SaveStatus } from '../model/autosave';
import { useEditorStore, useEditorStoreApi } from '../store/editor-context';

import { SaveIndicator } from './SaveIndicator';

const messages = defineMessages({
  nav: { id: 'editor.top.nav', defaultMessage: 'Where you are', description: 'Accessible name of the breadcrumb above the flow name: the project the flow belongs to.' },
  sections: { id: 'editor.top.sections', defaultMessage: 'Flow sections', description: 'Accessible name of the row of tabs that switches between the editor, the runs and the versions of the flow.' },
  tabEditor: { id: 'editor.top.tab.editor', defaultMessage: 'Editor', description: 'Tab of the flow page: the canvas where the flow is edited.' },
  tabExecutions: { id: 'editor.top.tab.executions', defaultMessage: 'Executions', description: 'Tab of the flow page: the runs of this flow.' },
  tabVersions: { id: 'editor.top.tab.versions', defaultMessage: 'Versions', description: 'Tab of the flow page: the published versions of this flow.' },
  versionsSoon: { id: 'editor.top.tab.versionsSoon', defaultMessage: 'The list of versions is not available yet.', description: 'Why the Versions tab cannot be opened: its screen is not built yet.' },
  rename: { id: 'editor.top.rename', defaultMessage: 'Rename the flow', description: 'Accessible name of the button that edits the name of the flow.' },
  nameField: { id: 'editor.top.nameField', defaultMessage: 'Flow name', description: 'Accessible name of the field where the name of the flow is edited.' },
});

/** Where the bar sends the person: the host (the screen) knows the router, the bar only knows the two places. */
export interface EditorNav {
  /** The name of the project, for the breadcrumb. */
  projectLabel: string;
  /** Renders a link to `project` (the flows of the project) or `executions` (the runs of this flow). */
  link: (target: 'project' | 'executions', props: { className: string; children: ReactNode }) => ReactNode;
}

export interface EditorTopBarProps {
  /** The editing controls need a tablet or a computer; a phone shows the flow only. */
  editable: boolean;
  status: SaveStatus;
  /** The flow the Active switch and Publish act on. Absent when the flow is not stored on the server, so there is nothing to publish. */
  flowId?: string | undefined;
  nav?: EditorNav | undefined;
}

const CRUMB = 'text-fg-muted hover:text-fg max-w-canvas-label-width truncate text-sm';
const tabBase = 'inline-flex min-h-target items-center rounded-md px-3 text-sm font-medium';

/**
 * The top of the editor (SPEC-019 R-UX-EDT-01): the project and the name of the flow (which can be renamed), the
 * tabs Editor, Executions and Versions, the state of the draft in words, the Active switch and Publish with the
 * number of changes not published yet. Tags and Share are not shown: they have no backend yet.
 */
export function EditorTopBar({ editable, status, flowId, nav }: EditorTopBarProps) {
  const intl = useIntl();
  const api = useEditorStoreApi();
  const name = useEditorStore((state) => state.document.name);
  const pending = usePendingChanges(editable ? flowId : undefined);
  const [renaming, setRenaming] = useState(false);

  return (
    <header className="border-border flex flex-wrap items-center gap-x-4 gap-y-2 border-b px-4 py-2">
      <div className="md:min-w-canvas-label-width flex min-w-0 basis-full items-center gap-1 md:flex-1 md:basis-0">
        {nav ? (
          <nav aria-label={intl.formatMessage(messages.nav)} className="flex items-center gap-1">
            {nav.link('project', { className: CRUMB, children: nav.projectLabel })}
            <ChevronRight aria-hidden className="text-fg-subtle size-icon-16 shrink-0" />
          </nav>
        ) : null}
        <h1 className="text-md min-w-0 font-semibold">
          {renaming && editable ? (
            <NameField
              value={name}
              label={intl.formatMessage(messages.nameField)}
              onCommit={(next) => {
                api.getState().renameFlow(next);
                setRenaming(false);
              }}
              onCancel={() => {
                setRenaming(false);
              }}
            />
          ) : editable ? (
            <Tooltip content={intl.formatMessage(messages.rename)}>
              <button
                type="button"
                onClick={() => {
                  setRenaming(true);
                }}
                onKeyDown={(event) => {
                  if (event.key === 'F2') {
                    event.preventDefault();
                    setRenaming(true);
                  }
                }}
                className="hover:bg-surface-sunken block max-w-full truncate rounded-md px-1 text-start wrap-anywhere"
              >
                {name}
              </button>
            </Tooltip>
          ) : (
            <span className="wrap-anywhere">{name}</span>
          )}
        </h1>
      </div>

      <nav aria-label={intl.formatMessage(messages.sections)} className="bg-surface-sunken border-border flex shrink-0 items-center gap-1 rounded-lg border p-1">
        <span aria-current="page" className={`${tabBase} bg-surface text-fg shadow-sm`}>
          {intl.formatMessage(messages.tabEditor)}
        </span>
        {nav ? (
          nav.link('executions', { className: `${tabBase} text-fg-muted hover:text-fg`, children: intl.formatMessage(messages.tabExecutions) })
        ) : (
          <span aria-disabled="true" className={`${tabBase} text-fg-disabled`}>
            {intl.formatMessage(messages.tabExecutions)}
          </span>
        )}
        <Tooltip content={intl.formatMessage(messages.versionsSoon)}>
          <span aria-disabled="true" tabIndex={0} className={`${tabBase} text-fg-disabled cursor-not-allowed`}>
            {intl.formatMessage(messages.tabVersions)}
            <span className="sr-only">. {intl.formatMessage(messages.versionsSoon)}</span>
          </span>
        </Tooltip>
      </nav>

      <div className="flex shrink-0 items-center justify-end gap-4 md:basis-0 md:flex-1">
        {editable ? <SaveIndicator status={status} /> : null}
        {editable && flowId !== undefined ? <PublishControls flowId={flowId} flowName={name} pending={pending} /> : null}
      </div>
    </header>
  );
}

interface NameFieldProps {
  value: string;
  label: string;
  onCommit: (name: string) => void;
  onCancel: () => void;
}

/** The field that renames the flow in place. Enter or leaving the field saves; Escape cancels. */
function NameField({ value, label, onCommit, onCancel }: NameFieldProps) {
  const [draft, setDraft] = useState(value);
  const field = useRef<HTMLInputElement>(null);
  const done = useRef(false);
  useEffect(() => {
    field.current?.focus();
    field.current?.select();
  }, []);
  const finish = (commit: boolean) => {
    if (done.current) return;
    done.current = true;
    if (commit && draft.trim() !== '' && draft.trim() !== value) onCommit(draft.trim());
    else onCancel();
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    event.stopPropagation();
    if (event.key === 'Enter') {
      event.preventDefault();
      finish(true);
    } else if (event.key === 'Escape') {
      event.preventDefault();
      finish(false);
    }
  };
  return (
    <input
      ref={field}
      value={draft}
      aria-label={label}
      maxLength={256}
      onChange={(event) => {
        setDraft(event.target.value);
      }}
      onBlur={() => {
        finish(true);
      }}
      onKeyDown={onKeyDown}
      className="bg-surface border-control text-fg h-control w-canvas-label-width rounded-md border px-2 text-md font-semibold outline-hidden focus-visible:outline-solid focus-visible:outline-2 focus-visible:outline-focus"
    />
  );
}
