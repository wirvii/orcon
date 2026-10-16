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

/** The HTTP request step: the connector the three generic services below serve. */
export const HTTP_CONNECTOR = 'orcon.http@1';

/**
 * The WhatsApp Cloud API package. One credential serves its send node and its trigger: `token` is the bearer
 * token of the outgoing calls, `appSecret` checks the signature of every inbound request and `verifyToken`
 * answers the subscription handshake (the fields the package reads). The phone number ID is a parameter of the
 * node, not a field of the credential.
 */
export const WHATSAPP_CONNECTOR = 'orcon.whatsapp@1';

/** The AI model and AI agent steps: they read one field, `apiKey`, of the credential their model names. */
export const MODEL_CONNECTOR = 'orcon.ai.llm@1';

/** The node types whose catalog icon stands for a credential of the WhatsApp package. */
const WHATSAPP_ICON = 'orcon.whatsapp.whatsapp@1';

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
  /** The node type whose catalog icon the service shows; the connector itself when it is a node type. */
  readonly iconType?: string;
  /** The hosts the form starts with: where the secret of the service is meant to go. Empty when the person must say. */
  readonly defaultHosts: readonly string[];
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
  whatsappName: {
    id: 'credentials.service.whatsapp.name',
    defaultMessage: 'WhatsApp Cloud API',
    description: 'Name of the service for the WhatsApp Cloud API credential of Meta. WhatsApp is a product name.',
  },
  whatsappSummary: {
    id: 'credentials.service.whatsapp.summary',
    defaultMessage: 'To send WhatsApp messages and to start a flow when a customer writes.',
    description: 'One line under the name of the WhatsApp Cloud API service.',
  },
  whatsappFieldToken: {
    id: 'credentials.service.whatsapp.field.token',
    defaultMessage: 'Access token',
    description: 'Label of the field with the access token that sends WhatsApp messages.',
  },
  whatsappFieldTokenHint: {
    id: 'credentials.service.whatsapp.field.token.hint',
    defaultMessage: 'The token of a system user in Meta. Meta shows it once, so copy it when you generate it.',
    description: 'Hint under the access token field of the WhatsApp service.',
  },
  whatsappFieldSecret: {
    id: 'credentials.service.whatsapp.field.appSecret',
    defaultMessage: 'App secret',
    description: 'Label of the field with the app secret of the Meta app.',
  },
  whatsappFieldSecretHint: {
    id: 'credentials.service.whatsapp.field.appSecret.hint',
    defaultMessage: 'Checks that each incoming message really comes from Meta. Needed to start a flow from WhatsApp; leave it empty if you only send.',
    description: 'Hint under the app secret field of the WhatsApp service.',
  },
  whatsappFieldVerify: {
    id: 'credentials.service.whatsapp.field.verifyToken',
    defaultMessage: 'Verify token',
    description: 'Label of the field with the verify token of the webhook.',
  },
  whatsappFieldVerifyHint: {
    id: 'credentials.service.whatsapp.field.verifyToken.hint',
    defaultMessage: 'A text you invent, such as a long random phrase. You will type the same text in Meta when you register the webhook. Leave it empty if you only send.',
    description: 'Hint under the verify token field of the WhatsApp service.',
  },
  whatsappStep1: {
    id: 'credentials.service.whatsapp.step1',
    defaultMessage: 'Open developers.facebook.com, choose My Apps and create an app of the Business type, or open the one you have. Add the WhatsApp product to it.',
    description: 'First step of the guide of the WhatsApp service, in the developer console of Meta.',
  },
  whatsappStep2: {
    id: 'credentials.service.whatsapp.step2',
    defaultMessage: 'In WhatsApp, API Setup, copy the Phone number ID (digits only, not the phone number). It goes in the WhatsApp step of your flow, not here.',
    description: 'Second step of the guide of the WhatsApp service.',
  },
  whatsappStep3: {
    id: 'credentials.service.whatsapp.step3',
    defaultMessage:
      'Access token: the one on that page lasts 24 hours, so for real use open Meta Business Settings, Users, System users, create one and assign it your app and your WhatsApp account. Select Generate token with the whatsapp_business_messaging and whatsapp_business_management permissions, and paste it here.',
    description: 'Third step of the guide of the WhatsApp service: where to get the access token.',
  },
  whatsappStep4: {
    id: 'credentials.service.whatsapp.step4',
    defaultMessage: 'App secret: in your app, open App settings, Basic, select Show next to App secret and paste it here.',
    description: 'Fourth step of the guide of the WhatsApp service: where to get the app secret.',
  },
  whatsappStep5: {
    id: 'credentials.service.whatsapp.step5',
    defaultMessage:
      'Verify token: type any long text here. Then, in WhatsApp, Configuration, Webhook, paste the address of your flow as the callback URL and the same text as the verify token, and subscribe to the messages field.',
    description: 'Fifth step of the guide of the WhatsApp service: the verify token and the webhook.',
  },
  whatsappStep6: {
    id: 'credentials.service.whatsapp.step6',
    defaultMessage: 'The domain graph.facebook.com is already added: it is where Meta answers. Save, then choose this credential in the WhatsApp steps of your flow.',
    description: 'Sixth step of the guide of the WhatsApp service.',
  },
  whatsappN8n: {
    id: 'credentials.service.whatsapp.n8n',
    defaultMessage:
      'In n8n the WhatsApp credential asks for an access token and a business account ID, and the trigger uses another credential with a client ID and a client secret. Here one credential does both: the n8n access token goes in Access token and the client secret is the App secret. The business account ID is not needed, and the phone number ID goes in the step.',
    description: 'Note for people who come from n8n, in the WhatsApp guide.',
  },
  whatsappProtection: {
    id: 'credentials.service.whatsapp.protection',
    defaultMessage: 'The access token is sent only to Meta, even if a step points somewhere else.',
    description: 'Why the allowed domains matter, in the WhatsApp guide.',
  },

  modelName: {
    id: 'credentials.service.model.name',
    defaultMessage: 'AI model API key',
    description: 'Name of the service for the API key of an AI model provider.',
  },
  modelSummary: {
    id: 'credentials.service.model.summary',
    defaultMessage: 'For the AI model and AI agent steps: the key of the company that runs the model.',
    description: 'One line under the name of the AI model key service.',
  },
  modelFieldKey: {
    id: 'credentials.service.model.field.apiKey',
    defaultMessage: 'API key',
    description: 'Label of the field with the API key of the model provider.',
  },
  modelFieldKeyHint: {
    id: 'credentials.service.model.field.apiKey.hint',
    defaultMessage: 'The secret key from the dashboard of the provider. Only the key, without the word Bearer.',
    description: 'Hint under the API key field of the AI model service.',
  },
  modelStep1: {
    id: 'credentials.service.model.step1',
    defaultMessage: 'Open the dashboard of the company that runs the model you want to use and find its API keys page.',
    description: 'First step of the guide of the AI model key service.',
  },
  modelStep2: {
    id: 'credentials.service.model.step2',
    defaultMessage: 'Create a key just for Orcon, and set a spending limit on it if the provider offers one.',
    description: 'Second step of the guide of the AI model key service.',
  },
  modelStep3: {
    id: 'credentials.service.model.step3',
    defaultMessage: 'Copy the key and paste it here. Most providers show it only once.',
    description: 'Third step of the guide of the AI model key service.',
  },
  modelStep4: {
    id: 'credentials.service.model.step4',
    defaultMessage: 'Add the domain of its API: the host in the provider documentation, such as api.example.com. The key is sent only there, and a step whose model runs on another host cannot use it.',
    description: 'Fourth step of the guide of the AI model key service.',
  },
  modelN8n: {
    id: 'credentials.service.model.n8n',
    defaultMessage: 'The same as the API key credential of an n8n model node. If you imported a flow, create this one and choose it in the AI model or AI agent step.',
    description: 'Note for people who come from n8n, in the AI model key guide.',
  },
  modelProtection: {
    id: 'credentials.service.model.protection',
    defaultMessage: 'The key is sent only to the domains you list, so a mistake in a step cannot send it to another company.',
    description: 'Why the allowed domains matter, in the AI model key guide.',
  },
});

