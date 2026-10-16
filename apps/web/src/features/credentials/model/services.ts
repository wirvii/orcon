// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import { defineMessages, type MessageDescriptor } from 'react-intl';

/**
 * The services a credential can be created for, with the fields each one asks and the guide that goes with it.
 *
 * Today the one connector that applies a credential is the HTTP request step (`orcon.http@1`). It reads a
 * credential by the fields it holds: `token` (a bearer token), `username` with `password` (basic
 * authentication) or `apiKey` with an optional `headerName` (a key in a header). The three entries are those
 * three ways, so every credential created here is one a flow can use. A service of a connector appears here
 * when that connector ships its own credential fields; no entry is invented ahead of it.
 */

/** The HTTP request step: the connector every service below serves. */
export const HTTP_CONNECTOR = 'orcon.http@1';

export interface ServiceField {
  readonly name: string;
  /** A secret field is write-only: sent once, never read back, never kept by the page. */
  readonly secret: boolean;
  readonly required: boolean;
  readonly label: MessageDescriptor;
  readonly hint: MessageDescriptor;
  readonly default?: string;
  /** Shown in a monospace face: identifiers, header names. */
  readonly mono?: boolean;
}

export interface Service {
  readonly id: string;
  /** The connector reference stored in the credential (`type`). */
  readonly connector: string;
  readonly name: MessageDescriptor;
  readonly summary: MessageDescriptor;
  /** Words besides the name that find the service in the search. */
  readonly keywords: readonly string[];
  readonly fields: readonly ServiceField[];
  /** The steps of the guide, in order. */
  readonly steps: readonly MessageDescriptor[];
  /** What a person coming from n8n should know, in one or two sentences. */
  readonly fromN8n: MessageDescriptor;
  /** Why the allowed domains matter for this service, shown in the guide. */
  readonly protection: MessageDescriptor;
}

