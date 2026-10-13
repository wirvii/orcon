// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages } from 'react-intl';

/** Sample content of the node details view in stories and in the editor template. */
export const sampleMessages = defineMessages({
  credential: { id: 'editor.sample.credential', defaultMessage: 'Credential', description: 'Label of the field that picks the stored credential a node uses.' },
  credentialValue: { id: 'editor.sample.credentialValue', defaultMessage: 'Team workspace', description: 'Name of the sample credential in the node settings.' },
  operation: { id: 'editor.sample.operation', defaultMessage: 'Operation', description: 'Label of the field that picks what the node does.' },
  operationSend: { id: 'editor.sample.operationSend', defaultMessage: 'Send message', description: 'A sample operation of a node: send a message.' },
  operationRead: { id: 'editor.sample.operationRead', defaultMessage: 'Read messages', description: 'A sample operation of a node: read messages.' },
  channel: { id: 'editor.sample.channel', defaultMessage: 'Channel', description: 'Label of the field that names the channel a message is sent to.' },
  channelHint: { id: 'editor.sample.channelHint', defaultMessage: 'Pick from the list or type a channel ID.', description: 'Hint under the channel field.' },
  message: { id: 'editor.sample.message', defaultMessage: 'Message text', description: 'Label of the field with the text of the message.' },
  messageValue: {
    id: 'editor.sample.messageValue',
    defaultMessage: 'New order {open}$json.id{close} from {open}$json.customer{close}',
    description: 'Sample expression in the message field. {open} and {close} are the braces that open and close an expression; they are filled in by the code and must stay.',
  },
  fixed: { id: 'editor.sample.fixed', defaultMessage: 'Fixed', description: 'Option that makes a field hold a plain value.' },
  expression: { id: 'editor.sample.expression', defaultMessage: 'Expression', description: 'Option that makes a field hold an expression computed from the data.' },
  valueKind: { id: 'editor.sample.valueKind', defaultMessage: 'Value type of {field}', description: 'Accessible name of the Fixed and Expression switch of a field.' },
  retryOnFail: { id: 'editor.sample.retry', defaultMessage: 'Retry when it fails', description: 'Label of the switch that makes a node try again after a failure.' },
  alwaysOutput: { id: 'editor.sample.alwaysOutput', defaultMessage: 'Always produce an output', description: 'Label of the switch that makes a node produce an output even with no items.' },
  notes: { id: 'editor.sample.notes', defaultMessage: 'Notes', description: 'Label of the field with free notes about a node.' },
  docsTitle: { id: 'editor.sample.docsTitle', defaultMessage: 'About this node', description: 'Heading of the documentation tab of a node.' },
  docsText: {
    id: 'editor.sample.docsText',
    defaultMessage: 'Sends a message to a channel. The message can use data from the previous steps through expressions.',
    description: 'Sample documentation text of a node.',
  },
  agentModel: { id: 'editor.sample.agentModel', defaultMessage: 'Language model', description: 'Label of the field that picks the language model of an agent.' },
  agentMemory: { id: 'editor.sample.agentMemory', defaultMessage: 'Window memory', description: 'Sample memory of an agent: it remembers the last messages.' },
  alertFailedTitle: { id: 'editor.sample.alert.failedTitle', defaultMessage: 'The step failed', description: 'Title of the alert that heads the settings of a node that failed.' },
  alertFailedText: {
    id: 'editor.sample.alert.failedText',
    defaultMessage: 'The service answered 401 Unauthorized: the credential has expired.',
    description: 'Text of the alert of a failed node: what the service answered and why.',
  },
  alertFailedReconnect: { id: 'editor.sample.alert.reconnect', defaultMessage: 'Reconnect credential', description: 'Button in the failed alert that renews the credential.' },
  alertFailedRun: { id: 'editor.sample.alert.runAgain', defaultMessage: 'Run again', description: 'Button in the failed alert that runs the step again.' },
  alertUncertainTitle: { id: 'editor.sample.alert.uncertainTitle', defaultMessage: 'Outcome unknown', description: 'Title of the alert that heads the settings of a node whose result is unknown.' },
  alertUncertainText: {
    id: 'editor.sample.alert.uncertainText',
    defaultMessage: 'The request timed out after the service may have accepted it. Check the channel before running anything again.',
    description: 'Text of the alert of an uncertain node: it may or may not have been applied, and what to do first.',
  },
  alertUncertainSent: { id: 'editor.sample.alert.markSent', defaultMessage: 'Mark as sent', description: 'Button in the uncertain alert that records that the operation was applied.' },
  alertUncertainRun: { id: 'editor.sample.alert.runAnyway', defaultMessage: 'Run again', description: 'Secondary button in the uncertain alert that runs the step again after checking.' },
  topbarFlow: { id: 'editor.frame.flowName', defaultMessage: 'Order intake', description: 'Name of the sample flow in the editor top bar.' },
  topbarLastRun: { id: 'editor.frame.lastRun', defaultMessage: 'Last run: {status}', description: 'Chip in the editor top bar with the state of the last run. {status} is the state label.' },
  topbarSave: { id: 'editor.frame.save', defaultMessage: 'Save', description: 'Button that saves the flow.' },
  topbarExecute: { id: 'editor.frame.execute', defaultMessage: 'Execute workflow', description: 'Button that runs the whole flow.' },
  topbarSaved: { id: 'editor.frame.saved', defaultMessage: 'Saved', description: 'Short confirmation after the flow was saved.' },
});
