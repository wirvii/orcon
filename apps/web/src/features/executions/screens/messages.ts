// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages } from 'react-intl';

/** The words of the runs screens: how a run started, and the column and section names. */
export const runMessages = defineMessages({
  modeWebhook: { id: 'runs.mode.webhook', defaultMessage: 'Webhook', description: 'How a run started: an HTTP call to the flow webhook.' },
  modeTrigger: { id: 'runs.mode.trigger', defaultMessage: 'Trigger', description: 'How a run started: an automatic trigger of an active flow.' },
  modeTest: { id: 'runs.mode.test', defaultMessage: 'Test', description: 'How a run started: a test session.' },
  modeManual: { id: 'runs.mode.manual', defaultMessage: 'By hand', description: 'How a run started: a person ran the flow.' },
  modeChild: { id: 'runs.mode.child', defaultMessage: 'Sub-flow', description: 'How a run started: another flow called this one.' },
  modeError: { id: 'runs.mode.error', defaultMessage: 'Error handler', description: 'How a run started: the error handler of another flow.' },
  modeRetry: { id: 'runs.mode.retry', defaultMessage: 'Retry', description: 'How a run started: a person or the system retried an earlier run.' },
  modeEvaluation: { id: 'runs.mode.evaluation', defaultMessage: 'Evaluation', description: 'How a run started: an evaluation of the flow.' },
  started: { id: 'runs.column.started', defaultMessage: 'Started', description: 'Column heading: when the run started.' },
  duration: { id: 'runs.column.duration', defaultMessage: 'Duration', description: 'Column heading: how long the run took.' },
  trigger: { id: 'runs.column.trigger', defaultMessage: 'Started by', description: 'Column heading: how the run started.' },
  run: { id: 'runs.column.run', defaultMessage: 'Run', description: 'Column heading: the short identifier of the run, which opens it.' },
  deletedFlow: {
    id: 'runs.list.deletedFlow',
    defaultMessage: 'Deleted flow',
    description: 'Names the flow of a run when that flow is no longer in the project: it was deleted, and its runs stay in the history until they are purged.',
  },
  unknownFlow: { id: 'runs.list.unknownFlow', defaultMessage: 'Flow', description: 'Names the flow of a run while its name is not known.' },
  notStarted: { id: 'runs.notStarted', defaultMessage: 'Not started', description: 'Shown instead of a start time while the run is still queued.' },
  stillRunning: { id: 'runs.stillRunning', defaultMessage: 'Still running', description: 'Shown instead of a duration while the run has not finished.' },
  dataHidden: {
    id: 'runs.dataHidden',
    defaultMessage: 'You cannot see the data of this run. Ask an administrator of the project for access to run data.',
    description: 'Explains that the items of a run are hidden for this person, and whom to ask.',
  },
  unavailableTitle: {
    id: 'runs.unavailable.title',
    defaultMessage: 'Run history will be available soon',
    description: 'Title of the runs list while the server cannot list runs yet.',
  },
  unavailableText: {
    id: 'runs.unavailable.text',
    defaultMessage: 'Flows can be edited and published already; their runs will be listed here once the history is ready.',
    description: 'Explains that the runs list is not ready yet and that nothing the person did is lost.',
  },
  noRunsTitle: { id: 'runs.empty.title', defaultMessage: 'No runs yet', description: 'Title of the runs list when nothing has run.' },
  noRunsText: {
    id: 'runs.empty.text',
    defaultMessage: 'Runs appear here when a flow starts: by hand, from a trigger or from a webhook.',
    description: 'Explains where runs come from when the list is empty.',
  },
  title: { id: 'runs.list.title', defaultMessage: 'Runs', description: 'Main heading of the list of runs of a project.' },
  titleOfFlow: {
    id: 'runs.list.titleOfFlow',
    defaultMessage: 'Runs of one flow',
    description: 'Says the runs list shows only the runs of one flow.',
  },
  showAll: { id: 'runs.list.showAll', defaultMessage: 'Show every run', description: 'Link that removes the flow filter from the runs list.' },
  loadMore: { id: 'runs.list.loadMore', defaultMessage: 'Load more runs', description: 'Button that loads the next page of runs.' },
  nodes: { id: 'runs.detail.nodes', defaultMessage: 'Steps', description: 'Section heading: the node runs of this run, in the order they ran.' },
  noNodes: { id: 'runs.detail.noNodes', defaultMessage: 'No step has run yet.', description: 'Shown when the run has no step runs yet.' },
  attempts: { id: 'runs.detail.attempts', defaultMessage: 'Attempts: {count}', description: 'How many attempts a step made, retries included.' },
  input: { id: 'runs.detail.input', defaultMessage: 'Input of {node}, port {port}', description: 'Accessible name of the items a step received on a port.' },
  output: { id: 'runs.detail.output', defaultMessage: 'Output of {node}, port {port}', description: 'Accessible name of the items a step produced on a port.' },
  noItems: { id: 'runs.detail.noItems', defaultMessage: 'No items on this port.', description: 'Shown when a port carried no items.' },
  failedTitle: { id: 'runs.detail.failedTitle', defaultMessage: 'This run failed', description: 'Title of the banner that explains why a run failed.' },
  failedNode: { id: 'runs.detail.failedNode', defaultMessage: 'Step: {node}', description: 'Names the step that failed in the failure banner.' },
  version: { id: 'runs.detail.version', defaultMessage: 'Flow version {version}', description: 'Which version of the flow the run executed.' },
});