const text = defineMessages({
  headerName: {
    id: 'credentials.service.header.name',
    defaultMessage: 'API key in a header',
    description: 'Name of the service for APIs that ask for a key in a request header.',
  },
  headerSummary: {
    id: 'credentials.service.header.summary',
    defaultMessage: 'For any API that wants a key in a header, such as X-API-Key.',
    description: 'One line under the name of the API key in a header service.',
  },
  headerFieldName: {
    id: 'credentials.service.header.field.headerName',
    defaultMessage: 'Header name',
    description: 'Label of the field with the name of the header the key is sent in.',
  },
  headerFieldNameHint: {
    id: 'credentials.service.header.field.headerName.hint',
    defaultMessage: 'The header the API expects, for example X-API-Key or Authorization.',
    description: 'Hint under the header name field.',
  },
  headerFieldKey: {
    id: 'credentials.service.header.field.apiKey',
    defaultMessage: 'Key',
    description: 'Label of the field with the key sent in the header.',
  },
  headerFieldKeyHint: {
    id: 'credentials.service.header.field.apiKey.hint',
    defaultMessage: 'Paste the whole value the API expects, including a prefix such as Bearer if it asks for one.',
    description: 'Hint under the key field of the header service.',
  },
  headerStep1: {
    id: 'credentials.service.header.step1',
    defaultMessage: 'Open the dashboard of the service you want to call and find its API keys.',
    description: 'First step of the guide of the header key service.',
  },
  headerStep2: {
    id: 'credentials.service.header.step2',
    defaultMessage: 'Check its documentation for the header it expects, for example X-API-Key, and type that name here.',
    description: 'Second step of the guide of the header key service.',
  },
  headerStep3: {
    id: 'credentials.service.header.step3',
    defaultMessage: 'Create a key, copy it and paste it. The service may show it only once.',
    description: 'Third step of the guide of the header key service.',
  },
  headerStep4: {
    id: 'credentials.service.header.step4',
    defaultMessage: 'Add the domain of the API, for example api.example.com. If the API signs in through its own page (OAuth), look for its own credential in the list instead.',
    description: 'Fourth step of the guide of the header key service.',
  },
  headerN8n: {
    id: 'credentials.service.header.n8n',
    defaultMessage: 'The same as an n8n Header Auth credential: a header name and its value. If you imported the flow, create this one and choose it in the step.',
    description: 'Note for people who come from n8n, in the header key guide.',
  },
  headerProtection: {
    id: 'credentials.service.header.protection',
    defaultMessage: 'The list of domains stops a badly built flow from leaking your key to another site.',
    description: 'Why the allowed domains matter, in the header key guide.',
  },

  bearerName: {
    id: 'credentials.service.bearer.name',
    defaultMessage: 'Bearer token',
    description: 'Name of the service for APIs that ask for a bearer token.',
  },
  bearerSummary: {
    id: 'credentials.service.bearer.summary',
    defaultMessage: 'For APIs that ask for Authorization: Bearer and a token.',
    description: 'One line under the name of the bearer token service.',
  },
  bearerFieldToken: {
    id: 'credentials.service.bearer.field.token',
    defaultMessage: 'Token',
    description: 'Label of the field with the bearer token.',
  },
  bearerFieldTokenHint: {
    id: 'credentials.service.bearer.field.token.hint',
    defaultMessage: 'Only the token. Orcon adds the word Bearer when it sends it.',
    description: 'Hint under the token field of the bearer token service.',
  },
  bearerStep1: {
    id: 'credentials.service.bearer.step1',
    defaultMessage: 'Open the dashboard of the service you want to call and find its API tokens.',
    description: 'First step of the guide of the bearer token service.',
  },
  bearerStep2: {
    id: 'credentials.service.bearer.step2',
    defaultMessage: 'Create a token with only the permissions your flow needs.',
    description: 'Second step of the guide of the bearer token service.',
  },
  bearerStep3: {
    id: 'credentials.service.bearer.step3',
    defaultMessage: 'Copy it and paste it here. Many services show a token only once.',
    description: 'Third step of the guide of the bearer token service.',
  },
  bearerStep4: {
    id: 'credentials.service.bearer.step4',
    defaultMessage: 'Add the domain of the API, for example api.example.com.',
    description: 'Fourth step of the guide of the bearer token service.',
  },
  bearerN8n: {
    id: 'credentials.service.bearer.n8n',
    defaultMessage: 'The same as an n8n Bearer Auth credential. If you imported the flow, create this one and choose it in the step.',
    description: 'Note for people who come from n8n, in the bearer token guide.',
  },
  bearerProtection: {
    id: 'credentials.service.bearer.protection',
    defaultMessage: 'The token is sent only to the domains you list, even if a step points somewhere else.',
    description: 'Why the allowed domains matter, in the bearer token guide.',
  },

  basicName: {
    id: 'credentials.service.basic.name',
    defaultMessage: 'Username and password',
    description: 'Name of the service for APIs that ask for basic authentication.',
  },
  basicSummary: {
    id: 'credentials.service.basic.summary',
    defaultMessage: 'For APIs that ask for a user name and a password (basic authentication).',
    description: 'One line under the name of the basic authentication service.',
  },
  basicFieldUser: {
    id: 'credentials.service.basic.field.username',
    defaultMessage: 'Username',
    description: 'Label of the field with the user name.',
  },
  basicFieldUserHint: {
    id: 'credentials.service.basic.field.username.hint',
    defaultMessage: 'The user name the service expects.',
    description: 'Hint under the user name field.',
  },
  basicFieldPassword: {
    id: 'credentials.service.basic.field.password',
    defaultMessage: 'Password',
    description: 'Label of the field with the password.',
  },
  basicFieldPasswordHint: {
    id: 'credentials.service.basic.field.password.hint',
    defaultMessage: 'Some services give an API password that is not your account password.',
    description: 'Hint under the password field of the basic authentication service.',
  },
  basicStep1: {
    id: 'credentials.service.basic.step1',
    defaultMessage: 'Check the documentation of the service: it says whether it uses basic authentication.',
    description: 'First step of the guide of the basic authentication service.',
  },
  basicStep2: {
    id: 'credentials.service.basic.step2',
    defaultMessage: 'Use a user made for integrations, not your own account.',
    description: 'Second step of the guide of the basic authentication service.',
  },
  basicStep3: {
    id: 'credentials.service.basic.step3',
    defaultMessage: 'Type the user name and paste the password.',
    description: 'Third step of the guide of the basic authentication service.',
  },
  basicStep4: {
    id: 'credentials.service.basic.step4',
    defaultMessage: 'Add the domain of the API, for example api.example.com.',
    description: 'Fourth step of the guide of the basic authentication service.',
  },
  basicN8n: {
    id: 'credentials.service.basic.n8n',
    defaultMessage: 'The same as an n8n Basic Auth credential. If you imported the flow, create this one and choose it in the step.',
    description: 'Note for people who come from n8n, in the basic authentication guide.',
  },
  basicProtection: {
    id: 'credentials.service.basic.protection',
    defaultMessage: 'The password is sent only to the domains you list, even if a step points somewhere else.',
    description: 'Why the allowed domains matter, in the basic authentication guide.',
  },
});

