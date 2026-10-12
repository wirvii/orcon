// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, useIntl } from 'react-intl';

import { exampleCatalog, exampleDraft } from '../canvas/fixtures';
import type { CatalogItem, Graph } from '../canvas/model';
import { WorkflowCanvas, type NodeExecutionState } from '../canvas/WorkflowCanvas';
import { Toaster } from '../components/toast/Toast';

import { PageHeader } from './PageHeader';
import { ProductShell } from './ProductShell';

const messages = defineMessages({
  title: { id: 'screen.editor.title', defaultMessage: 'Order fulfilment', description: 'Name of the example flow shown in the flow editor template.' },
  description: {
    id: 'screen.editor.description',
    defaultMessage: 'Checks stock, charges the card and sends the receipt.',
    description: 'One line under the name of the example flow in the editor.',
  },
  flows: { id: 'screen.editor.breadcrumb.flows', defaultMessage: 'Flows', description: 'First step of the path above the flow editor: the list of flows.' },
  viewing: {
    id: 'screen.editor.viewing',
    defaultMessage: 'Showing the last execution',
    description: 'Note above a read-only flow that says the states shown come from the last execution.',
  },
});

export interface FlowEditorScreenProps {
  /** The flow to edit. */
  graph?: Graph;
  catalog?: readonly CatalogItem[];
  /** Execution overlay shown on top of the flow. */
  statuses?: Readonly<Record<string, NodeExecutionState>>;
  /** Show the flow without editing, as on a phone. Below md the canvas is always view only. */
  readOnly?: boolean;
  /** Link to the execution detail from a node. */
  executionHref?: (nodeId: string) => string;
}

/**
 * S3: the flow editor. Editing is for tablet and desktop; on a phone the same
 * canvas is view only: pan, zoom and tap a node for its state and counts. The
 * canvas fills the space under the page header.
 */
export function FlowEditorScreen({ graph = exampleDraft, catalog = exampleCatalog, statuses, readOnly = false, executionHref }: FlowEditorScreenProps) {
  const intl = useIntl();
  return (
    <ProductShell current="flows" fill>
      <PageHeader
        title={intl.formatMessage(messages.title)}
        description={statuses || readOnly ? intl.formatMessage(messages.viewing) : intl.formatMessage(messages.description)}
        breadcrumb={[{ label: intl.formatMessage(messages.flows), href: '#flows' }, { label: intl.formatMessage(messages.title) }]}
      />
      <div className="min-h-0 flex-1">
        <WorkflowCanvas defaultGraph={graph} catalog={catalog} statuses={statuses} readOnly={readOnly} executionHref={executionHref} />
      </div>
      <Toaster />
    </ProductShell>
  );
}
