// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

import type { Catalog } from './node-manifests';

/**
 * The credential types a node slot can ask for, with the fields the "Create credential" dialog
 * builds its form from (SPEC-003 `credentials[]`: `fields[]` with `name`, `type`, `sensitive`,
 * `required`). Until the credential type view of the API exists this table is the source; labels
 * come from the same kind of language catalog as the node manifests.
 */

export interface CredentialField {
  readonly name: string;
  /** A secret field is write-only: never prefilled, never shown again, never kept by the editor (SPEC-010 FM-8). */
  readonly secret: boolean;
  readonly required: boolean;
  readonly default?: string;
}

export interface CredentialType {
  readonly id: string;
  readonly fields: readonly CredentialField[];
  readonly catalog: Catalog;
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
];

export function credentialType(id: string): CredentialType | undefined {
  return CREDENTIAL_TYPES.find((type) => type.id === id);
}