export const SERVICES: readonly Service[] = [
  {
    id: 'header',
    connector: HTTP_CONNECTOR,
    name: text.headerName,
    summary: text.headerSummary,
    keywords: ['api key', 'x-api-key', 'authorization', 'header', 'http', 'key', 'clave', 'cabecera', 'encabezado'],
    fields: [
      {
        name: 'headerName',
        secret: false,
        required: true,
        default: 'X-API-Key',
        mono: true,
        label: text.headerFieldName,
        hint: text.headerFieldNameHint,
      },
      {
        name: 'apiKey',
        secret: true,
        required: true,
        label: text.headerFieldKey,
        hint: text.headerFieldKeyHint,
      },
    ],
    steps: [text.headerStep1, text.headerStep2, text.headerStep3, text.headerStep4],
    fromN8n: text.headerN8n,
    protection: text.headerProtection,
  },
  {
    id: 'bearer',
    connector: HTTP_CONNECTOR,
    name: text.bearerName,
    summary: text.bearerSummary,
    keywords: ['bearer', 'token', 'authorization', 'http', 'jwt'],
    fields: [
      {
        name: 'token',
        secret: true,
        required: true,
        label: text.bearerFieldToken,
        hint: text.bearerFieldTokenHint,
      },
    ],
    steps: [text.bearerStep1, text.bearerStep2, text.bearerStep3, text.bearerStep4],
    fromN8n: text.bearerN8n,
    protection: text.bearerProtection,
  },
  {
    id: 'basic',
    connector: HTTP_CONNECTOR,
    name: text.basicName,
    summary: text.basicSummary,
    keywords: ['basic', 'user', 'password', 'http', 'usuario', 'contraseña'],
    fields: [
      {
        name: 'username',
        secret: false,
        required: true,
        label: text.basicFieldUser,
        hint: text.basicFieldUserHint,
      },
      {
        name: 'password',
        secret: true,
        required: true,
        label: text.basicFieldPassword,
        hint: text.basicFieldPasswordHint,
      },
    ],
    steps: [text.basicStep1, text.basicStep2, text.basicStep3, text.basicStep4],
    fromN8n: text.basicN8n,
    protection: text.basicProtection,
  },
];

export function serviceById(id: string): Service | undefined {
  return SERVICES.find((service) => service.id === id);
}

/** The values a form for the service starts with: empty, except the fields with a default. */
export function defaultFieldValues(service: Service | undefined): Record<string, string> {
  return Object.fromEntries((service?.fields ?? []).map((field) => [field.name, field.default ?? '']));
}

/**
 * The service a stored credential was made for, read from the public fields it holds, because the API names only
 * the connector. A credential with a header name is a key in a header, one with a user name is basic authentication,
 * and one with neither holds a token. A connector this page has no entry for gives `undefined`.
 */
export function serviceOfCredential(credential: { readonly type: string; readonly publicFields?: Readonly<Record<string, string>> | undefined }): Service | undefined {
  if (credential.type !== HTTP_CONNECTOR) return undefined;
  const fields = credential.publicFields ?? {};
  if ('headerName' in fields) return serviceById('header');
  if ('username' in fields) return serviceById('basic');
  return serviceById('bearer');
}