export const SERVICES: readonly Service[] = [
  {
    id: 'whatsapp',
    connector: WHATSAPP_CONNECTOR,
    iconType: WHATSAPP_ICON,
    defaultHosts: ['graph.facebook.com'],
    name: text.whatsappName,
    summary: text.whatsappSummary,
    keywords: ['whatsapp', 'meta', 'facebook', 'cloud api', 'mensajes', 'messages', 'chat', 'ventas', 'sales'],
    fields: [
      { name: 'token', secret: true, required: true, label: text.whatsappFieldToken, hint: text.whatsappFieldTokenHint },
      { name: 'appSecret', secret: true, required: false, label: text.whatsappFieldSecret, hint: text.whatsappFieldSecretHint },
      { name: 'verifyToken', secret: true, required: false, label: text.whatsappFieldVerify, hint: text.whatsappFieldVerifyHint },
    ],
    steps: [text.whatsappStep1, text.whatsappStep2, text.whatsappStep3, text.whatsappStep4, text.whatsappStep5, text.whatsappStep6],
    fromN8n: text.whatsappN8n,
    protection: text.whatsappProtection,
  },
  {
    id: 'model',
    connector: MODEL_CONNECTOR,
    defaultHosts: [],
    name: text.modelName,
    summary: text.modelSummary,
    keywords: ['ai', 'ia', 'model', 'modelo', 'llm', 'agent', 'agente', 'api key', 'clave', 'inteligencia artificial'],
    fields: [{ name: 'apiKey', secret: true, required: true, label: text.modelFieldKey, hint: text.modelFieldKeyHint }],
    steps: [text.modelStep1, text.modelStep2, text.modelStep3, text.modelStep4],
    fromN8n: text.modelN8n,
    protection: text.modelProtection,
  },
  {
    id: 'header',
    connector: HTTP_CONNECTOR,
    defaultHosts: [],
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
    defaultHosts: [],
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
    defaultHosts: [],
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

/**
 * The service a connector reference stands for on its own: WhatsApp and the AI model key each have a connector of
 * their own, while the three HTTP services share one, so the connector alone does not say which of them it is.
 */
export function serviceOfConnector(connector: string): Service | undefined {
  const matches = SERVICES.filter((service) => service.connector === connector);
  return matches.length === 1 ? matches[0] : undefined;
}

/** The brand icon key of a service: the node type whose icon the catalog carries. */
export function iconTypeOf(service: Pick<Service, 'connector' | 'iconType'>): string {
  return service.iconType ?? service.connector;
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
  if (credential.type === WHATSAPP_CONNECTOR) return serviceById('whatsapp');
  if (credential.type === MODEL_CONNECTOR) return serviceById('model');
  if (credential.type !== HTTP_CONNECTOR) return undefined;
  const fields = credential.publicFields ?? {};
  if ('headerName' in fields) return serviceById('header');
  if ('username' in fields) return serviceById('basic');
  return serviceById('bearer');
}
