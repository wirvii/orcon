// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages } from 'react-intl';

/** The texts of the credentials screen and its dialogs. Values of secrets are never among them. */
export const credentialMessages = defineMessages({
  title: {
    id: 'credentials.title',
    defaultMessage: 'Credentials',
    description: 'Main heading of the list of the project credentials.',
  },
  description: {
    id: 'credentials.description',
    defaultMessage: 'The accounts your flows use to connect. Keys are stored encrypted and nobody can read them back, not even an owner: replace one instead.',
    description: 'One line under the heading of the credentials list: what a credential is and that its values cannot be read.',
  },
  newCredential: {
    id: 'credentials.new',
    defaultMessage: 'Create credential',
    description: 'Button that opens the dialog to create a credential.',
  },
  listLabel: {
    id: 'credentials.list.label',
    defaultMessage: 'Credentials of the project',
    description: 'Accessible name of the list of the credentials of a project.',
  },
  emptyTitle: {
    id: 'credentials.empty.title',
    defaultMessage: 'No credentials yet',
    description: 'Title of the credentials list when the project has none.',
  },
  emptyDescription: {
    id: 'credentials.empty.description',
    defaultMessage: 'Create one for each service a flow calls, then choose it in the step.',
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
  connectorHttp: {
    id: 'credentials.connector.http',
    defaultMessage: 'HTTP request',
    description: 'Name of the HTTP request connector, the service a credential is for, in the credentials list.',
  },
  secretSet: {
    id: 'credentials.secretSet',
    defaultMessage: 'Secret set',
    description: 'Shown next to a credential whose secret is stored. The value itself is never shown.',
  },
  statusNeedsSecret: {
    id: 'credentials.status.needsSecret',
    defaultMessage: 'Needs a secret',
    description: 'State of a credential that has no secret value yet.',
  },
  statusNeedsReauth: {
    id: 'credentials.status.needsReauth',
    defaultMessage: 'Needs to be reconnected',
    description: 'State of a credential whose grant must be given again.',
  },
  statusDisabled: {
    id: 'credentials.status.disabled',
    defaultMessage: 'Switched off',
    description: 'State of a credential its owner switched off.',
  },
  shared: {
    id: 'credentials.shared',
    defaultMessage: 'Shared with this project',
    description: 'Tag on a credential that belongs to another project and is shared with this one.',
  },
  sharedHint: {
    id: 'credentials.sharedHint',
    defaultMessage: 'Only the project that owns it can change it.',
    description: 'Says why a shared credential has no actions in this project.',
  },
  changed: {
    id: 'credentials.changed',
    defaultMessage: 'Changed',
    description: 'Label before the time a credential last changed.',
  },
  domains: {
    id: 'credentials.domains',
    defaultMessage: 'Sends only to',
    description: 'Label before the sites a credential may be sent to.',
  },
  domainsMore: {
    id: 'credentials.domains.more',
    defaultMessage: '+{count} more',
    description: 'After the first allowed site of a credential: how many more there are. {count} is a number.',
  },
  actions: {
    id: 'credentials.actions',
    defaultMessage: 'Actions for {name}',
    description: 'Accessible name of the menu button of a credential. {name} is its name.',
  },
  replace: {
    id: 'credentials.replace',
    defaultMessage: 'Replace secret',
    description: 'Menu item that opens the dialog to replace the secret values of a credential.',
  },
  editDomains: {
    id: 'credentials.editDomains',
    defaultMessage: 'Change allowed domains',
    description: 'Menu item that opens the dialog to change the sites a credential may be sent to.',
  },
  remove: {
    id: 'credentials.delete',
    defaultMessage: 'Delete',
    description: 'Menu item that asks to delete a credential.',
  },
  cancel: {
    id: 'credentials.cancel',
    defaultMessage: 'Cancel',
    description: 'Button that closes a credential dialog without changes.',
  },
  deleteFailed: {
    id: 'credentials.deleteFailed',
    defaultMessage: 'The credential could not be deleted. Try again.',
    description: 'Banner when deleting a credential fails.',
  },
  deleteTitle: {
    id: 'credentials.delete.title',
    defaultMessage: 'Delete {name}?',
    description: 'Question before deleting a credential. {name} is its name.',
  },
  deleteImpact: {
    id: 'credentials.delete.impact',
    defaultMessage: 'Flows that use this credential fail at their next step until another one is chosen. This cannot be undone.',
    description: 'What deleting a credential does, in plain words, and that it cannot be undone.',
  },
  deleteConfirm: {
    id: 'credentials.delete.confirm',
    defaultMessage: 'Delete credential',
    description: 'Button that confirms the deletion of a credential.',
  },
});

/** The create dialog: three columns (service, form, guide). */
export const createMessages = defineMessages({
  title: {
    id: 'credentials.create.title',
    defaultMessage: 'Create credential',
    description: 'Title of the dialog that creates a credential.',
  },
  text: {
    id: 'credentials.create.text',
    defaultMessage: 'Keys are stored encrypted and you will not see them again after you save.',
    description: 'Explains that the secret values of a new credential are write-only.',
  },
  where: {
    id: 'credentials.create.where',
    defaultMessage: 'It is saved in {project}.',
    description: 'Says which project a new credential is saved in. {project} is the project name.',
  },
  servicesHeading: {
    id: 'credentials.create.services',
    defaultMessage: 'Service',
    description: 'Heading of the left column of the create dialog: the services a credential can be for.',
  },
  search: {
    id: 'credentials.create.search',
    defaultMessage: 'Search service',
    description: 'Label and placeholder of the search box over the services.',
  },
  noService: {
    id: 'credentials.create.noService',
    defaultMessage: 'No service matches “{query}”.',
    description: 'Shown when the search matches no service. {query} is the text typed.',
  },
  formHeading: {
    id: 'credentials.create.form',
    defaultMessage: 'Details',
    description: 'Accessible name of the middle column of the create dialog: the form.',
  },
  guideHeading: {
    id: 'credentials.create.guide',
    defaultMessage: 'How to get these details',
    description: 'Heading of the right column of the create dialog: the step-by-step guide.',
  },
  fieldName: {
    id: 'credentials.field.name',
    defaultMessage: 'Credential name',
    description: 'Label of the field with the name of a credential.',
  },
  fieldNameHint: {
    id: 'credentials.field.nameHint',
    defaultMessage: 'How you will see it when you choose it in a step, for example Support line.',
    description: 'Hint under the name of a credential.',
  },
  encrypted: {
    id: 'credentials.field.encrypted',
    defaultMessage: 'Encrypted',
    description: 'Tag next to a field whose value is stored encrypted and never shown again.',
  },
  save: {
    id: 'credentials.save',
    defaultMessage: 'Create credential',
    description: 'Button that creates the credential.',
  },
  nameTaken: {
    id: 'credentials.nameTaken',
    defaultMessage: 'A credential with this name already exists in the project.',
    description: 'Error under the name when it is taken.',
  },
  failed: {
    id: 'credentials.failed',
    defaultMessage: 'The credential could not be saved. Try again.',
    description: 'Banner when creating or replacing a credential fails.',
  },
  testConnection: {
    id: 'credentials.create.test',
    defaultMessage: 'Test connection',
    description: 'Button that checks a credential works before it is saved.',
  },
  fromN8nTitle: {
    id: 'credentials.create.fromN8n',
    defaultMessage: 'Coming from n8n?',
    description: 'Bold start of the note for people who come from n8n, in the connection guide.',
  },
  protectionTitle: {
    id: 'credentials.create.protection',
    defaultMessage: 'Protection',
    description: 'Bold start of the note about the allowed domains, in the connection guide.',
  },
  noServices: {
    id: 'credentials.create.noServices',
    defaultMessage: 'More services appear here as their connectors are installed.',
    description: 'Note under the list of services: the list grows with the installed connectors.',
  },
});

/** The allowed domains field and its two uses: when creating and when changing. */
export const hostMessages = defineMessages({
  label: {
    id: 'credentials.hosts.label',
    defaultMessage: 'Send only to these domains',
    description: 'Label of the field with the sites a credential may be sent to.',
  },
  hint: {
    id: 'credentials.hosts.hint',
    defaultMessage: 'Orcon never sends this key to another site, even if a step asks it to. Type a domain and press Enter. A leading *. covers one more level, for example *.example.com.',
    description: 'Explains the allowed domains field: the key only goes to these sites, how to add one and what a wildcard covers.',
  },
  placeholder: {
    id: 'credentials.hosts.placeholder',
    defaultMessage: 'api.example.com',
    description: 'Example shown inside the empty field that adds an allowed domain.',
  },
  remove: {
    id: 'credentials.hosts.remove',
    defaultMessage: 'Remove {host}',
    description: 'Accessible name of the button that removes an allowed domain. {host} is the domain.',
  },
  listLabel: {
    id: 'credentials.hosts.list',
    defaultMessage: 'Allowed domains',
    description: 'Accessible name of the list of the domains already added.',
  },
  required: {
    id: 'credentials.hosts.required',
    defaultMessage: 'Add at least one domain. Without one, the key would have nowhere to go.',
    description: 'Error when no allowed domain was added.',
  },
  invalid: {
    id: 'credentials.hosts.invalid',
    defaultMessage: 'That is not a domain. Use a name such as api.example.com, without a path.',
    description: 'Error when what was typed as an allowed domain is not a domain name.',
  },
  wildcardAddress: {
    id: 'credentials.hosts.wildcardAddress',
    defaultMessage: 'A wildcard cannot cover an IP address. Add the exact address instead.',
    description: 'Error when a wildcard was written over an IP address.',
  },
  tooMany: {
    id: 'credentials.hosts.tooMany',
    defaultMessage: 'A credential holds at most {max} domains.',
    description: 'Error when more allowed domains than the limit were added. {max} is a number.',
  },
  editTitle: {
    id: 'credentials.hosts.edit.title',
    defaultMessage: 'Allowed domains of {name}',
    description: 'Title of the dialog that changes the allowed domains. {name} is the credential name.',
  },
  editText: {
    id: 'credentials.hosts.edit.text',
    defaultMessage: 'The key goes only to these domains. Runs that start after you save use the new list.',
    description: 'Explains what changing the allowed domains does to the runs that follow.',
  },
  editSave: {
    id: 'credentials.hosts.edit.save',
    defaultMessage: 'Save domains',
    description: 'Button that saves the allowed domains.',
  },
});

/** The dialogs that change an existing credential. */
export const changeMessages = defineMessages({
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
  rotateSave: {
    id: 'credentials.rotate.save',
    defaultMessage: 'Replace secret',
    description: 'Button that replaces the secret values.',
  },
  failed: {
    id: 'credentials.change.failed',
    defaultMessage: 'The change could not be saved. Try again.',
    description: 'Banner when changing a credential fails.',
  },
  stale: {
    id: 'credentials.change.stale',
    defaultMessage: 'Someone else changed this credential while you were editing it. Close this window and open it again to see the current version.',
    description: 'Banner when a change is refused because the credential changed in the meantime.',
  },
  loadFailed: {
    id: 'credentials.change.loadFailed',
    defaultMessage: 'The credential could not be loaded. Close this window and try again.',
    description: 'Banner when the credential a dialog is about to change cannot be read.',
  },
  loading: {
    id: 'credentials.change.loading',
    defaultMessage: 'Loading the credential',
    description: 'Accessible name of the placeholder shown while the credential a dialog changes is read.',
  },
  unknownService: {
    id: 'credentials.change.unknownService',
    defaultMessage: 'This credential belongs to a service this page cannot edit yet.',
    description: 'Shown in the dialog that replaces a secret when the credential is for a service the page has no form for.',
  },
  replacesAll: {
    id: 'credentials.change.replacesAll',
    defaultMessage: 'The new version replaces all the secret values of this credential. Type again the ones you want to keep.',
    description: 'Hint in the dialog that replaces the secret of a credential that holds several secret values.',
  },
  currentValueHidden: {
    id: 'credentials.change.hidden',
    defaultMessage: 'The current value is never shown. Type the new one.',
    description: 'Hint in the dialog that replaces a secret: the stored value cannot be read.',
  },
});
