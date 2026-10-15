// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The credential types the app can create, with the fields their forms are built from (SPEC-003
 * `credentials[]`: `fields[]` with `sensitive` and `required`; SPEC-004 R-CRED-01). A secret field is
 * write-only: never prefilled, never shown again, never kept by the app (SPEC-010 FM-8). Labels come
 * from the catalog of each type, in the same shape a connector package ships (`i18n/{en,es}.json`).
 * Until the credential-type view of the API exists this table is the source.
 */

export type CatalogLocale = 'en' | 'es';

export interface CredentialField {
  readonly name: string;
  readonly secret: boolean;
  readonly required: boolean;
  readonly default?: string;
}

export interface CredentialType {
  readonly id: string;
  readonly fields: readonly CredentialField[];
  readonly catalog: Readonly<Record<CatalogLocale, Readonly<Record<string, string>>>>;
}

/** The locale of a language tag: Spanish when the tag starts with `es`, English otherwise. */
export const catalogLocaleOf = (tag: string): CatalogLocale => (tag.toLowerCase().startsWith('es') ? 'es' : 'en');

/** A text of a type's catalog in the person's language; the key itself when the catalog lacks it. */
export function credentialText(type: Pick<CredentialType, 'catalog'>, locale: CatalogLocale, key: string): string {
  return type.catalog[locale][key] ?? type.catalog.en[key] ?? key;
}

export const CREDENTIAL_TYPES: readonly CredentialType[] = [
  {
    id: 'httpHeaderAuth',
    fields: [
      { name: 'headerName', secret: false, required: true, default: 'Authorization' },
      { name: 'headerValue', secret: true, required: true },
    ],
    catalog: {
      en: {
        name: 'Header authentication',
        'field.headerName.label': 'Header name',
        'field.headerName.description': 'The header the value is sent in, for example Authorization.',
        'field.headerValue.label': 'Header value',
        'field.headerValue.description': 'Stored encrypted. It is never shown again after you save it.',
      },
      es: {
        name: 'Autenticación por cabecera',
        'field.headerName.label': 'Nombre de la cabecera',
        'field.headerName.description': 'La cabecera en la que se envía el valor, por ejemplo Authorization.',
        'field.headerValue.label': 'Valor de la cabecera',
        'field.headerValue.description': 'Se guarda cifrado. No se vuelve a mostrar después de guardarlo.',
      },
    },
  },
  {
    id: 'httpBearerAuth',
    fields: [{ name: 'token', secret: true, required: true }],
    catalog: {
      en: {
        name: 'Bearer token',
        'field.token.label': 'Token',
        'field.token.description': 'Sent as Authorization: Bearer <token> with every request of the node. Never shown again after you save it.',
      },
      es: {
        name: 'Token bearer',
        'field.token.label': 'Token',
        'field.token.description': 'Se envía como Authorization: Bearer <token> en cada petición del nodo. No se vuelve a mostrar después de guardarlo.',
      },
    },
  },
  {
    id: 'httpBasicAuth',
    fields: [
      { name: 'username', secret: false, required: true },
      { name: 'password', secret: true, required: true },
    ],
    catalog: {
      en: {
        name: 'Basic authentication',
        'field.username.label': 'Username',
        'field.username.description': 'The user name the service expects.',
        'field.password.label': 'Password',
        'field.password.description': 'Stored encrypted. It is never shown again after you save it.',
      },
      es: {
        name: 'Autenticación básica',
        'field.username.label': 'Usuario',
        'field.username.description': 'El nombre de usuario que espera el servicio.',
        'field.password.label': 'Contraseña',
        'field.password.description': 'Se guarda cifrado. No se vuelve a mostrar después de guardarlo.',
      },
    },
  },
  {
    id: 'whatsappCloudApi',
    fields: [
      { name: 'accessToken', secret: true, required: true },
      { name: 'phoneNumberId', secret: false, required: true },
      { name: 'appSecret', secret: true, required: true },
      { name: 'verifyToken', secret: true, required: true },
    ],
    catalog: {
      en: {
        name: 'WhatsApp Cloud API',
        'field.accessToken.label': 'Access token',
        'field.accessToken.description': 'The system user token from Meta. Meta shows it once, so copy it when you generate it.',
        'field.phoneNumberId.label': 'Phone number ID',
        'field.phoneNumberId.description': 'The digits of the WhatsApp Business phone number, not the number itself.',
        'field.appSecret.label': 'App secret',
        'field.appSecret.description': 'Checks that incoming webhooks come from Meta. Stored encrypted.',
        'field.verifyToken.label': 'Verify token',
        'field.verifyToken.description': 'The text you type in Meta when you register the webhook. Stored encrypted.',
      },
      es: {
        name: 'WhatsApp Cloud API',
        'field.accessToken.label': 'Token de acceso',
        'field.accessToken.description': 'El token del usuario del sistema de Meta. Meta lo muestra una vez, así que cópialo al generarlo.',
        'field.phoneNumberId.label': 'ID del número de teléfono',
        'field.phoneNumberId.description': 'Los dígitos del número de WhatsApp Business, no el número en sí.',
        'field.appSecret.label': 'Secreto de la aplicación',
        'field.appSecret.description': 'Comprueba que los webhooks entrantes vienen de Meta. Se guarda cifrado.',
        'field.verifyToken.label': 'Token de verificación',
        'field.verifyToken.description': 'El texto que escribes en Meta al registrar el webhook. Se guarda cifrado.',
      },
    },
  },
  {
    // One type for every model provider: the name the person gives it says which provider it is.
    id: 'modelProviderApiKey',
    fields: [{ name: 'apiKey', secret: true, required: true }],
    catalog: {
      en: {
        name: 'Model provider API key',
        'field.apiKey.label': 'API key',
        'field.apiKey.description': 'The secret key from the model provider. Stored encrypted; never shown again.',
      },
      es: {
        name: 'Clave de API de un proveedor de modelos',
        'field.apiKey.label': 'Clave de API',
        'field.apiKey.description': 'La clave secreta del proveedor del modelo. Se guarda cifrada y no se vuelve a mostrar.',
      },
    },
  },
];

export function credentialType(id: string): CredentialType | undefined {
  return CREDENTIAL_TYPES.find((type) => type.id === id);
}
