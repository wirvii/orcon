// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { cn, ListRow, RelativeTime, StatusBadge } from '@orcon/ui';
import { useIntl } from 'react-intl';

import { runStatusKey, tailId } from '../model/format';
import type { Execution } from '../model/types';

import { runMessages } from './messages';

export interface ExecutionRowsProps {
  runs: readonly Execution[];
  /** The name the person knows the flow of a run by. */
  flowName: (flowId: string) => string;
  /** The address of a run: the row is a link to it. */
  hrefOf: (run: Execution) => string;
  /** Opens a run without reloading the page. */
  onOpen: (run: Execution) => void;
  /** The run that is open beside the list. */
  selectedId?: string | undefined;
}

/**
 * The runs as dense rows: the flow, the end of the identifier and when it started, then the status. A row is one link to the run. The status is the icon and the word; a run with an operation of
 * unknown outcome says "Needs review" and is never drawn as a failure or a success.
 */
export function ExecutionRows({ runs, flowName, hrefOf, onOpen, selectedId }: ExecutionRowsProps) {
  const intl = useIntl();
  return (
    <ul aria-label={intl.formatMessage(runMessages.listLabel)} className="border-border bg-surface overflow-hidden rounded-lg border">
      {runs.map((run) => {
        const selected = run.id === selectedId;
        return (
          <li key={run.id}>
            <ListRow
              href={hrefOf(run)}
              onNavigate={() => {
                onOpen(run);
              }}
              aria-current={selected ? 'true' : undefined}
              className={cn(selected && 'bg-surface-hover')}
              title={flowName(run.flowId)}
              meta={
                <>
                  {intl.formatMessage(runMessages.runNumber, { id: tailId(run.id) })} ·{' '}
                  {run.startedAt === undefined ? intl.formatMessage(runMessages.notStarted) : <RelativeTime value={run.startedAt} />}
                </>
              }
              trailing={<StatusBadge status={runStatusKey(run)} />}
            />
          </li>
        );
      })}
    </ul>
  );
}
