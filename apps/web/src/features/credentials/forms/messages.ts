// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages } from 'react-intl';

/** The texts of the credentials screens and dialogs. Values of secrets are never among them. */
export const credentialMessages = defineMessages({
  title: { id: 'credentials.title', defaultMessage: 'Credentials', description: 'Main heading of the list of the project credentials.' },
  description: {
    id: 'credentials.description',
    defaultMessage: 'Secrets that flows use to reach other services. Nobody can read a value back, not even an owner: replace it instead.',
    description: 'One line under the heading of the credentials list: what a credential is and that its values cannot be read.',
  },
  previewNotice: {
    id: 'credentials.previewNotice',
    defaultMessage: 'Preview: credentials are kept in this browser session only. The server does not store them yet.',
    description: 'Banner on the credentials list while the credentials are kept in the browser, not on the server.',
  },
  newCredential: { id: 'credentials.new', defaultMessage: 'New credential', description: 'Button that opens the dialog to create a credential.' },
  emptyTitle: { id: 'credentials.empty.title', defaultMessage: 'No credentials yet', description: 'Title of the credentials list when the project has none.' },
  emptyDescription: {
    id: 'credentials.empty.description',
    defaultMessage: 'Create one for each service a flow calls, then choose it in the node.',
    description: 'Explains the empty credentials list and the next step.',
  },
  unavailableTitle: {
    id: 'credentials.unavailable.title',
    defaultMessage: 'Credentials will be available soon',
    description: 'Title of the credentials list while the server cannot list credentials yet.',
  },
  unavailableText: {
    id: 'credentials.unavailable.text',
    defaultMessage: 'Nothing you did is lost. The credentials of this project will be listed here once the server is ready.',
    description: 'Explains that the credentials list is not ready yet and that nothing the person did is lost.',
  },
  secretSet: { id: 'credentials.secretSet', defaultMessage: 'Secret set', description: 'Shown next to a credential whose secret is stored. The value itself is never shown.' },
  changed: { id: 'credentials.changed', defaultMessage: 'Changed', description: 'Label before the time a credential last changed.' },
  replace: { id: 'credentials.replace', defaultMessage: 'Replace secret', description: 'Button that opens the dialog to replace the secret values of a credential.' },
  remove: { id: 'credentials.delete', defaultMessage: 'Delete', description: 'Button that asks to delete a credential.' },
  createTitle: { id: 'credentials.create.title', defaultMessage: 'New credential', description: 'Title of the dialog that creates a credential.' },
  createText: {
    id: 'credentials.create.text',
    defaultMessage: 'The values are stored encrypted. You will not see them again after you save.',
    description: 'Explains that the secret values of a new credential are write-only.',
  },
  fieldName: { id: 'credentials.field.name', defaultMessage: 'Name', description: 'Label of the field with the name of a credential.' },
  fieldNameHint: {
    id: 'credentials.field.nameHint',
    defaultMessage: 'How flows and nodes refer to it, for example Support line.',
    description: 'Hint under the name of a credential.',
  },
  fieldType: { id: 'credentials.field.type', defaultMessage: 'Type', description: 'Label of the field that chooses the credential type.' },
  save: { id: 'credentials.save', defaultMessage: 'Create credential', description: 'Button that creates the credential.' },
  cancel: { id: 'credentials.cancel', defaultMessage: 'Cancel', description: 'Button that closes a credential dialog without changes.' },
  nameTaken: { id: 'credentials.nameTaken', defaultMessage: 'A credential with this name already exists in the project.', description: 'Error under the name when it is taken.' },
  failed: { id: 'credentials.failed', defaultMessage: 'The credential could not be saved. Try again.', description: 'Banner when creating or replacing a credential fails.' },
  deleteFailed: { id: 'credentials.deleteFailed', defaultMessage: 'The credential could not be deleted. Try again.', description: 'Banner when deleting a credential fails.' },
  rotateTitle: {
    id: 'credentials.rotate.title',
    defaultMessage: 'Replace the secret of {name}',
    description: 'Title of the dialog that replaces the secret values. {name} is the credential name.',
  },
  rotateText: {
    id: 'credentials.rotate.text',
    defaultMessage: 'The new values become the current version. Runs that start after you save use them.',
    description: 'Explains what replacing a secret does to the runs that follow.',
  },
  rotateSave: { id: 'credentials.rotate.save', defaultMessage: 'Replace secret', description: 'Button that replaces the secret values.' },
  deleteTitle: { id: 'credentials.delete.title', defaultMessage: 'Delete {name}?', description: 'Question before deleting a credential. {name} is its name.' },
  deleteImpact: {
    id: 'credentials.delete.impact',
    defaultMessage: 'Flows that use this credential fail at their next step until another one is chosen. This cannot be undone.',
    description: 'What deleting a credential does, in plain words, and that it cannot be undone.',
  },
  deleteConfirm: { id: 'credentials.delete.confirm', defaultMessage: 'Delete credential', description: 'Button that confirms the deletion of a credential.' },
});
